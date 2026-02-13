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

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/node-controller/internal/registry"

	// Import controller packages to trigger init() registration.
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroupstatus"
	_ "github.com/deckhouse/node-controller/internal/controller/updateapproval"
)

// Register adds a controller to the registry. Call this in init() of controller packages.
func Register(name string, setup registry.SetupFunc) {
	registry.Register(name, setup)
}

// Names returns the names of all registered controllers.
func Names() []string {
	return registry.Names()
}

// SetupAll registers all controllers with the manager.
func SetupAll(mgr ctrl.Manager, disabledControllers string) error {
	return registry.SetupAll(mgr, disabledControllers)
}
