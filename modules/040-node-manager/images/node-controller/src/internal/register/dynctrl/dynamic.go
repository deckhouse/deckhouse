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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = (*dynamicController)(nil)

type dynamicController struct {
	name                    string
	obj                     client.Object
	reconcilers             []Reconciler
	isGroup                 bool
	maxConcurrentReconciles int
	client                  client.Client
	cache                   cache.Cache
	scheme                  *runtime.Scheme
	recorder                record.EventRecorder
}

func (dc *dynamicController) setupWithManager(mgr ctrl.Manager) error {
	if dc.maxConcurrentReconciles < 1 {
		dc.maxConcurrentReconciles = 1
	}

	dc.client = mgr.GetClient()
	dc.cache = mgr.GetCache()
	dc.scheme = mgr.GetScheme()
	dc.recorder = mgr.GetEventRecorderFor(dc.name)

	for _, r := range dc.reconcilers {
		dc.inject(r)
		if v, ok := r.(NeedsSetup); ok {
			if err := v.Setup(mgr); err != nil {
				return fmt.Errorf("setup %s: %w", dc.name, err)
			}
		}
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(dc.name).
		For(dc.obj).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: dc.maxConcurrentReconciles,
		})

	w := &builderWatcher{b: b}
	for _, r := range dc.reconcilers {
		r.SetupWatches(w)
	}

	if err := b.Complete(dc); err != nil {
		return fmt.Errorf("build controller %s: %w", dc.name, err)
	}

	return nil
}

func SetupController(mgr ctrl.Manager, name string, obj client.Object, reconcilers []Reconciler, isGroup bool, maxConcurrentReconciles int) error {
	if maxConcurrentReconciles < 1 {
		maxConcurrentReconciles = 1
	}
	dc := &dynamicController{
		name:                    name,
		obj:                     obj,
		reconcilers:             reconcilers,
		isGroup:                 isGroup,
		maxConcurrentReconciles: maxConcurrentReconciles,
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
	start := time.Now()

	log.V(1).Info("reconcile started", "key", req.NamespacedName.String(), "isGroup", dc.isGroup)

	if !dc.isGroup {
		result, err := dc.reconcilers[0].Reconcile(ctx, req)
		if err != nil {
			log.Error(err, "reconcile failed", "key", req.NamespacedName.String(), "duration", time.Since(start).String())
			return result, err
		}
		log.V(1).Info("reconcile completed", "key", req.NamespacedName.String(), "duration", time.Since(start).String(), "requeueAfter", result.RequeueAfter.String())
		return result, nil
	}

	result, err := dc.reconcileGroup(ctx, req)
	if err != nil {
		log.Error(err, "reconcile group failed", "key", req.NamespacedName.String(), "duration", time.Since(start).String())
		return result, err
	}
	log.V(1).Info("reconcile group completed", "key", req.NamespacedName.String(), "duration", time.Since(start).String(), "requeueAfter", result.RequeueAfter.String())
	return result, nil
}

func (dc *dynamicController) reconcileGroup(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	var (
		combined reconcile.Result
		errs     []error
	)

	for _, r := range dc.reconcilers {
		subStart := time.Now()
		log.V(1).Info("subreconciler started", "reconciler", fmt.Sprintf("%T", r), "key", req.NamespacedName.String())
		result, err := r.Reconcile(ctx, req)
		if err != nil {
			log.Error(err, "reconciler failed", "reconciler", fmt.Sprintf("%T", r))
			errs = append(errs, fmt.Errorf("reconciler %T: %w", r, err))
			continue
		}
		log.V(1).Info("subreconciler completed", "reconciler", fmt.Sprintf("%T", r), "key", req.NamespacedName.String(), "duration", time.Since(subStart).String(), "requeueAfter", result.RequeueAfter.String())
		if result.RequeueAfter > 0 && (combined.RequeueAfter == 0 || result.RequeueAfter < combined.RequeueAfter) {
			combined.RequeueAfter = result.RequeueAfter
		}
	}

	return combined, errors.Join(errs...)
}
