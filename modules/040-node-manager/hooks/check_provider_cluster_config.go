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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	CheckCloudProviderConfigRaw = "checkCloudProviderConfigRaw"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/requirements/check-config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-discovery-data"},
			},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckCloudProviderConfig)

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	return secret, nil
}

func CheckCloudProviderConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["provider_cluster_configuration"]
	if len(snap) > 0 {
		secret := snap[0].(*v1.Secret)
		if clusterConfigurationYAML, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]; ok && len(clusterConfigurationYAML) > 0 {
			err := config.CheckParseConfigFromData(string(clusterConfigurationYAML))
			if err != nil {
				requirements.SaveValue(CheckCloudProviderConfigRaw, true)
				input.MetricsCollector.Set("d8_check_cloud_provider_config", 1, nil)
				return nil
			}
		}
	}
	requirements.SaveValue(CheckCloudProviderConfigRaw, false)
	input.MetricsCollector.Expire("d8_check_cloud_provider_config")
	return nil
}
