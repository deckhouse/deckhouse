/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
