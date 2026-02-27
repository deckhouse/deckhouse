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
	"fmt"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	NodeGroupLabel                   = "node.deckhouse.io/group"
	ConfigurationChecksumAnnotation  = "node.deckhouse.io/configuration-checksum"
	MachineNamespace                 = "d8-cloud-instance-manager"
	ConfigurationChecksumsSecretName = "configuration-checksums"
	CloudProviderSecretName          = "d8-node-manager-cloud-provider"
	DisruptionRequiredAnnotation     = "update.node.deckhouse.io/disruption-required"
	ApprovedAnnotation               = "update.node.deckhouse.io/approved"
)

// SetupFunc is a function that sets up a controller with the manager.
type SetupFunc func(mgr ctrl.Manager) error

type controllerEntry struct {
	name  string
	setup SetupFunc
}

var controllers []controllerEntry

// Register adds a controller to the registry. Call this in init() of controller packages.
func Register(name string, setup SetupFunc) {
	controllers = append(controllers, controllerEntry{name: name, setup: setup})
}

// Names returns the names of all registered controllers.
func Names() []string {
	names := make([]string, len(controllers))
	for i, c := range controllers {
		names[i] = c.name
	}
	return names
}

// SetupAll registers all controllers with the manager.
// Controllers auto-register via init() in their packages.
// disabledControllers is a comma-separated list of controller names to skip.
func SetupAll(mgr ctrl.Manager, disabledControllers string) error {
	setupLog := ctrl.Log.WithName("setup")

	disabled := make(map[string]bool)
	for _, name := range strings.Split(disabledControllers, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			disabled[name] = true
		}
	}

	for _, c := range controllers {
		if disabled[c.name] {
			setupLog.Info("controller disabled", "controller", c.name)
			continue
		}
		if err := c.setup(mgr); err != nil {
			return fmt.Errorf("unable to setup %s controller: %w", c.name, err)
		}
		setupLog.Info("controller enabled", "controller", c.name)
	}

	return nil
}
