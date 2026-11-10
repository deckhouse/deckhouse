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

// Package groupprefix deletes the refresh tokens of a DexProvider when its spec.groupPrefix changes.
//
// Dex stores the claims of a user in the refresh token and puts the prefix in front of the groups when it issues
// the tokens, not when it reads them back. A refresh token issued before the change carries the groups as they were
// then, and with it the user would keep the old names until the next sign-in. Deleting the tokens makes the users sign
// in again, and the sign-in gives them the groups with the new prefix.
//
// The prefix each provider had at the last pass is kept in a ConfigMap, because the controller has no other way to tell
// that the prefix changed. A provider the ConfigMap does not know is taken to have had no prefix: a provider
// created with a prefix has no tokens, and one that already had tokens when the controller learned of its prefix
// gets them deleted, which is what its users need too.
//
// Dex takes the new configuration some time after the object changed, and a token issued in between still has the
// old prefix. The tokens of the connector are therefore deleted a second time 30 minutes after the change, and that
// pass deletes the tokens issued before the change and that margin. A token a Dex pod still running the old
// configuration issues later than those 30 minutes can keep the old names. A refresh puts the current prefix in
// front of the groups it is given. The local connector has no prefix, and a browser session reads the groups from
// the user identity, which keeps them before the prefix, so a prefix change names new:admins. The case that stacks
// the prefixes is a refresh token still inside the reuse window (2 minutes in the Dex config): it keeps the groups
// stored after the previous prefix, and applying the prefix again names new:old:admins until the token expires or
// this controller deletes it. With allowed groups set to the new prefix that refresh is refused, and the doubled
// name matches no rule for the new prefix. The change is kept next to the prefix.
package groupprefix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/naming"
)

const (
	// StateConfigMap is the ConfigMap in the Dex namespace that keeps the prefix of every DexProvider as of the last pass.
	StateConfigMap = "dexprovider-group-prefixes"

	// DefaultResweepDelay is how long after a change of the prefix the tokens of the connector are deleted once more.
	// It is the rollout margin: the second pass has to run once that margin has elapsed. An earlier pass would see a
	// cutoff still in the future and delete every token that exists, and a token an old pod issued after the pass
	// would survive inside the margin.
	DefaultResweepDelay = 30 * time.Minute

	// DefaultRolloutMargin is how long after a change of the prefix a Dex pod may still issue tokens with the old one:
	// the time its configuration takes to roll out, with a reserve. The second pass deletes the tokens issued before
	// the change and this margin. A token issued later than the margin, by a pod that still runs the old configuration,
	// can survive.
	DefaultRolloutMargin = 30 * time.Minute
)

// Revoker deletes the refresh tokens of a connector.
type Revoker interface {
	DeleteConnectorTokens(ctx context.Context, connectorID string, createdBefore time.Time) (int, error)
}

var _ Revoker = (*revoke.Revoker)(nil)

// entry is what the ConfigMap keeps for a provider: the prefix at the last pass and, after a change, when to delete
// the tokens a second time and which tokens: those issued before SweepBefore, the change plus the rollout margin.
type entry struct {
	Prefix      string     `json:"prefix"`
	ResweepAt   *time.Time `json:"resweepAt,omitempty"`
	SweepBefore *time.Time `json:"sweepBefore,omitempty"`
}

// Reconciler compares the prefix of every DexProvider with the one the last pass saw.
type Reconciler struct {
	client  client.Client
	reader  client.Reader
	revoker Revoker
	log     logr.Logger
	now     func() time.Time
	resweep time.Duration
	margin  time.Duration
}

// New returns a Reconciler. reader must bypass the cache: the ConfigMap is read fresh. now defaults to time.Now,
// resweep to DefaultResweepDelay and margin to DefaultRolloutMargin.
func New(c client.Client, reader client.Reader, revoker Revoker, log logr.Logger, now func() time.Time, resweep, margin time.Duration) *Reconciler {
	if now == nil {
		now = time.Now
	}
	if resweep <= 0 {
		resweep = DefaultResweepDelay
	}
	if margin <= 0 {
		margin = DefaultRolloutMargin
	}
	return &Reconciler{client: c, reader: reader, revoker: revoker, log: log, now: now, resweep: resweep, margin: margin}
}

// Register wires the reconciler onto the manager.
//
// Required RBAC: dexproviders get, list, watch; configmaps get (the state) and create, update; the rights of the
// revoke package for the refresh tokens and the OfflineSessions.
func Register(mgr manager.Manager) error {
	log := ctrl.Log.WithName("groupprefix")
	revoker := revoke.New(mgr.GetClient(), mgr.GetAPIReader(), log, nil)
	r := New(mgr.GetClient(), mgr.GetAPIReader(), revoker, log, nil, 0, 0)
	return ctrl.NewControllerManagedBy(mgr).
		Named("groupprefix").
		// One key for every change: a pass covers all the providers, so the changes of several providers are one pass.
		Watches(controller.Object(controller.DexProviderGVK), handler.EnqueueRequestsFromMapFunc(passRequest)).
		Complete(r)
}

// passRequest maps every DexProvider to the one request of a pass.
func passRequest(context.Context, client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: StateConfigMap}}}
}

// Reconcile makes one pass over all the providers, whichever one the request names.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	state, existed, err := r.readState(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	providers := controller.List(controller.DexProviderGVK)
	if err := r.client.List(ctx, providers); err != nil {
		return reconcile.Result{}, fmt.Errorf("list dexproviders: %w", err)
	}

	next := make(map[string]entry, len(providers.Items))
	var errs []error
	var wait time.Duration
	now := r.now()
	for i := range providers.Items {
		provider := &providers.Items[i]
		name := provider.GetName()
		desired := prefixOf(provider.Object)
		current, seen := state[name]

		sweep := false
		var createdBefore time.Time // the first pass deletes every token of the connector
		switch {
		case current.Prefix != desired:
			// A provider the state does not know and without a prefix has nothing to delete.
			sweep = seen || desired != ""
			current = entry{Prefix: desired}
			if sweep {
				at, before := now.Add(r.resweep), now.Add(r.margin)
				current.ResweepAt, current.SweepBefore = &at, &before
			}
		case current.ResweepAt != nil && !now.Before(*current.ResweepAt):
			sweep = true
			// The tokens issued before the change and its margin; those of the first pass are gone.
			if current.SweepBefore != nil {
				createdBefore = *current.SweepBefore
			}
			current.ResweepAt, current.SweepBefore = nil, nil
		}

		if sweep {
			deleted, err := r.revoker.DeleteConnectorTokens(ctx, name, createdBefore)
			if err != nil {
				// Keep what the last pass saw, so that the next one tries again.
				errs = append(errs, fmt.Errorf("delete the refresh tokens of %s: %w", name, err))
				next[name] = state[name]
				continue
			}
			r.log.Info("Deleted the refresh tokens of a DexProvider for a changed groupPrefix", "name", name, "tokens", deleted)
		}
		next[name] = current
		if current.ResweepAt != nil {
			if left := current.ResweepAt.Sub(now); wait == 0 || left < wait {
				wait = left
			}
		}
	}

	if err := r.writeState(ctx, next, state, existed); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return reconcile.Result{}, errors.Join(errs...)
	}
	return reconcile.Result{RequeueAfter: wait}, nil
}

// prefixOf returns spec.groupPrefix of a DexProvider, or "".
func prefixOf(obj map[string]any) string {
	prefix, _, _ := unstructured.NestedString(obj, "spec", "groupPrefix")
	return prefix
}

func (r *Reconciler) readState(ctx context.Context) (map[string]entry, bool, error) {
	cm := &corev1.ConfigMap{}
	err := r.reader.Get(ctx, client.ObjectKey{Namespace: naming.DexNamespace, Name: StateConfigMap}, cm)
	switch {
	case apierrors.IsNotFound(err):
		return map[string]entry{}, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("get %s: %w", StateConfigMap, err)
	}
	state := make(map[string]entry, len(cm.Data))
	for name, raw := range cm.Data {
		var e entry
		// An entry that cannot be read is as good as none: the provider is swept again and the entry rewritten.
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			r.log.Info("Ignoring an unreadable entry of the group prefix state", "name", name)
			continue
		}
		state[name] = e
	}
	return state, true, nil
}

func (r *Reconciler) writeState(ctx context.Context, next, previous map[string]entry, existed bool) error {
	if existed && equal(next, previous) {
		return nil
	}
	data := make(map[string]string, len(next))
	for name, e := range next {
		raw, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("encode the state of %s: %w", name, err)
		}
		data[name] = string(raw)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StateConfigMap,
			Namespace: naming.DexNamespace,
			Labels:    map[string]string{"heritage": "deckhouse", "module": "user-authn"},
		},
		Data: data,
	}
	if !existed {
		if err := r.client.Create(ctx, cm); err != nil {
			return fmt.Errorf("create %s: %w", StateConfigMap, err)
		}
		return nil
	}
	if err := r.client.Update(ctx, cm); err != nil {
		return fmt.Errorf("update %s: %w", StateConfigMap, err)
	}
	return nil
}

func equal(a, b map[string]entry) bool {
	return maps.EqualFunc(a, b, func(x, y entry) bool {
		return x.Prefix == y.Prefix && sameTime(x.ResweepAt, y.ResweepAt) && sameTime(x.SweepBefore, y.SweepBefore)
	})
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
