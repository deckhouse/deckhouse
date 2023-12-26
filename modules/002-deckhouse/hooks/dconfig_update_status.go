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
}, updateModuleConfigStatuses)

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
			State: module.Properties.State,
		},
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

func updateModuleConfigStatuses(input *go_hook.HookInput) error {
	allConfigs := snapshotToModuleConfigList(input.Snapshots["configs"])

	var enabledModules map[string]struct{}
	for _, item := range input.Snapshots["modules"] {
		module := item.(*v1alpha1.Module)
		if module.Properties.State == "Enabled" {
			enabledModules[module.GetName()] = struct{}{}
		}
	}

	bundleName := os.Getenv("DECKHOUSE_BUNDLE")

	moduleNamesToSources := d8config.Service().ModuleToSourcesNames()
	for _, cfg := range allConfigs {
		_, moduleEnabled := enabledModules[cfg.GetName()]

		moduleStatus := d8config.Service().StatusReporter().ForConfig(cfg, bundleName, moduleNamesToSources, moduleEnabled)
		sPatch := makeStatusPatch(cfg, moduleStatus)
		if sPatch != nil {
			input.LogEntry.Debugf(
				"Patch /status for moduleconfig/%s: state '%s' to '%s', version '%s' to %s', status '%s' to '%s'",
				cfg.GetName(),
				cfg.Status.State, sPatch.State,
				cfg.Status.Version, sPatch.Version,
				cfg.Status.Status, sPatch.Status,
			)
			input.PatchCollector.MergePatch(sPatch, "deckhouse.io/v1alpha1", "ModuleConfig", "", cfg.GetName(), object_patch.WithSubresource("/status"))
		}
	}

	// Export metrics for configs with specified but obsolete versions.
	input.MetricsCollector.Expire(d8ConfigGroup)
	for _, cfg := range allConfigs {
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

func makeStatusPatch(cfg *v1alpha1.ModuleConfig, moduleStatus d8config.Status) *statusPatch {
	if cfg == nil || !isStatusChanged(cfg.Status, moduleStatus) {
		return nil
	}

	return &statusPatch{
		Status:  moduleStatus.Status,
		State:   moduleStatus.State,
		Version: moduleStatus.Version,
		Type:    moduleStatus.Type,
	}
}

func isStatusChanged(currentStatus v1alpha1.ModuleConfigStatus, moduleStatus d8config.Status) bool {
	switch {
	case currentStatus.State != moduleStatus.State:
		return true
	case currentStatus.Status != moduleStatus.Status:
		return true
	case currentStatus.Version != moduleStatus.Version:
		return true
	case currentStatus.Type != moduleStatus.Type:
		return true
	}
	return false
}

type statusPatch v1alpha1.ModuleConfigStatus

func (sp statusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.ModuleConfigStatus(sp),
	}

	return json.Marshal(m)
}

// snapshotToModuleConfigList returns a typed array of ModuleConfig items from untyped items in the snapshot.
func snapshotToModuleConfigList(snapshot []go_hook.FilterResult) []*v1alpha1.ModuleConfig {
	configs := make([]*v1alpha1.ModuleConfig, 0, len(snapshot))
	for _, item := range snapshot {
		cfg := item.(*v1alpha1.ModuleConfig)
		configs = append(configs, cfg)
	}
	return configs
}
