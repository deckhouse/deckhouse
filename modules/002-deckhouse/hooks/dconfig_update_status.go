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

package hooks

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
)

/*
This hook tracks changes in ModuleConfig resources and updates
their statuses.
It uses AddonOperator dependency to get enabled state for all modules
and get access to each module state.

ModuleConfig status consists of:
- 'state' field - describes a module's enabled state:
    * N/A - cannot get status, or module is ignored by a reason
    * Enabled
    * Disabled
    * Disabled by script - module is disabled by 'enabled' script
    * Enabled/Disabled by config - module state is determined by ModuleConfig
- 'status' field - describes state of the module:
    * Unknown module name - ModuleConfig resource name is not a known module name.
    * (NOT IMPLEMENTED) Running - ModuleRun task is in progress: module starts or reloads.
    * (NOT IMPLEMENTED) Ready - helm install for module was successful.
	* Converging - module is waiting for the first run.
	* Ready - module successfully passed the initialization stage
    * HookError: ... - problem with module's hook.
    * ModuleError: ... - problem during installing helm chart.
*/

// TODO: get rid of this hook
// replace with event model
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/status-configs",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "configs",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleConfig",
			FilterFunc:                   filterModuleConfigForStatus,
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
		},
		{
			Name:                         "modules",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "Module",
			FilterFunc:                   filterModuleForState,
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "update_statuses",
			Crontab: "*/15 * * * * *",
		},
	},
	Settings: &go_hook.HookConfigSettings{
		EnableSchedulesOnStartup: true,
	},
}, updateStatuses)

// filterModuleForState returns name and current state from Module object.
func filterModuleForState(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var module v1alpha1.Module

	err := sdk.FromUnstructured(unstructured, &module)
	if err != nil {
		return nil, err
	}

	// Extract name, spec and status.
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name: module.Name,
		},
		Properties: v1alpha1.ModuleProperties{
			State:  module.Properties.State,
			Source: module.Properties.Source,
		},
		Status: module.Status,
	}, nil
}

// filterModuleConfigForStatus returns name, enabled flag and the current status from ModuleConfig object.
func filterModuleConfigForStatus(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cfg v1alpha1.ModuleConfig

	err := sdk.FromUnstructured(unstructured, &cfg)
	if err != nil {
		return nil, err
	}

	// Extract name, spec and status.
	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: cfg.Name,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Version:  cfg.Spec.Version,
			Enabled:  cfg.Spec.Enabled,
			Settings: cfg.Spec.Settings,
		},
		Status: cfg.Status,
	}, nil
}

const (
	d8ConfigGroup      = "deckhouse_config_metrics"
	d8ConfigMetricName = "module_config_obsolete_version"
)

func updateStatuses(input *go_hook.HookInput) error {
	allModuleConfigsMap := snapshotToModuleConfigMap(input.Snapshots["configs"])
	allModulesList := snapshotToModuleList(input.Snapshots["modules"])

	bundleName := os.Getenv("DECKHOUSE_BUNDLE")

	for _, module := range allModulesList {
		cfg := allModuleConfigsMap[module.GetName()]
		moduleStatus := d8config.Service().StatusReporter().ForModule(module, cfg, bundleName)
		sPatch := makeStatusPatchForModule(module, moduleStatus)
		if sPatch != nil {
			input.LogEntry.Debugf(
				"Patch /status for module/%s: status '%s' to '%s'",
				module.GetName(),
				module.Status.Status, sPatch.Status,
			)
			input.PatchCollector.MergePatch(sPatch, "deckhouse.io/v1alpha1", "Module", "", module.GetName(), object_patch.WithSubresource("/status"))
		}
	}

	// Export metrics for configs with specified but obsolete versions and update module configs' statuses
	input.MetricsCollector.Expire(d8ConfigGroup)
	for _, cfg := range allModuleConfigsMap {
		moduleConfigStatus := d8config.Service().StatusReporter().ForConfig(cfg)
		sPatch := makeStatusPatchForModuleConfig(cfg, moduleConfigStatus)
		if sPatch != nil {
			input.LogEntry.Debugf(
				"Patch /status for moduleconfig/%s: version '%s' to %s', message '%s' to '%s'",
				cfg.GetName(),
				cfg.Status.Version, sPatch.Version,
				cfg.Status.Message, sPatch.Message,
			)
			input.PatchCollector.MergePatch(sPatch, "deckhouse.io/v1alpha1", "ModuleConfig", "", cfg.GetName(), object_patch.WithSubresource("/status"))
		}

		chain := conversion.Registry().Chain(cfg.GetName())
		if cfg.Spec.Version > 0 && chain.Conversion(cfg.Spec.Version) != nil {
			input.MetricsCollector.Set(d8ConfigMetricName, 1.0, map[string]string{
				"name":    cfg.GetName(),
				"version": strconv.Itoa(cfg.Spec.Version),
				"latest":  strconv.Itoa(chain.LatestVersion()),
			}, metrics.WithGroup(d8ConfigGroup))
		}
	}

	return nil
}

func makeStatusPatchForModuleConfig(cfg *v1alpha1.ModuleConfig, moduleConfigStatus d8config.ModuleConfigStatus) *moduleConfigStatusPatch {
	if cfg == nil || !isModuleConfigStatusChanged(cfg.Status, moduleConfigStatus) {
		return nil
	}

	return &moduleConfigStatusPatch{
		Version: moduleConfigStatus.Version,
		Message: moduleConfigStatus.Message,
	}
}

func makeStatusPatchForModule(module *v1alpha1.Module, moduleStatus d8config.ModuleStatus) *moduleStatusPatch {
	if module == nil || !isModuleStatusChanged(module.Status, moduleStatus) {
		return nil
	}

	return &moduleStatusPatch{
		Status: moduleStatus.Status,
	}
}

func isModuleConfigStatusChanged(currentStatus v1alpha1.ModuleConfigStatus, moduleConfigStatus d8config.ModuleConfigStatus) bool {
	switch {
	case currentStatus.Version != moduleConfigStatus.Version:
		return true
	case currentStatus.Message != moduleConfigStatus.Message:
		return true
	}
	return false
}

func isModuleStatusChanged(currentStatus v1alpha1.ModuleStatus, moduleStatus d8config.ModuleStatus) bool {
	return currentStatus.Status != moduleStatus.Status
}

// snapshotToModuleConfigList returns a map of ModuleConfig items from untyped items in the snapshot.
func snapshotToModuleConfigMap(snapshot []go_hook.FilterResult) map[string]*v1alpha1.ModuleConfig {
	configs := make(map[string]*v1alpha1.ModuleConfig, 0)
	for _, item := range snapshot {
		cfg := item.(*v1alpha1.ModuleConfig)
		configs[cfg.GetName()] = cfg
	}
	return configs
}

// snapshotToModuleList returns a typed array of Module items from untyped items in the snapshot.
func snapshotToModuleList(snapshot []go_hook.FilterResult) []*v1alpha1.Module {
	modules := make([]*v1alpha1.Module, 0, len(snapshot))
	for _, item := range snapshot {
		module := item.(*v1alpha1.Module)
		modules = append(modules, module)
	}
	return modules
}

type moduleConfigStatusPatch v1alpha1.ModuleConfigStatus
type moduleStatusPatch v1alpha1.ModuleStatus

func (sp moduleConfigStatusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.ModuleConfigStatus(sp),
	}

	return json.Marshal(m)
}

func (sp moduleStatusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.ModuleStatus(sp),
	}

	return json.Marshal(m)
}
