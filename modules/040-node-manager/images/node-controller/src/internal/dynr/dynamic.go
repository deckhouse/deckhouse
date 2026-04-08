/*
Copyright 2025 Flant JSC

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

package dynr

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/rcname"
)

var _ reconcile.Reconciler = (*dynamicReconciler)(nil)

type dynamicReconciler struct {
	name             rcname.ReconcilerName
	obj              client.Object
	childReconcilers []Reconciler
	isGroup          bool
	client           client.Client
	cache            cache.Cache
	scheme           *runtime.Scheme
}

func (dc *dynamicReconciler) setupWithManager(mgr ctrl.Manager) error {
	dc.client = mgr.GetClient()
	dc.cache = mgr.GetCache()
	dc.scheme = mgr.GetScheme()

	for _, r := range dc.childReconcilers {
		dc.inject(r)
	}

	var forOpts []builder.ForOption
	for _, r := range dc.childReconcilers {
		if fp, ok := r.(HasForPredicates); ok {
			if preds := fp.SetupForPredicates(); len(preds) > 0 {
				forOpts = append(forOpts, builder.WithPredicates(preds...))
			}
		}
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(dc.name.String()).
		For(dc.obj, forOpts...)

	w := &builderWatcher{b: b}
	for _, r := range dc.childReconcilers {
		r.SetupWatches(w)
	}

	if err := b.Complete(dc); err != nil {
		return fmt.Errorf("build controller %s: %w", dc.name, err)
	}

	return nil
}

func (dc *dynamicReconciler) inject(r Reconciler) {
	if v, ok := r.(NeedsClient); ok {
		v.InjectClient(dc.client)
	}
	if v, ok := r.(NeedsCache); ok {
		v.InjectCache(dc.cache)
	}
	if v, ok := r.(NeedsScheme); ok {
		v.InjectScheme(dc.scheme)
	}
	if v, ok := r.(NeedsLogger); ok {
		v.InjectLogger(
			logf.Log.WithName("controller").WithName(dc.name.String()).WithName(fmt.Sprintf("%T", r)),
		)
	}
}

func (dc *dynamicReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx).WithValues("controller", dc.name)
	ctx = logf.IntoContext(ctx, log)

	if !dc.isGroup {
		return dc.childReconcilers[0].Reconcile(ctx, req)
	}

	return dc.reconcileGroup(ctx, req)
}

func (dc *dynamicReconciler) reconcileGroup(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	var (
		combined reconcile.Result
		errs     []error
	)

	for _, r := range dc.childReconcilers {
		result, err := r.Reconcile(ctx, req)
		if err != nil {
			log.Error(err, "reconciler failed", "reconciler", fmt.Sprintf("%T", r))
			errs = append(errs, fmt.Errorf("reconciler %T: %w", r, err))
			continue
		}
		combined.Requeue = combined.Requeue || result.Requeue
		if result.RequeueAfter > 0 && (combined.RequeueAfter == 0 || result.RequeueAfter < combined.RequeueAfter) {
			combined.RequeueAfter = result.RequeueAfter
		}
	}

	return combined, errors.Join(errs...)
}
