/*
Copyright 2022 Flant JSC

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

package deckhouse_config

import (
	"fmt"
	"strconv"

	bootstrappedextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/bootstrapped"
	d7sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/deckhouseversion"
	k8sversionextender "github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	dynamicextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/dynamically_enabled"
	kubeconfig "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/kube_config"
	scriptextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/script_enabled"
	staticextender "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/static"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
)

type StatusReporter struct {
	moduleManager ModuleManager
}

func NewStatusReporter(mm ModuleManager) *StatusReporter {
	return &StatusReporter{moduleManager: mm}
}

func (s *StatusReporter) RefreshModule(module *v1alpha1.Module) {
	basicModule := s.moduleManager.GetModule(module.Name)
	// TODO(ipaqas): how to handle it ?
	if basicModule == nil {
		return
	}

	if s.moduleManager.IsModuleEnabled(module.Name) {
		module.SetConditionStatus(v1alpha1.ModuleConditionEnabledByModuleManager, true)
		module.SetConditionReason(v1alpha1.ModuleConditionEnabledByModuleManager, "Enabled", "enabled")

		if module.Status.HooksState != basicModule.GetHookErrorsSummary() {
			module.Status.HooksState = basicModule.GetHookErrorsSummary()
		}

		if hookErr := basicModule.GetLastHookError(); hookErr != nil {
			module.Status.Phase = v1alpha1.ModulePhaseError
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, "HookError", hookErr.Error())
			return
		}

		if moduleError := basicModule.GetModuleError(); moduleError != nil {
			module.Status.Phase = v1alpha1.ModulePhaseError
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, "ModuleError", moduleError.Error())
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
		case modules.CanRunHelm:
			module.Status.Phase = v1alpha1.ModulePhaseReady
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, true)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, "", "")

		case modules.Startup:
			module.Status.Phase = v1alpha1.ModulePhaseEnqueued
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, v1alpha1.ModulePhaseEnqueued, "enqueued")

		case modules.OnStartupDone:
			module.Status.Phase = v1alpha1.ModulePhaseEnqueued
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, v1alpha1.ModulePhaseEnqueued, "completed OnStartUp hooks")

		case modules.WaitForSynchronization:
			module.Status.Phase = v1alpha1.ModulePhaseWaitSync
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, v1alpha1.ModulePhaseWaitSync, "run sync tasks")

		case modules.HooksDisabled:
			module.Status.Phase = v1alpha1.ModulePhasePending
			module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
			module.SetConditionReason(v1alpha1.ModuleConditionIsReady, v1alpha1.ModulePhasePending, "hooks are disabled")
		}

		return
	}

	updatedBy, updatedByErr := s.moduleManager.GetUpdatedByExtender(module.Name)
	if updatedByErr != nil {
		module.Status.Phase = v1alpha1.ModulePhaseError
		module.SetConditionStatus(v1alpha1.ModuleConditionEnabledByModuleManager, false)
		module.SetConditionReason(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModulePhaseError, updatedByErr.Error())

		module.SetConditionStatus(v1alpha1.ModuleConditionIsReady, false)
		module.SetConditionReason(v1alpha1.ModuleConditionIsReady, v1alpha1.ModulePhaseError, updatedByErr.Error())

		return
	}

	var reason string
	var message string

	switch extenders.ExtenderName(updatedBy) {
	case "", staticextender.Name:
		reason = "Bundle"
		message = "turned off by bundle"
		if !module.IsEmbedded() {
			reason = "Disabled"
			message = "disabled"
		}

	case kubeconfig.Name:
		reason = "ModuleConfig"
		message = "turned off by module config"

	case dynamicextender.Name:
		reason = "DynamicGlobalHookExtender"
		message = "turned off by global hook"

	case scriptextender.Name:
		reason = "EnabledScriptExtender"
		message = "turned off by enabled script"

	case d7sversionextender.Name:
		reason = "DeckhouseVersionExtender"
		_, errMsg := d7sversionextender.Instance().Filter(module.Name, map[string]string{})
		message = "turned off by deckhouse version"
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}

	case bootstrappedextender.Name:
		reason = "ClusterBootstrappedExtender"
		message = "turned off because the cluster not bootstrapped yet"

	case k8sversionextender.Name:
		reason = "KubernetesVersionExtender"
		_, errMsg := k8sversionextender.Instance().Filter(module.Name, map[string]string{})
		message = "turned off by kubernetes version"
		if errMsg != nil {
			message += ": " + errMsg.Error()
		}
	}

	if module.Status.Phase != v1alpha1.ModulePhaseNotInstalled {
		module.Status.Phase = v1alpha1.ModulePhasePending
	}
	module.SetConditionStatus(v1alpha1.ModuleConditionEnabledByModuleManager, false)
	module.SetConditionReason(v1alpha1.ModuleConditionEnabledByModuleManager, reason, message)
}

func (s *StatusReporter) RefreshConfig(config *v1alpha1.ModuleConfig) {
	if !Service().possibleNames.Has(config.Name) {
		config.Status.Version = ""
		config.Status.Message = "Ignored: unknown module name"
		return
	}

	validationResult := Service().ConfigValidator().Validate(config)
	if validationResult.HasError() {
		config.Status.Version = ""
		config.Status.Message = fmt.Sprintf("Error: %s", validationResult.Error)
		return
	}

	// fill the 'version' field. The value is a spec.version or the latest version from registered conversions.
	// also create warning if version is unknown or outdated.
	version := ""
	versionWarning := ""
	converter := conversion.Store().Get(config.Name)
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
