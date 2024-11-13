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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/cm_check",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "prometheus_config_map",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-monitoring",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"prometheus-pp-envs",
			}},
		},
	},
}, handleConfigMaps)

func handleConfigMaps(input *go_hook.HookInput) error {
	prometheusConfigMapSnapshots := input.Snapshots["prometheus_config_map"]

	if len(prometheusConfigMapSnapshots) > 0 {
		input.Values.Set("prometheus.internal.prometheusPlusPlus.configMapFound", true)
	} else {
		input.Values.Set("prometheus.internal.prometheusPlusPlus.configMapFound", false)
	}

	return nil
}
