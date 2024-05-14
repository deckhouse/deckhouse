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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type InternalValues struct {
	KeyName                   string            `json:"keyName"`
	Instances                 Instances         `json:"instances"`
	LoadBalancerSecurityGroup string            `json:"loadBalancerSecurityGroup"`
	Zones                     []string          `json:"zones"`
	ZoneToSubnetIDMap         interface{}       `json:"zoneToSubnetIdMap"`
	ProviderAccessKeyID       string            `json:"providerAccessKeyId"`
	ProviderSecretAccessKey   string            `json:"providerSecretAccessKey"`
	Region                    string            `json:"region"`
	Tags                      map[string]string `json:"tags"`
}

type DiscoveryData struct {
	APIVersion                string      `json:"apiVersion"`
	Kind                      string      `json:"kind"`
	KeyName                   string      `json:"keyName"`
	Instances                 Instances   `json:"instances"`
	LoadBalancerSecurityGroup string      `json:"loadBalancerSecurityGroup"`
	Zones                     []string    `json:"zones"`
	ZoneToSubnetIDMap         interface{} `json:"zoneToSubnetIdMap"`
}

type Provider struct {
	ProviderAccessKeyID     string `json:"providerAccessKeyId"`
	ProviderSecretAccessKey string `json:"providerSecretAccessKey"`
	Region                  string `json:"region"`
}

type Instances struct {
	Ami                      string   `json:"ami"`
	AdditionalSecurityGroups []string `json:"additionalSecurityGroups"`
	AssociatePublicIPAddress bool     `json:"associatePublicIPAddress"`
	IamProfileName           string   `json:"iamProfileName"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
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
}, clusterConfiguration)

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return secret, nil
}

func clusterConfiguration(input *go_hook.HookInput) error {
	if len(input.Snapshots["provider_cluster_configuration"]) == 0 {
		return fmt.Errorf("%s", "Can't find Secret d8-provider-cluster-configuration in Namespace kube-system")
	}

	secret := input.Snapshots["provider_cluster_configuration"][0].(*v1.Secret)

	clusterConfiguration := secret.Data["cloud-provider-cluster-configuration.yaml"]

	cloudDiscoveryData := secret.Data["cloud-provider-discovery-data.json"]

	metaCfg, err := config.ParseConfigFromData(string(clusterConfiguration))
	if err != nil {
		return fmt.Errorf("validate cloud-provider-cluster-configuration.yaml: %v", err)
	}

	_, err = config.ValidateDiscoveryData(&cloudDiscoveryData, []string{})
	if err != nil {
		return fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
	}

	var provider Provider
	if err := json.Unmarshal(metaCfg.ProviderClusterConfig["provider"], &provider); err != nil {
		return err
	}

	var discoveryData DiscoveryData
	err = json.Unmarshal(cloudDiscoveryData, &discoveryData)
	if err != nil {
		return err
	}

	tags := make(map[string]string)
	if len(metaCfg.ProviderClusterConfig["tags"]) != 0 {
		if err := json.Unmarshal(metaCfg.ProviderClusterConfig["tags"], &tags); err != nil {
			return err
		}
	}

	values := InternalValues{
		KeyName:                   discoveryData.KeyName,
		LoadBalancerSecurityGroup: discoveryData.LoadBalancerSecurityGroup,
		Zones:                     discoveryData.Zones,
		ZoneToSubnetIDMap:         discoveryData.ZoneToSubnetIDMap,
		Instances:                 discoveryData.Instances,
		Region:                    provider.Region,
		ProviderAccessKeyID:       provider.ProviderAccessKeyID,
		ProviderSecretAccessKey:   provider.ProviderSecretAccessKey,
		Tags:                      tags,
	}

	input.Values.Set("cloudProviderAws.internal", values)

	return nil
}
