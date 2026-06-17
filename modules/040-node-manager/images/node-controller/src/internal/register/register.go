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
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type entry struct {
	name       string
	obj        client.Object
	reconciler Reconciler
}

var entries []entry

func RegisterController(name string, obj client.Object, r Reconciler) {
	entries = append(entries, entry{name: name, obj: obj, reconciler: r})
}

func SetupAll(mgr ctrl.Manager, disabledControllers string, defaultMaxConcurrent int, perControllerMaxConcurrent map[string]int) error {
	setupLog := ctrl.Log.WithName("setup")

	disabled := make(map[string]bool)
	for _, name := range strings.Split(disabledControllers, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			disabled[name] = true
		}
	}

	registered := make(map[string]bool, len(entries))
	for _, e := range entries {
		registered[e.name] = true
	}

	for name := range perControllerMaxConcurrent {
		if !registered[name] {
			setupLog.Info("WARNING: unknown controller in max-concurrent-reconciles, ignoring", "controller", name, "registeredControllers", registeredNames())
			delete(perControllerMaxConcurrent, name)
		}
	}

	for _, e := range entries {
		if disabled[e.name] {
			setupLog.Info("controller disabled", "controller", e.name)
			continue
		}

		maxConcurrent := defaultMaxConcurrent
		if v, ok := perControllerMaxConcurrent[e.name]; ok {
			maxConcurrent = v
		}

		if err := setupController(mgr, e.name, e.obj, e.reconciler, maxConcurrent); err != nil {
			return fmt.Errorf("setting up controller %s: %w", e.name, err)
		}
		setupLog.Info("controller enabled", "controller", e.name, "maxConcurrentReconciles", maxConcurrent)
	}

	return nil
}

func registeredNames() string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.name)
	}
	return strings.Join(names, ", ")
}
