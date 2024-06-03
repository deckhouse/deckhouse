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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	yandexDeprecatedZoneInConfigKey = "yandex:hasDeprecatedZoneInConfig"
)

type NodeGroupZones struct {
	Zones []string `yaml:"zones"`
}

type ProviderClusterConfigZones struct {
	MasterNodeGroup NodeGroupZones   `yaml:"masterNodeGroup"`
	NodeGroups      []NodeGroupZones `yaml:"nodeGroups"`
	Zones           []string         `yaml:"zones"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
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
}, setDeprecatedZoneInUseFlag)

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return secret, nil
}

func setDeprecatedZoneInUseFlag(input *go_hook.HookInput) error {
	if len(input.Snapshots["provider_cluster_configuration"]) == 0 {
		return fmt.Errorf("%s", "Can't find Secret d8-provider-cluster-configuration in Namespace kube-system")
	}

	secret := input.Snapshots["provider_cluster_configuration"][0].(*v1.Secret)

	data := secret.Data["cloud-provider-cluster-configuration.yaml"]

	pcc := ProviderClusterConfigZones{}

	err := yaml.Unmarshal(data, &pcc)
	if err != nil {
		return fmt.Errorf("Failed to unmarshall d8-provider-cluster-configuration from the secret: %v", err)
	}

	hasDeprecatedZone := false
	for _, zone := range pcc.Zones {
		if zone == "ru-central1-c" {
			hasDeprecatedZone = true
		}
	}

	for _, zone := range pcc.MasterNodeGroup.Zones {
		if zone == "ru-central1-c" {
			hasDeprecatedZone = true
		}
	}

	for _, ng := range pcc.NodeGroups {
		for _, zone := range ng.Zones {
			if zone == "ru-central1-c" {
				hasDeprecatedZone = true
			}
		}
	}

	requirements.SaveValue(yandexDeprecatedZoneInConfigKey, hasDeprecatedZone)

	return nil
}
