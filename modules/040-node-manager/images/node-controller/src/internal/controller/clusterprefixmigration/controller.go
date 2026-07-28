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

// Package clusterprefixmigration seeds the cluster prefix into the global
// ModuleConfig. The prefix is being migrated out of ClusterConfiguration
// (cloud.prefix, which is going away together with the whole cloud section)
// into the global ModuleConfig as spec.settings.prefix. This controller copies
// the value from the deprecated ClusterConfiguration.cloud.prefix into the
// global ModuleConfig when it is not already set, so in-cluster consumers can
// rely on global.prefix.
package clusterprefixmigration

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/node-controller/internal/clusterprefix"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	// prefixSeededAnnotation marks that the one-time cloud.prefix -> global
	// ModuleConfig migration has already run, so the field is never re-asserted
	// afterwards (letting the operator remove it, e.g. before a downgrade).
	prefixSeededAnnotation = "node-manager.deckhouse.io/cluster-prefix-seeded"

	// checkInterval re-evaluates the migration as a safety net until it has run:
	// the ClusterConfiguration value lives in a Secret this controller does not
	// watch. Once seeded, the controller stops requeuing.
	checkInterval = 5 * time.Minute
)

// reconcileRequest is the single synthetic request all triggers collapse to:
// the migration is cluster-global, not tied to any one object.
var reconcileRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterprefix.GlobalModuleConfigName}}

func newModuleConfig() *unstructured.Unstructured {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(clusterprefix.ModuleConfigGVK())
	return mc
}

func setSeededAnnotation(mc *unstructured.Unstructured) {
	annotations := mc.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[prefixSeededAnnotation] = "true"
	mc.SetAnnotations(annotations)
}

func init() {
	// The global ModuleConfig is the primary object: editing it re-runs the
	// migration check, so a manually cleared prefix (before seeding) is handled.
	register.RegisterController("cluster-prefix-migration", newModuleConfig(), &Reconciler{})
}

type Reconciler struct {
	register.Base
	apiReader client.Reader
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	// Read directly from the API server: the manager cache does not watch the
	// d8-cluster-configuration Secret.
	r.apiReader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Only the global ModuleConfig is relevant — ignore events for every other
	// ModuleConfig in the cluster.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == clusterprefix.GlobalModuleConfigName
	}))
	// Evaluate once at startup even if the global ModuleConfig does not exist yet.
	w.WatchesRawSource(source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		q.Add(reconcileRequest)
		return nil
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cloudPrefix, err := clusterprefix.FromClusterConfigurationSecret(ctx, r.apiReader)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cloudPrefix == "" {
		// Nothing to migrate: static cluster, or cloud.prefix unset/already removed.
		// Nothing to watch either, so don't requeue.
		return ctrl.Result{}, nil
	}

	mc := newModuleConfig()
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: clusterprefix.GlobalModuleConfigName}, mc); err != nil {
		if errors.IsNotFound(err) {
			// The global ModuleConfig is not created here (avoids guessing its
			// config version). In-cluster consumers fall back to cloud.prefix until
			// an operator creates it; keep checking until then.
			logger.Info("global ModuleConfig not found; cluster prefix stays sourced from the deprecated ClusterConfiguration.cloud.prefix")
			return ctrl.Result{RequeueAfter: checkInterval}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get global ModuleConfig: %w", err)
	}

	// The migration is one-time. Once the seeded annotation is present the
	// controller never re-asserts the field, so the operator can remove
	// spec.settings.prefix (e.g. before a downgrade) without it reappearing.
	if mc.GetAnnotations()[prefixSeededAnnotation] == "true" {
		return ctrl.Result{}, nil
	}

	patched := mc.DeepCopy()
	setSeededAnnotation(patched)
	if existing, _, _ := unstructured.NestedString(mc.Object, "spec", "settings", "prefix"); existing == "" {
		if err := unstructured.SetNestedField(patched.Object, cloudPrefix, "spec", "settings", "prefix"); err != nil {
			return ctrl.Result{}, fmt.Errorf("set spec.settings.prefix: %w", err)
		}
		logger.Info("seeded global ModuleConfig prefix from the deprecated ClusterConfiguration.cloud.prefix", "prefix", cloudPrefix)
	} else {
		// A prefix is already set (by the operator or a previous run): adopt it as
		// migrated without overwriting, so future removals are not reverted.
		logger.Info("marking existing global ModuleConfig prefix as migrated", "prefix", existing)
	}
	if err := r.Client.Patch(ctx, patched, client.MergeFrom(mc)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch global ModuleConfig prefix: %w", err)
	}

	// Migration done — the annotation makes future reconciles no-ops, so stop requeuing.
	return ctrl.Result{}, nil
}
