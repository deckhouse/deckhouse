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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
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

func clusterConfiguration(ctx context.Context, input *go_hook.HookInput) error {
	secrets, err := sdkobjectpatch.UnmarshalToStruct[v1.Secret](input.Snapshots, "provider_cluster_configuration")
	if err != nil {
		return fmt.Errorf("can't unmarshal snapshot provider_cluster_configuration: %w", err)
	}

	if len(secrets) == 0 {
		return fmt.Errorf("can't find Secret d8-provider-cluster-configuration in Namespace kube-system")
	}

	secret := secrets[0]

	clusterConfiguration := secret.Data["cloud-provider-cluster-configuration.yaml"]

	cloudDiscoveryData := secret.Data["cloud-provider-discovery-data.json"]

	metaCfg, err := config.ParseConfigFromData(ctx, string(clusterConfiguration), infrastructureprovider.MetaConfigPreparatorProvider(
		infrastructureprovider.NewPreparatorProviderParamsWithoutLogger()))
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

	input.Values.Set("cloudProviderAws.internal.keyName", discoveryData.KeyName)
	input.Values.Set("cloudProviderAws.internal.loadBalancerSecurityGroup", discoveryData.LoadBalancerSecurityGroup)
	input.Values.Set("cloudProviderAws.internal.zones", discoveryData.Zones)
	input.Values.Set("cloudProviderAws.internal.zoneToSubnetIdMap", discoveryData.ZoneToSubnetIDMap)
	input.Values.Set("cloudProviderAws.internal.instances", discoveryData.Instances)
	input.Values.Set("cloudProviderAws.internal.region", provider.Region)
	input.Values.Set("cloudProviderAws.internal.providerAccessKeyId", provider.ProviderAccessKeyID)
	input.Values.Set("cloudProviderAws.internal.providerSecretAccessKey", provider.ProviderSecretAccessKey)
	input.Values.Set("cloudProviderAws.internal.tags", tags)

	return nil
}
