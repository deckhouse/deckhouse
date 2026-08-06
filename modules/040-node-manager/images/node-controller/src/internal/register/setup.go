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
	"context"
	"fmt"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func setupController(ctx context.Context, mgr ctrl.Manager, c client.Client, name string, obj client.Object, r Reconciler, maxConcurrentReconciles int) error {
	if maxConcurrentReconciles < 1 {
		maxConcurrentReconciles = 1
	}

	if v, ok := r.(NeedsClient); ok {
		v.InjectClient(c)
	}
	if v, ok := r.(NeedsRecorder); ok {
		v.InjectRecorder(mgr.GetEventRecorderFor(name))
	}

	if v, ok := r.(NeedsSetup); ok {
		if err := v.Setup(ctx, mgr); err != nil {
			return fmt.Errorf("setup %s: %w", name, err)
		}
	} else if _, hasMethod := reflect.TypeOf(r).MethodByName("Setup"); hasMethod {
		// NeedsSetup is optional, so a Setup whose signature drifted is not a compile error — it
		// is silently never called, and the reconciler runs with its fields left nil.
		return fmt.Errorf("controller %s has a Setup method that does not implement NeedsSetup: want Setup(context.Context, ctrl.Manager) error", name)
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
		return fmt.Errorf("build controller %s: %w", name, err)
	}

	return nil
}
