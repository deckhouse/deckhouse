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
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

// syncModules syncs modules at start
func (r *reconciler) syncModules(ctx context.Context) error {
	// wait until module manager init
	r.log.Debug("wait until module manager is inited")
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return r.moduleManager.AreModulesInited(), nil
	}); err != nil {
		r.log.Errorf("failed to init module manager: %v", err)
		return err
	}

	r.log.Debugf("init registered modules")
	for _, moduleName := range r.moduleManager.GetModuleNames() {
		module := new(v1alpha1.Module)
		if err := r.client.Get(ctx, types.NamespacedName{Name: moduleName}, module); err != nil {
			if apierrors.IsNotFound(err) {
				r.log.Warnf("the '%s' module not found", moduleName)
				continue
			}
			r.log.Errorf("failed to get module %s: %v", moduleName, err)
			return err
		}

		// handle too long disabled embedded modules
		if module.DisabledByModuleConfigMoreThan(deleteReleasesAfter) && !module.IsEmbedded() {
			// delete module releases of a stale module
			r.log.Warnf("the '%s' module disabled too long, delete module releases", module.Name)
			moduleReleases := new(v1alpha1.ModuleReleaseList)
			if err := r.client.List(ctx, moduleReleases, &client.MatchingLabels{"module": module.Name}); err != nil {
				r.log.Errorf("failed to list module releases for the '%s' module: %v", module.Name, err)
				return err
			}
			for _, release := range moduleReleases.Items {
				if err := r.client.Delete(ctx, &release); err != nil {
					r.log.Errorf("failed to delete the '%s' module release for the '%s' module: %v", release.Name, module.Name, err)
					return err
				}
			}

			// clear module
			err := utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				availableSources := module.Properties.AvailableSources
				module.Properties = v1alpha1.ModuleProperties{
					AvailableSources: availableSources,
				}
				return true
			})
			if err != nil {
				r.log.Errorf("failed to clear the '%s' module: %v", module.Name, err)
				return err
			}

			err = utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
				module.Status.Phase = v1alpha1.ModulePhaseNotInstalled
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
				return true
			})
			if err != nil {
				r.log.Errorf("failed to set NotInstalled module phase for the '%s' module: %v", module.Name, err)
				return err
			}
			continue
		}

		// init modules status
		err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonInit, v1alpha1.ModuleMessageInit)
			if r.moduleManager.IsModuleEnabled(module.Name) {
				module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleManager)
			} else {
				module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModuleReasonInit, v1alpha1.ModuleMessageInit)
			}
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonInit, v1alpha1.ModuleMessageInit)
			return true
		})
		if err != nil {
			r.log.Errorf("failed to set enabled to the '%s' module: %v", moduleName, err)
			return err
		}
	}
	r.log.Debug("registered modules are inited, init module configs")

	if err := r.syncModuleConfigs(ctx); err != nil {
		r.log.Errorf("failed to sync module configs: %v", err)
		return err
	}

	r.log.Debug("module configs are inited, run event loop")

	r.init.Done()
	return r.runModuleEventLoop(ctx)
}

// syncModuleConfigs syncs module configs at start up
func (r *reconciler) syncModuleConfigs(ctx context.Context) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		configs := new(v1alpha1.ModuleConfigList)
		if err := r.client.List(ctx, configs); err != nil {
			return err
		}
		for _, moduleConfig := range configs.Items {
			if err := r.refreshModuleConfigStatus(ctx, moduleConfig.Name); err != nil {
				return err
			}
		}
		return nil
	})
}

// refreshModuleConfigStatus refreshes module config and module status by addon-operator
func (r *reconciler) refreshModuleConfigStatus(ctx context.Context, configName string) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			r.log.Debugf("refresh the '%s' module config status", configName)

			metricGroup := fmt.Sprintf("%s_%s", "obsoleteVersion", configName)
			r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

			moduleConfig := new(v1alpha1.ModuleConfig)
			if err := r.client.Get(ctx, client.ObjectKey{Name: configName}, moduleConfig); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			r.refreshConfigStatus(moduleConfig)
			if err := r.client.Status().Update(ctx, moduleConfig); err != nil {
				return err
			}

			// update metrics
			converter := conversion.Store().Get(moduleConfig.Name)
			if moduleConfig.Spec.Version > 0 && moduleConfig.Spec.Version < converter.LatestVersion() {
				r.metricStorage.Grouped().GaugeSet(metricGroup, "module_config_obsolete_version", 1.0, map[string]string{
					"name":    moduleConfig.Name,
					"version": strconv.Itoa(moduleConfig.Spec.Version),
					"latest":  strconv.Itoa(converter.LatestVersion()),
				})
			}
			return nil
		})
	})
}

// runModuleEventLoop triggers module refreshing at any event from addon-operator
func (r *reconciler) runModuleEventLoop(ctx context.Context) error {
	for event := range r.moduleManager.GetModuleEventsChannel() {
		if event.ModuleName == "" {
			continue
		}
		if err := r.refreshModule(ctx, event.ModuleName); err != nil {
			r.log.Errorf("failed to handle the event: failed to refresh the '%s' module: %v", event.ModuleName, err)
		}
	}
	return nil
}

// refreshModule refreshes module status by addon-operator
func (r *reconciler) refreshModule(ctx context.Context, moduleName string) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			module := new(v1alpha1.Module)
			if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
				return err
			}
			r.refreshModuleStatus(module)
			return r.client.Status().Update(ctx, module)
		})
	})
}
