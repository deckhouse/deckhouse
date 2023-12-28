/*
Copyright 2021 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/deprecate_outdated_grafana_settings",
}, grafanaSettingsHandler)

func grafanaSettingsHandler(input *go_hook.HookInput) error {
	customPlugins := input.Values.Get("prometheus.grafana.customPlugins")
	if customPlugins.Exists() {
		pluginList := customPlugins.Array()
		for _, plugin := range pluginList {
			input.MetricsCollector.Set("d8_grafana_settings_outdated_plugin",
				1, map[string]string{
					"plugin": sanitizeLabelName(plugin.String()),
				},
			)
		}
	}
	return nil
}
