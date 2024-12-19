/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/prometheus/cm_check",
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
			FilterFunc: filterPrometheusConfigMap,
		},
	},
}, handleConfigMaps)

func filterPrometheusConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleConfigMaps(input *go_hook.HookInput) error {
	prometheusConfigMapSnapshots := input.Snapshots["prometheus_config_map"]

	input.Values.Set("prometheus.internal.prometheusPlusPlus.enabled", len(prometheusConfigMapSnapshots) > 0)

	return nil
}
