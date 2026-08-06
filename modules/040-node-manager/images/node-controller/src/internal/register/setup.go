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

package register

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// setupController wires one controller into the manager and returns the number
// of workers it was actually built with — which is not always the number asked
// for, since a reconciler may cap itself (NeedsMaxConcurrentReconciles).
func setupController(mgr ctrl.Manager, c client.Client, name string, obj client.Object, r Reconciler, maxConcurrentReconciles int) (int, error) {
	requested := maxConcurrentReconciles
	maxConcurrentReconciles = effectiveMaxConcurrentReconciles(r, requested)
	if maxConcurrentReconciles != requested {
		ctrl.Log.WithName("setup").Info("controller caps its own concurrency",
			"controller", name, "requested", requested, "maxConcurrentReconciles", maxConcurrentReconciles)
	}

	if v, ok := r.(NeedsClient); ok {
		v.InjectClient(c)
	}
	if v, ok := r.(NeedsRecorder); ok {
		v.InjectRecorder(mgr.GetEventRecorderFor(name))
	}

	if v, ok := r.(NeedsSetup); ok {
		if err := v.Setup(mgr); err != nil {
			return 0, fmt.Errorf("setup %s: %w", name, err)
		}
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		})
	if v, ok := r.(NeedsForPredicates); ok {
		b = b.For(obj, builder.WithPredicates(v.ForPredicates()...))
	} else {
		b = b.For(obj)
	}

	w := &builderWatcher{b: b}
	r.SetupWatches(w)

	if err := b.Complete(r); err != nil {
		return 0, fmt.Errorf("build controller %s: %w", name, err)
	}

	return maxConcurrentReconciles, nil
}

// effectiveMaxConcurrentReconciles is how many workers a controller is actually
// run with: what was asked for, lowered to the reconciler's own cap when it has
// one. A reconciler that is only correct below some number of workers holds that
// number itself (NeedsMaxConcurrentReconciles) rather than depending on a
// deployment argument that can be edited, mistyped or dropped elsewhere.
func effectiveMaxConcurrentReconciles(r Reconciler, requested int) int {
	if requested < 1 {
		requested = 1
	}
	v, ok := r.(NeedsMaxConcurrentReconciles)
	if !ok {
		return requested
	}
	capped := v.MaxConcurrentReconciles()
	if capped < 1 || capped >= requested {
		return requested
	}
	return capped
}
