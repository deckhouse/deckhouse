// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
)

// syncModules syncs modules at start
// TODO(ipaqsa): move it to module loader and run it in goroutine not at start
func (r *reconciler) syncModules(ctx context.Context) error {
	// wait until module manager init
	r.log.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		return fmt.Errorf("init module manager: %w", err)
	}

	r.log.Debug("sync modules")

	modules := new(v1alpha1.ModuleList)
	if err := r.client.List(ctx, modules); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}

	for _, module := range modules.Items {
		// handle too long disabled embedded modules
		if module.DisabledByModuleConfigMoreThan(deleteReleasesAfter) && !module.IsEmbedded() {
			// delete module releases of a stale module
			r.log.Infof("the %q module disabled too long, delete module releases", module.Name)
			moduleReleases := new(v1alpha1.ModuleReleaseList)
			if err := r.client.List(ctx, moduleReleases, &client.MatchingLabels{"module": module.Name}); err != nil {
				return fmt.Errorf("list module releases for the '%s' module: %w", module.Name, err)
			}
			for _, release := range moduleReleases.Items {
				if err := r.client.Delete(ctx, &release); err != nil {
					return fmt.Errorf("delete the '%s' module release for the '%s' module: %w", release.Name, module.Name, err)
				}
			}

			// clear module
			err := utils.Update[*v1alpha1.Module](ctx, r.client, &module, func(module *v1alpha1.Module) bool {
				availableSources := module.Properties.AvailableSources
				module.Properties = v1alpha1.ModuleProperties{
					AvailableSources: availableSources,
				}
				return true
			})
			if err != nil {
				return fmt.Errorf("clear the %q module: %w", module.Name, err)
			}

			// set available and skip
			err = utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, &module, func(module *v1alpha1.Module) bool {
				module.Status.Phase = v1alpha1.ModulePhaseAvailable
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
				return true
			})
			if err != nil {
				return fmt.Errorf("set the Available module phase for the '%s' module: %w", module.Name, err)
			}
		}
	}

	r.log.Debug("controller is ready")
	r.init.Done()

	return r.runModuleEventLoop(ctx)
}

// runModuleEventLoop triggers module refreshing at any event from addon-operator
func (r *reconciler) runModuleEventLoop(ctx context.Context) error {
	for event := range r.moduleManager.GetModuleEventsChannel() {
		if event.ModuleName == "" {
			continue
		}
		if err := r.refreshModule(ctx, event.ModuleName); err != nil {
			r.log.Errorf("failed to handle the event for the '%s' module: %v", event.ModuleName, err)
		}
	}
	return nil
}
