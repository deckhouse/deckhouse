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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func setupController(mgr ctrl.Manager, name string, obj client.Object, r Reconciler, maxConcurrentReconciles int) error {
	if maxConcurrentReconciles < 1 {
		maxConcurrentReconciles = 1
	}

	if v, ok := r.(NeedsClient); ok {
		v.InjectClient(mgr.GetClient())
	}
	if v, ok := r.(NeedsRecorder); ok {
		v.InjectRecorder(mgr.GetEventRecorderFor(name))
	}

	if v, ok := r.(NeedsSetup); ok {
		if err := v.Setup(mgr); err != nil {
			return fmt.Errorf("setup %s: %w", name, err)
		}
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(obj).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		})

	w := &builderWatcher{b: b}
	r.SetupWatches(w)

	if err := b.Complete(r); err != nil {
		return fmt.Errorf("build controller %s: %w", name, err)
	}

	return nil
}
