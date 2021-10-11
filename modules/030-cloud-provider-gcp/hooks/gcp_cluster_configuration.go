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
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type ProviderClusterConfigurationValues struct {
	Provider Provider `json:"provider"`
	SSHKey   string   `json:"sshKey"`
}

type Provider struct {
	ServiceAccountJSON string `json:"serviceAccountJSON"`
	Region             string `json:"region"`
}

type ProviderDiscoveryDataValues struct {
	Instances         Instances `json:"instances"`
	DisableExternalIP bool      `json:"disableExternalIP"`
	NetworkName       string    `json:"networkName"`
	SubnetworkName    string    `json:"subnetworkName"`
	Zones             []string  `json:"zones"`
}

type Instances struct {
	Image       string            `json:"image"`
	DiskSizeGb  int64             `json:"diskSizeGb"`
	DiskType    string            `json:"diskType"`
	NetworkTags []string          `json:"networkTags"`
	Labels      map[string]string `json:"labels"`
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
	secret := input.Snapshots["provider_cluster_configuration"][0].(*v1.Secret)

	clusterConfigurationYAML := secret.Data["cloud-provider-cluster-configuration.yaml"]
	var clusterConfiguration unstructured.Unstructured
	err := yaml.Unmarshal(clusterConfigurationYAML, &clusterConfiguration)
	if err != nil {
		return err
	}

	discoveryDataJSON := secret.Data["cloud-provider-discovery-data.json"]
	var providerDiscoveryData unstructured.Unstructured
	err = json.Unmarshal(discoveryDataJSON, &providerDiscoveryData)
	if err != nil {
		return err
	}

	metaCfg, err := config.ParseConfigFromData(string(clusterConfigurationYAML))
	if err != nil {
		return fmt.Errorf("validate cloud-provider-cluster-configuration.yaml: %v", err)
	}

	_, err = config.ValidateDiscoveryData(&discoveryDataJSON)
	if err != nil {
		return fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
	}

	var provider Provider
	if err := json.Unmarshal(metaCfg.ProviderClusterConfig["provider"], &provider); err != nil {
		return err
	}

	clusterConfigurationValues := ProviderClusterConfigurationValues{
		Provider: Provider{
			Region:             provider.Region,
			ServiceAccountJSON: provider.ServiceAccountJSON,
		},
		SSHKey: clusterConfiguration.Object["sshKey"].(string),
	}

	input.Values.Set("cloudProviderGcp.internal.providerClusterConfiguration", clusterConfigurationValues)

	instances, _, err := unstructured.NestedMap(providerDiscoveryData.Object, "instances")
	if err != nil {
		return err
	}

	var networkTags []string
	for _, networkTag := range instances["networkTags"].([]interface{}) {
		networkTags = append(networkTags, networkTag.(string))
	}

	labels := make(map[string]string)
	for k, v := range instances["labels"].(map[string]interface{}) {
		labels[k] = v.(string)
	}

	zones, _, err := unstructured.NestedStringSlice(providerDiscoveryData.Object, "zones")
	if err != nil {
		return err
	}

	discoveryDataValues := ProviderDiscoveryDataValues{
		DisableExternalIP: providerDiscoveryData.Object["disableExternalIP"].(bool),
		NetworkName:       providerDiscoveryData.Object["networkName"].(string),
		SubnetworkName:    providerDiscoveryData.Object["subnetworkName"].(string),
		Instances: Instances{
			Image:       instances["image"].(string),
			DiskSizeGb:  instances["diskSizeGb"].(int64),
			DiskType:    instances["diskType"].(string),
			NetworkTags: networkTags,
			Labels:      labels,
		},
		Zones: zones,
	}

	input.Values.Set("cloudProviderGcp.internal.providerDiscoveryData", discoveryDataValues)

	return nil
}
