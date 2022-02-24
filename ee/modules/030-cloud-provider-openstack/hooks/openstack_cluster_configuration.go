/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
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
			FilterFunc: filterClusterConfigurationSecret,
		},
	},
}, handleProviderConfiguration)

func handleProviderConfiguration(input *go_hook.HookInput) error {
	snap := input.Snapshots["secret"]
	var conf clusterConfiguration
	if len(snap) == 0 {
		conf = clusterConfiguration{
			DiscoveryData:         []byte("{}"),
			ProviderConfiguration: []byte("{}"),
		}
	} else {
		conf = snap[0].(clusterConfiguration)
	}

	//   values::unset cloudProviderOpenstack.internal.connection
	//  provider='{}'
	//  tags='{}'
	//  provider_cluster_configuration_yaml=$(echo "$1" | jq -r .provider_cluster_configuration)
	//  if [[ "$provider_cluster_configuration_yaml" != "null" ]]; then
	//    provider_cluster_configuration=$(echo "$provider_cluster_configuration_yaml" | deckhouse-controller helper cluster-configuration | jq '.providerClusterConfiguration')
	//    provider=$(echo "$provider_cluster_configuration" | jq '.provider | . //= {}')
	//    tags=$(echo "$provider_cluster_configuration" | jq '.tags | . //= {}')
	//  fi
	//
	//  provider_discovery_data=$(echo "$1" | jq -r '
	//    if (.provider_discovery_data=="" or .provider_discovery_data==null) then .provider_discovery_data={
	//      "instances": {},
	//      "loadBalancer": {}
	//    } end | .provider_discovery_data')

	input.Values.Remove("cloudProviderOpenstack.internal.connection")

}

func filterClusterConfigurationSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	discoveryData := sec.Data["cloud-provider-discovery-data.json"]
	providerConf := sec.Data["cloud-provider-cluster-configuration.yaml"]

	if len(discoveryData) == 0 {
		discoveryData = json.RawMessage("{}")
	}

	if len(providerConf) == 0 {
		providerConf = []byte("{}")
	}

	return clusterConfiguration{
		ProviderConfiguration: providerConf,
		DiscoveryData:         discoveryData,
	}, nil
}

type clusterConfiguration struct {
	DiscoveryData         []byte
	ProviderConfiguration []byte
}
