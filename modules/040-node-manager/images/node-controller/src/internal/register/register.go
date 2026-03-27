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

package register

import (
	"fmt"
	"strings"
	"sync"

	"github.com/deckhouse/node-controller/internal/register/dynctrl"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerName string

func (cn ControllerName) String() string { return string(cn) }

const (
	NodeControllers      ControllerName = "node"
	NodeGroupControllers ControllerName = "nodegroup"
	InstanceControllers  ControllerName = "instance"
)

type controllerEntry struct {
	name        ControllerName
	obj         client.Object
	reconcilers []dynctrl.Reconciler
	isGroup     bool
}

var (
	registryMu sync.Mutex
	registry   []controllerEntry
)

func RegisterController(name ControllerName, obj client.Object, r dynctrl.Reconciler) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry = append(registry, controllerEntry{name: name, obj: obj, reconcilers: []dynctrl.Reconciler{r}, isGroup: false})
}

func RegisterGroup(name ControllerName, obj client.Object, reconcilers ...dynctrl.Reconciler) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry = append(registry, controllerEntry{name: name, obj: obj, reconcilers: reconcilers, isGroup: true})
}

func SetupAll(mgr ctrl.Manager, disabledControllers string, maxConcurrentReconciles int) error {
	setupLog := ctrl.Log.WithName("setup")

	disabled := make(map[string]bool)
	for _, name := range strings.Split(disabledControllers, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			disabled[name] = true
		}
	}

	registryMu.Lock()
	entries := make([]controllerEntry, len(registry))
	copy(entries, registry)
	registryMu.Unlock()

	for _, entry := range entries {
		if disabled[entry.name.String()] {
			setupLog.Info("controller disabled", "controller", entry.name)
			continue
		}

		if err := dynctrl.SetupController(mgr, entry.name.String(), entry.obj, entry.reconcilers, entry.isGroup, maxConcurrentReconciles); err != nil {
			return fmt.Errorf("setting up controller %s: %w", entry.name, err)
		}

		setupLog.Info("controller enabled", "controller", entry.name, "maxConcurrentReconciles", maxConcurrentReconciles)
	}

	return nil
}
