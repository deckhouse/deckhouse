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

// This hook keeps MachineDeployment.spec.replicas owned by the "deckhouse-hook"
// field manager so nelm's adopt-deckhouse-controller-fields gate never prunes it
// (which would scale the MachineDeployment to zero — the template does not render
// spec.replicas, the replica count is maintained out-of-band by
// set_replicas_on_machine_deployment.go and by cluster-autoscaler).
//
// Every hook write already uses the "deckhouse-hook" field manager
// (shapp.KubeClientFieldManager, set in
// deckhouse-controller/cmd/deckhouse-controller/start.go). Before the Dec 2025
// rename that manager was "deckhouse-controller" — the same name the pre-nelm
// Helm 3 engine used — so fields written by hooks before the rename and never
// changed since (a merge patch only transfers ownership of fields whose value it
// changes) are still owned by the legacy manager and become pruning targets.
//
// The FilterFunc keeps only MachineDeployments whose spec.replicas is still owned
// by the legacy field manager (everything else is dropped from the snapshot by
// returning a nil result), so the handler's snapshots contain exactly the objects
// that must be fixed. For each of them it moves spec.replicas ownership to
// "deckhouse-hook" by editing metadata.managedFields directly — dropping only that
// one field from the legacy manager and leaving the replica value and every other
// field the legacy manager owns untouched.
//
// A server-side apply cannot do this: SSA conflict detection is value-based
// (structured-merge-diff computes conflicts as ownedFields ∩ (Modified ∪ Added)),
// so re-applying spec.replicas at its current value adds a "deckhouse-hook" Apply
// co-owner but never strips the legacy "deckhouse-controller" owner — nelm's
// adopt gate then still adopts and prunes the field. Only an explicit
// managedFields rewrite transfers ownership of an unchanged value.
//
// It runs OnBeforeHelm, right before nelm applies the node-manager release, so
// ownership is migrated BEFORE the apply that would otherwise prune it — that is
// what prevents the scale-to-zero instead of only healing it afterwards.
// WaitForSynchronization is left at its default (true) so the snapshots are
// populated before beforeHelm runs; both CRDs ship in node-manager/crds and
// node-manager is critical, so they are always present and the synchronization
// cannot stall.

package hooks

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
)

const (
	machineDeploymentNamespace = "d8-cloud-instance-manager"

	// hookFieldManager must match shapp.KubeClientFieldManager set in
	// deckhouse-controller/cmd/deckhouse-controller/start.go. Fields owned by it
	// are NOT subsumed by nelm's adopt-deckhouse-controller-fields gate.
	hookFieldManager = "deckhouse-hook"

	// legacyFieldManager is the field manager used by the pre-nelm Helm 3 engine
	// and, before the Dec 2025 rename to hookFieldManager, by hook patches too.
	// nelm's adopt-deckhouse-controller-fields gate reclaims fields owned by it.
	legacyFieldManager = "deckhouse-controller"
)

var (
	mcmMachineDeploymentGVR  = schema.GroupVersionResource{Group: "machine.sapcloud.io", Version: "v1alpha1", Resource: "machinedeployments"}
	capiMachineDeploymentGVR = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "machinedeployments"}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Run before the node-manager release is applied, so ownership is migrated
	// before the adopting apply can prune spec.replicas.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/node-manager/force_machine_deployment_replicas_ownership",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "mds",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineDeployment",
			// beforeHelm needs the snapshot populated, so wait for synchronization.
			// The CRD always exists (node-manager/crds, critical module), so the wait
			// cannot stall the converge.
			WaitForSynchronization: ptr.To(true),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{machineDeploymentNamespace},
				},
			},
			FilterFunc: machineDeploymentReplicasOwnershipFilter,
		},
		{
			Name:                   "capi_mds",
			ApiVersion:             "cluster.x-k8s.io/v1beta1",
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(true),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{machineDeploymentNamespace},
				},
			},
			FilterFunc: capiMachineDeploymentReplicasOwnershipFilter,
		},
	},
}, dependency.WithExternalDependencies(handleForceMachineDeploymentReplicasOwnership))

// machineDeploymentRef is the snapshot entry for a MachineDeployment that still
// needs its spec.replicas ownership migrated. MachineDeployments that are already
// off the legacy manager never reach the snapshot (see the filters). Only the
// name is carried: the migration rewrites managedFields and never reads or writes
// the replica value.
type machineDeploymentRef struct {
	Name string
}

func machineDeploymentReplicasOwnershipFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// Keep only MachineDeployments whose spec.replicas is still owned by the legacy
	// field manager — the ones nelm's adoption gate would prune. A nil result drops
	// the object from the snapshot, so the handler only ever sees what it must fix.
	if !replicasOwnedByLegacyManager(obj.GetManagedFields()) {
		return nil, nil
	}

	var md v1alpha1.MachineDeployment
	if err := sdk.FromUnstructured(obj, &md); err != nil {
		return nil, err
	}

	return machineDeploymentRef{Name: md.Name}, nil
}

func capiMachineDeploymentReplicasOwnershipFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if !replicasOwnedByLegacyManager(obj.GetManagedFields()) {
		return nil, nil
	}

	var md v1beta1.MachineDeployment
	if err := sdk.FromUnstructured(obj, &md); err != nil {
		return nil, err
	}

	return machineDeploymentRef{Name: md.Name}, nil
}

func handleForceMachineDeploymentReplicasOwnership(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	client, err := dc.GetK8sClient()
	if err != nil {
		// Best-effort: never block the node-manager release on ownership upkeep.
		input.Logger.Warn("force replicas ownership: get k8s client", slog.Any("error", err))
		return nil
	}

	forceReplicasOwnership(ctx, input, client.Dynamic(), input.Snapshots.Get("mds"), mcmMachineDeploymentGVR)
	forceReplicasOwnership(ctx, input, client.Dynamic(), input.Snapshots.Get("capi_mds"), capiMachineDeploymentGVR)

	return nil
}

func forceReplicasOwnership(ctx context.Context, input *go_hook.HookInput, dyn dynamic.Interface, snaps []pkg.Snapshot, gvr schema.GroupVersionResource) {
	for md, err := range sdkobjectpatch.SnapshotIter[machineDeploymentRef](snaps) {
		if err != nil {
			input.Logger.Warn("force replicas ownership: iterate snapshot",
				slog.String("gvr", gvr.String()), slog.Any("error", err))
			continue
		}

		// The snapshot only contains MachineDeployments whose spec.replicas is still
		// legacy-owned (the filter drops the rest), so every entry needs the claim.
		if err := claimReplicasOwnership(ctx, dyn, gvr, md.Name); err != nil {
			input.Logger.Warn("force replicas ownership: migrate spec.replicas owner",
				slog.String("machinedeployment", md.Name), slog.Any("error", err))
		}
	}
}

// replicasOwnedByLegacyManager reports whether spec.replicas is currently owned
// by the legacy "deckhouse-controller" field manager on the main resource
// (subresources are ignored).
func replicasOwnedByLegacyManager(managedFields []metav1.ManagedFieldsEntry) bool {
	for _, entry := range managedFields {
		if entry.Manager != legacyFieldManager || entry.Subresource != "" || entry.FieldsV1 == nil {
			continue
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(entry.FieldsV1.Raw, &fields); err != nil {
			continue
		}

		specRaw, ok := fields["f:spec"]
		if !ok {
			continue
		}

		var spec map[string]json.RawMessage
		if err := json.Unmarshal(specRaw, &spec); err != nil {
			continue
		}

		if _, ok := spec["f:replicas"]; ok {
			return true
		}
	}

	return false
}

// claimReplicasOwnership moves ownership of spec.replicas — and only
// spec.replicas — off the legacy "deckhouse-controller" field manager onto
// "deckhouse-hook", without changing the replica value, so nelm's
// adopt-deckhouse-controller-fields gate no longer prunes it. Any other field the
// legacy manager happens to own is left untouched.
//
// This is done by rewriting metadata.managedFields directly rather than with a
// server-side apply, because a forced apply that re-applies spec.replicas at its
// current value does NOT strip the legacy owner: SSA conflict detection only fires
// for fields an operation adds or modifies, so re-applying an unchanged value adds
// a "deckhouse-hook" co-owner but keeps the legacy owner in place (see the package
// comment). The patch pins metadata.resourceVersion, so a concurrent write makes
// it fail with a conflict rather than clobber that write; the next converge
// retries.
func claimReplicasOwnership(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, name string) error {
	live, err := dyn.Resource(gvr).Namespace(machineDeploymentNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newManagedFields, changed, err := migrateReplicasOwnerToHook(live.GetManagedFields(), gvr.GroupVersion().String())
	if err != nil {
		return err
	}
	if !changed {
		// spec.replicas is already off the legacy manager (a concurrent run or a
		// prior converge migrated it) — nothing to do.
		return nil
	}

	// Replace the whole managedFields array and pin resourceVersion. resourceVersion
	// is a "replace" (not "test") so a stale object is rejected by etcd with a 409
	// conflict instead of by the apiserver with an invalid-request error.
	patch, err := json.Marshal([]map[string]any{
		{"op": "replace", "path": "/metadata/managedFields", "value": newManagedFields},
		{"op": "replace", "path": "/metadata/resourceVersion", "value": live.GetResourceVersion()},
	})
	if err != nil {
		return err
	}

	_, err = dyn.Resource(gvr).Namespace(machineDeploymentNamespace).Patch(
		ctx, name, apitypes.JSONPatchType, patch,
		metav1.PatchOptions{FieldManager: hookFieldManager},
	)

	return err
}

// migrateReplicasOwnerToHook returns a copy of managedFields with spec.replicas
// removed from the legacy "deckhouse-controller" manager (dropping any entry that
// is left owning nothing) and owned by "deckhouse-hook" instead. Every other field
// and manager is preserved verbatim. The bool reports whether anything changed; if
// not, the original slice is returned and no patch is needed. Only entries on the
// main resource (empty subresource) are considered.
func migrateReplicasOwnerToHook(entries []metav1.ManagedFieldsEntry, apiVersion string) ([]metav1.ManagedFieldsEntry, bool, error) {
	out := make([]metav1.ManagedFieldsEntry, 0, len(entries))
	changed := false

	for _, entry := range entries {
		if entry.Manager == legacyFieldManager && entry.Subresource == "" && entry.FieldsV1 != nil {
			removed, newRaw, err := removeReplicasFromFields(entry.FieldsV1.Raw)
			if err != nil {
				return nil, false, err
			}
			if removed {
				changed = true
				if len(newRaw) == 0 {
					// The legacy entry owned nothing but spec.replicas — drop it.
					continue
				}
				// entry is a copy from range; repointing FieldsV1 does not mutate the
				// caller's slice (the raw bytes are never modified in place).
				entry.FieldsV1 = &metav1.FieldsV1{Raw: newRaw}
			}
		}
		out = append(out, entry)
	}

	if !changed {
		return entries, false, nil
	}

	if err := ensureHookOwnsReplicas(&out, apiVersion); err != nil {
		return nil, false, err
	}

	return out, true, nil
}

// removeReplicasFromFields removes the f:spec/f:replicas path from a FieldsV1 blob.
// It returns whether the path was present, and the re-encoded blob with empty
// parents pruned (nil when the whole blob is left empty).
func removeReplicasFromFields(raw []byte) (bool, []byte, error) {
	top := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return false, nil, err
	}

	specRaw, ok := top["f:spec"]
	if !ok {
		return false, raw, nil
	}

	spec := map[string]json.RawMessage{}
	if err := json.Unmarshal(specRaw, &spec); err != nil {
		return false, nil, err
	}
	if _, ok := spec["f:replicas"]; !ok {
		return false, raw, nil
	}

	delete(spec, "f:replicas")
	if len(spec) == 0 {
		delete(top, "f:spec")
	} else {
		newSpec, err := json.Marshal(spec)
		if err != nil {
			return false, nil, err
		}
		top["f:spec"] = newSpec
	}

	if len(top) == 0 {
		return true, nil, nil
	}

	newRaw, err := json.Marshal(top)
	if err != nil {
		return false, nil, err
	}

	return true, newRaw, nil
}

// ensureHookOwnsReplicas makes the "deckhouse-hook" manager own f:spec/f:replicas:
// it adds the path to the first existing deckhouse-hook entry on the main resource,
// or appends a new Update entry when there is none. Update mirrors the entry that
// set_replicas_on_machine_deployment.go produces and that nelm already leaves
// alone.
func ensureHookOwnsReplicas(entries *[]metav1.ManagedFieldsEntry, apiVersion string) error {
	for i := range *entries {
		entry := &(*entries)[i]
		if entry.Manager != hookFieldManager || entry.Subresource != "" || entry.FieldsV1 == nil {
			continue
		}

		added, newRaw, err := addReplicasToFields(entry.FieldsV1.Raw)
		if err != nil {
			return err
		}
		if added {
			entry.FieldsV1 = &metav1.FieldsV1{Raw: newRaw}
		}
		return nil
	}

	*entries = append(*entries, metav1.ManagedFieldsEntry{
		Manager:    hookFieldManager,
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: apiVersion,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)},
	})

	return nil
}

// addReplicasToFields adds the f:spec/f:replicas path to a FieldsV1 blob, reporting
// whether it had to change anything (it is a no-op when the path is already there).
func addReplicasToFields(raw []byte) (bool, []byte, error) {
	top := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return false, nil, err
	}

	spec := map[string]json.RawMessage{}
	if specRaw, ok := top["f:spec"]; ok {
		if err := json.Unmarshal(specRaw, &spec); err != nil {
			return false, nil, err
		}
	}
	if _, ok := spec["f:replicas"]; ok {
		return false, raw, nil
	}

	spec["f:replicas"] = json.RawMessage(`{}`)
	newSpec, err := json.Marshal(spec)
	if err != nil {
		return false, nil, err
	}
	top["f:spec"] = newSpec

	newRaw, err := json.Marshal(top)
	if err != nil {
		return false, nil, err
	}

	return true, newRaw, nil
}
