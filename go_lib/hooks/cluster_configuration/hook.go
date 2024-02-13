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

package cluster_configuration

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type Handler func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, secretFound bool) error

func RegisterHook(handler Handler) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "provider_cluster_configuration",
				ApiVersion: "v1",
				Kind:       "Secret",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-provider-cluster-configuration"},
				},
				FilterFunc: applyProviderClusterConfigurationSecretFilter,
			},
		},
	}, func(input *go_hook.HookInput) error {
		return clusterConfiguration(input, handler)
	})
}

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	return secret, nil
}

func clusterConfiguration(input *go_hook.HookInput, handler Handler) error {
	var (
		metaCfg               *config.MetaConfig
		providerDiscoveryData *unstructured.Unstructured
		secretFound           bool
	)

	snap := input.Snapshots["provider_cluster_configuration"]
	if len(snap) > 0 {
		secretFound = true
		secret := snap[0].(*v1.Secret)
		if clusterConfigurationYAML, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]; ok && len(clusterConfigurationYAML) > 0 {
			m, err := config.ParseConfigFromData(string(clusterConfigurationYAML))
			if err != nil {
				return fmt.Errorf("validate cloud-provider-cluster-configuration.yaml: %v", err)
			}
			metaCfg = m
		}
		if discoveryDataJSON, ok := secret.Data["cloud-provider-discovery-data.json"]; ok && len(discoveryDataJSON) > 0 {
			err := json.Unmarshal(discoveryDataJSON, &providerDiscoveryData)
			if err != nil {
				return fmt.Errorf("cannot unmarshal cloud-provider-discovery-data.json key: %v", err)
			}
			_, err = config.ValidateDiscoveryData(&discoveryDataJSON, []string{})
			if err != nil {
				return fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
			}
		}
	}

	return handler(input, metaCfg, providerDiscoveryData, secretFound)
}
