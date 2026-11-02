/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	coordinationv1 "k8s.io/api/coordination/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// The leader of the quota ledger writes its address into the EndpointSlice of the ledger Service, and
// a follower takes it out when the lease stands still. When both replicas are cut off from the API
// server, nobody inside the module pod can do that, and the API server keeps calling an address that
// does not answer, seconds per create. This hook runs in the Deckhouse pod, on a queue of its own so a
// blocked module queue does not hold it: when the lease stood still for quotaLedgerStillFor between
// two reads by the monotonic clock of Deckhouse, or its last renewal is older than
// quotaLedgerStaleAfter by the wall clock, it empties the slice, and the calls fail at once.
const (
	quotaLedgerNamespace     = "d8-multitenancy-manager"
	quotaLedgerName          = "multitenancy-manager-quota-ledger"
	quotaLedgerLeaseSnapshot = "quota_ledger_lease"
	quotaLedgerSliceSnapshot = "quota_ledger_slice"

	// quotaLedgerStillFor is how long the hook sees the lease unchanged, by its own monotonic clock,
	// before it empties the slice. The lease is renewed every second, and the clocks of the masters
	// are not compared.
	quotaLedgerStillFor = 10 * time.Second
	// quotaLedgerStaleAfter is the age of the last renewal by the wall clock at which the hook empties
	// the slice anyway, also right after Deckhouse restarted; the margin covers the clocks of the
	// masters disagreeing.
	quotaLedgerStaleAfter = 30 * time.Second
)

// quotaLedgerNow is time.Now unless a test sets it.
var quotaLedgerNow = time.Now

// quotaLedgerObservation is the lease as the hook last saw it change: its holder and renewal, and
// when by the monotonic clock of Deckhouse. A restart of Deckhouse starts it anew.
var quotaLedgerObservation struct {
	mu   sync.Mutex
	key  string
	seen time.Time
}

// quotaLedgerStill records the lease and reports whether it stood still for quotaLedgerStillFor.
func quotaLedgerStill(lease quotaLedgerLease, now time.Time) bool {
	key := lease.Holder + "@" + lease.Renew.UTC().Format(time.RFC3339Nano)
	quotaLedgerObservation.mu.Lock()
	defer quotaLedgerObservation.mu.Unlock()
	if key != quotaLedgerObservation.key || quotaLedgerObservation.seen.IsZero() {
		quotaLedgerObservation.key, quotaLedgerObservation.seen = key, now
		return false
	}
	return now.Sub(quotaLedgerObservation.seen) >= quotaLedgerStillFor
}

type quotaLedgerLease struct {
	Holder string    `json:"holder"`
	Renew  time.Time `json:"renew"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager/quota-ledger-janitor",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "quota_ledger_janitor", Crontab: "*/5 * * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         quotaLedgerLeaseSnapshot,
			ApiVersion:                   "coordination.k8s.io/v1",
			Kind:                         "Lease",
			NamespaceSelector:            &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{quotaLedgerNamespace}}},
			NameSelector:                 &types.NameSelector{MatchNames: []string{quotaLedgerName}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterQuotaLedgerLease,
		},
		{
			Name:                         quotaLedgerSliceSnapshot,
			ApiVersion:                   "discovery.k8s.io/v1",
			Kind:                         "EndpointSlice",
			NamespaceSelector:            &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{quotaLedgerNamespace}}},
			NameSelector:                 &types.NameSelector{MatchNames: []string{quotaLedgerName}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterQuotaLedgerSlice,
		},
	},
}, dependency.WithExternalDependencies(func(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return clearStaleQuotaLedgerRoute(ctx, input, dc, quotaLedgerNow())
}))

// readQuotaLedgerLease reads the lease from the API server; a missing lease has no holder.
func readQuotaLedgerLease(ctx context.Context, dc dependency.Container) (quotaLedgerLease, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return quotaLedgerLease{}, fmt.Errorf("get the Kubernetes client: %w", err)
	}
	lease, err := client.CoordinationV1().Leases(quotaLedgerNamespace).Get(ctx, quotaLedgerName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return quotaLedgerLease{}, nil
	case err != nil:
		return quotaLedgerLease{}, fmt.Errorf("get the lease: %w", err)
	}
	out := quotaLedgerLease{Holder: ptr.Deref(lease.Spec.HolderIdentity, "")}
	if lease.Spec.RenewTime != nil {
		out.Renew = lease.Spec.RenewTime.Time
	}
	return out, nil
}

// quotaLedgerRenewedSince reports whether the lease the API server returns was renewed after the one
// the hook decided on, recently enough to belong to a leader that answers.
func quotaLedgerRenewedSince(seen, live quotaLedgerLease, now time.Time) bool {
	if live.Holder == "" || now.Sub(live.Renew) >= quotaLedgerStaleAfter {
		return false
	}
	return live.Holder != seen.Holder || live.Renew.After(seen.Renew)
}

func filterQuotaLedgerLease(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	lease := new(coordinationv1.Lease)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, lease); err != nil {
		return nil, fmt.Errorf("convert the lease: %w", err)
	}
	out := quotaLedgerLease{Holder: ptr.Deref(lease.Spec.HolderIdentity, "")}
	if lease.Spec.RenewTime != nil {
		out.Renew = lease.Spec.RenewTime.Time
	}
	return out, nil
}

// filterQuotaLedgerSlice keeps the number of addresses in the slice.
func filterQuotaLedgerSlice(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	slice := new(discoveryv1.EndpointSlice)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, slice); err != nil {
		return nil, fmt.Errorf("convert the EndpointSlice: %w", err)
	}
	return len(slice.Endpoints), nil
}

func clearStaleQuotaLedgerRoute(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, now time.Time) error {
	leases, err := sdkobjectpatch.UnmarshalToStruct[quotaLedgerLease](input.Snapshots, quotaLedgerLeaseSnapshot)
	if err != nil {
		return fmt.Errorf("unmarshal the %s snapshot: %w", quotaLedgerLeaseSnapshot, err)
	}
	slices, err := sdkobjectpatch.UnmarshalToStruct[int](input.Snapshots, quotaLedgerSliceSnapshot)
	if err != nil {
		return fmt.Errorf("unmarshal the %s snapshot: %w", quotaLedgerSliceSnapshot, err)
	}
	// the lease is watched on every run, so that a slice published later finds the count going
	still := len(leases) > 0 && quotaLedgerStill(leases[0], now)
	if len(slices) == 0 || slices[0] == 0 {
		return nil
	}
	if len(leases) > 0 && leases[0].Holder != "" && !still && now.Sub(leases[0].Renew) < quotaLedgerStaleAfter {
		return nil
	}
	// The snapshot is what the watch of Deckhouse shows; a watch that lags makes a renewed lease look
	// still. The lease is read once more from the API server before the address of the leader goes.
	live, err := readQuotaLedgerLease(ctx, dc)
	if err != nil {
		input.Logger.Warn("read the quota ledger lease; leaving its EndpointSlice as it is", "error", err)
		return nil
	}
	var seen quotaLedgerLease
	if len(leases) > 0 {
		seen = leases[0]
	}
	if quotaLedgerRenewedSince(seen, live, now) {
		return nil
	}
	input.Logger.Warn("the quota ledger lease is not renewed; emptying its EndpointSlice so that the calls fail at once")
	input.PatchCollector.PatchWithMerge(map[string]any{"endpoints": []any{}}, "discovery.k8s.io/v1", "EndpointSlice", quotaLedgerNamespace, quotaLedgerName)
	return nil
}
