/*
Copyright 2025 Flant JSC

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
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	CheckStaticClusterConfigRaw = "checkStaticClusterConfigRaw"
)

// TODO: Remove this hook after 1.70.0 release
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/requirements/check_static_cluster_configuration",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "check_static_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-static-cluster-configuration",
			}},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckStaticClusterConfig)

func CheckStaticClusterConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["check_static_cluster_configuration"]
	if len(snap) == 0 {
		return nil
	}
	secret := snap[0].(*v1.Secret)
	if YAML, ok := secret.Data["static-cluster-configuration.yaml"]; ok && len(YAML) > 0 {
		err := config.ValidateConf(&YAML)
		if err != nil {
			requirements.SaveValue(CheckStaticClusterConfigRaw, true)
			input.MetricsCollector.Set("d8_check_static_cluster_config", 1, nil)
			errText := err.Error()
			if str := findErrorLines(errText); str != "" {
				input.Logger.Error(str)
			} else {
				input.Logger.Error(errText)
			}
			return nil
		}
		requirements.SaveValue(CheckStaticClusterConfigRaw, false)
		input.MetricsCollector.Expire("d8_check_static_cluster_config")
	}
	return nil
}
