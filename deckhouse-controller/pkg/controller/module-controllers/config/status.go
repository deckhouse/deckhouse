/*
Copyright 2024 Flant JSC

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

package config

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	dynamicextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/dynamically_enabled"
	kubeconfigextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/kube_config"
	scriptextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/script_enabled"
	staticextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/static"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	bootstrappedextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	d7sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	editionavailablextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionavailable"
	editionenabledextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/editionenabled"
	k8sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	moduledependencyextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

// refreshModule refreshes module in cluster
func (r *reconciler) refreshModule(ctx context.Context, moduleName string) error {
	r.logger.Debug("refresh module status", slog.String("name", moduleName))

	// events happen quite often, so conflicts happen often, default backoff not suitable
	backoff := wait.Backoff{
		Steps: 6,
		// magic number
		Duration: 20 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	module := new(v1alpha1.Module)
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(backoff, func() error {
			if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
				return err
			}
			r.refreshModuleStatus(module)
			return r.client.Status().Update(ctx, module)
		})
	})
}

// refreshModuleConfig refreshes module config in cluster
func (r *reconciler) refreshModuleConfig(ctx context.Context, configName string) error {
	r.logger.Debug("refresh module config status", slog.String("name", configName))

	// clear metrics
	metricGroup := fmt.Sprintf(obsoleteConfigMetricGroup, configName)
	r.metricStorage.Grouped().ExpireGroupMetrics(metricGroup)

	moduleConfig := new(v1alpha1.ModuleConfig)
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.client.Get(ctx, client.ObjectKey{Name: configName}, moduleConfig); err != nil {
				if apierrors.IsNotFound(err) {
					r.logger.Debug("module config not found", slog.String("name", configName))
					return nil
				}
				return fmt.Errorf("refresh the '%s' module config: %w", configName, err)
			}

			r.refreshModuleConfigStatus(moduleConfig)
			if err := r.client.Status().Update(ctx, moduleConfig); err != nil {
				return err
			}

			// skip firing alert for global module
			if moduleConfig.Name != "global" {
				// update metrics
				converter := r.conversionsStore.Get(moduleConfig.Name)
				// fire alert at obsolete version
				if moduleConfig.Spec.Version > 0 && moduleConfig.Spec.Version < converter.LatestVersion() {
					r.metricStorage.Grouped().GaugeSet(metricGroup, "d8_module_config_obsolete_version", 1.0, map[string]string{
						"name":    moduleConfig.Name,
						"version": strconv.Itoa(moduleConfig.Spec.Version),
						"latest":  strconv.Itoa(converter.LatestVersion()),
					})
				}
			}
			return nil
		})
	})
}

// refreshModuleStatus refreshes module status by addon-operator
func (r *reconciler) refreshModuleStatus(module *v1alpha1.Module) {
	basicModule := r.moduleManager.GetModule(module.Name)
	if basicModule == nil {
		return
	}

	if r.moduleManager.IsModuleEnabled(module.Name) {
		module.SetConditionTrue(v1alpha1.ModuleConditionEnabledByModuleManager)

		if module.Status.HooksState != basicModule.GetHookErrorsSummary() {
			module.Status.HooksState = basicModule.GetHookErrorsSummary()
		}

		if hookErr := basicModule.GetLastHookError(); hookErr != nil {
			module.Status.Phase = v1alpha1.ModulePhaseError
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonHookError, hookErr.Error())
			return
		}

		if moduleError := basicModule.GetModuleError(); moduleError != nil {
			module.Status.Phase = v1alpha1.ModulePhaseError
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonModuleError, moduleError.Error())
			return
		}

		switch basicModule.GetPhase() {
		// Best effort alarm!
		//
		// Actually, this condition is not correct because the `CanRunHelm` status appears right before the first run.c
		// The right approach is to check the queue for the module run task.
		// However, there are too many addon-operator internals involved.
		// We should consider moving these statuses to the `Module` resource,
		// which is directly controlled by addon-operator.
		case modules.Ready:
			if !basicModule.HasReadiness() {
				module.Status.Phase = v1alpha1.ModulePhaseReady
				module.SetConditionTrue(v1alpha1.ModuleConditionIsReady)
			}

		case modules.Startup:
			if module.Status.Phase == v1alpha1.ModulePhaseDownloading {
				module.Status.Phase = v1alpha1.ModulePhaseInstalling
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonInstalling, v1alpha1.ModuleMessageInstalling)
			} else {
				module.Status.Phase = v1alpha1.ModulePhaseReconciling
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonReconciling, v1alpha1.ModuleMessageReconciling)
			}

		case modules.OnStartupDone:
			if module.Status.Phase != v1alpha1.ModulePhaseInstalling {
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonReconciling, v1alpha1.ModuleMessageOnStartupHook)
			} else {
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonInstalling, v1alpha1.ModuleMessageOnStartupHook)
			}
		}

		return
	}

	// clear hook state
	module.Status.HooksState = ""

	updatedBy, updatedByErr := r.moduleManager.GetUpdatedByExtender(module.Name)
	if updatedByErr != nil {
		module.Status.Phase = v1alpha1.ModulePhaseError
		module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModuleReasonError, updatedByErr.Error())
		module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonError, updatedByErr.Error())
		return
	}

	var reason string
	var message string

	switch extenders.ExtenderName(updatedBy) {
	case "", staticextender.Name:
		reason = v1alpha1.ModuleReasonBundle
		message = v1alpha1.ModuleMessageBundle
		if !module.IsEmbedded() {
			reason = v1alpha1.ModuleReasonDisabled
			message = v1alpha1.ModuleMessageDisabled
		}

	case kubeconfigextender.Name:
		reason = v1alpha1.ModuleReasonModuleConfig
		message = v1alpha1.ModuleMessageModuleConfig

	case dynamicextender.Name:
		reason = v1alpha1.ModuleReasonDynamicGlobalHookExtender
		message = v1alpha1.ModuleMessageDynamicGlobalHookExtender

	case scriptextender.Name:
		reason = v1alpha1.ModuleReasonEnabledScriptExtender
		message = v1alpha1.ModuleMessageEnabledScriptExtender
		if txt := basicModule.GetEnabledScriptReason(); txt != nil && *txt != "" {
			message += ": " + *txt
		}
	case d7sversionextender.Name:
		reason = v1alpha1.ModuleReasonDeckhouseVersionExtender
		_, errMsg := r.exts.DeckhouseVersion.Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageDeckhouseVersionExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}

	case editionavailablextender.Name:
		module.Status.Phase = v1alpha1.ModulePhaseUnavailable
		reason = v1alpha1.ModuleReasonEditionAvailableExtender
		_, errMsg := r.exts.EditionAvailable.Filter(module.Name, map[string]string{})
		if errMsg != nil {
			message = errMsg.Error()
		}

	case editionenabledextender.Name:
		module.Status.Phase = v1alpha1.ModulePhaseDownloaded
		reason = v1alpha1.ModuleReasonEditionEnabledExtender
		_, errMsg := r.exts.EditionEnabled.Filter(module.Name, map[string]string{})
		if errMsg != nil {
			message = errMsg.Error()
		}

	case k8sversionextender.Name:
		reason = v1alpha1.ModuleReasonKubernetesVersionExtender
		_, errMsg := k8sversionextender.Instance().Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageKubernetesVersionExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}

	case bootstrappedextender.Name:
		reason = v1alpha1.ModuleReasonBootstrappedExtender
		message = v1alpha1.ModuleMessageBootstrappedExtender

	case moduledependencyextender.Name:
		reason = v1alpha1.ModuleReasonModuleDependencyExtender
		_, errMsg := moduledependencyextender.Instance().Filter(module.Name, map[string]string{})
		message = v1alpha1.ModuleMessageModuleDependencyExtender
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}
	}

	// do not change phase of not installed module
	if module.Status.Phase != v1alpha1.ModulePhaseAvailable && module.Status.Phase != v1alpha1.ModulePhaseUnavailable {
		module.Status.Phase = v1alpha1.ModulePhaseDownloaded
	}

	module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, reason, message)
	module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, reason, message)
}

// refreshModuleConfigStatus refreshes module config status by validator and conversions
func (r *reconciler) refreshModuleConfigStatus(config *v1alpha1.ModuleConfig) {
	if r.configValidator != nil {
		validationResult := r.configValidator.Validate(config)
		if validationResult.HasError() {
			config.Status.Version = ""
			config.Status.Message = fmt.Sprintf("Error: %s", validationResult.Error)
			return
		}
	}

	// fill the 'version' field. The value is a spec.version or the latest version from registered conversions.
	// also create warning if version is unknown or outdated.
	version := ""
	versionWarning := ""
	converter := r.conversionsStore.Get(config.Name)
	if config.Spec.Version == 0 {
		// use latest version if spec.version is empty.
		version = strconv.Itoa(converter.LatestVersion())
	}
	if config.Spec.Version > 0 {
		version = strconv.Itoa(config.Spec.Version)
		if !converter.IsKnownVersion(config.Spec.Version) {
			versionWarning = fmt.Sprintf("Error: invalid spec.version, use version %d", converter.LatestVersion())
		} else if config.Spec.Version < converter.LatestVersion() {
			// warn about obsolete version if there is conversion for spec.version.
			versionWarning = fmt.Sprintf("Update available, latest spec.settings schema version is %d", converter.LatestVersion())
		}
	}

	if (config.Status.Version != version) || (config.Status.Message != versionWarning) {
		config.Status.Version = version
		config.Status.Message = versionWarning
	}
}
