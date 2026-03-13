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

package dynctrl

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
)

var _ reconcile.Reconciler = (*dynamicController)(nil)

type dynamicController struct {
	name        string
	obj         client.Object
	reconcilers []Reconciler
	isGroup     bool
	client      client.Client
	cache       cache.Cache
	scheme      *runtime.Scheme
	recorder    record.EventRecorder
}

func (dc *dynamicController) setupWithManager(mgr ctrl.Manager) error {
	dc.client = mgr.GetClient()
	dc.cache = mgr.GetCache()
	dc.scheme = mgr.GetScheme()
	dc.recorder = mgr.GetEventRecorderFor(dc.name)

	for _, r := range dc.reconcilers {
		dc.inject(r)
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(dc.name).
		For(dc.obj)

	w := &builderWatcher{b: b}
	for _, r := range dc.reconcilers {
		r.SetupWatches(w)
	}

	if err := b.Complete(dc); err != nil {
		return fmt.Errorf("build controller %s: %w", dc.name, err)
	}

	return nil
}

func SetupController(mgr ctrl.Manager, name string, obj client.Object, reconcilers []Reconciler, isGroup bool) error {
	dc := &dynamicController{
		name:        name,
		obj:         obj,
		reconcilers: reconcilers,
		isGroup:     isGroup,
	}
	return dc.setupWithManager(mgr)
}

func (dc *dynamicController) inject(r Reconciler) {
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
			logf.Log.WithName("controller").WithName(dc.name).WithName(fmt.Sprintf("%T", r)),
		)
	}
	if v, ok := r.(NeedsRecorder); ok {
		v.InjectRecorder(dc.recorder)
	}
}

func (dc *dynamicController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx).WithValues("controller", dc.name)
	ctx = logf.IntoContext(ctx, log)

	if !dc.isGroup {
		return dc.reconcilers[0].Reconcile(ctx, req)
	}

	return dc.reconcileGroup(ctx, req)
}

func (dc *dynamicController) reconcileGroup(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	var (
		combined reconcile.Result
		errs     []error
	)

	for _, r := range dc.reconcilers {
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
