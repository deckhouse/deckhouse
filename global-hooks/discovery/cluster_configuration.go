package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/candictl/pkg/config"
)

type ClusterConfigurationYaml struct {
	Content []byte
}

func (*ClusterConfigurationYaml) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := go_hook.ConvertUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	cc := &ClusterConfigurationYaml{}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	cc.Content = ccYaml

	return cc, err
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "clusterConfiguration",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			Filterable:        &ClusterConfigurationYaml{},
		},
	},
}, clusterConfiguration)

func clusterConfiguration(input *go_hook.HookInput) error {
	currentConfig, ok := input.Snapshots["clusterConfiguration"]

	// no cluster configuration — unset global value if there is one.
	if !ok {
		if input.Values.Values.ExistsP("global.clusterConfiguration") {
			input.Values.Remove("global.clusterConfiguration")
		}
	}

	if ok && len(currentConfig) > 0 {
		var err error

		// FilterResult is a YAML encoded as a JSON string. Unmarshal it.
		configYamlBytes := currentConfig[0].(*ClusterConfigurationYaml)

		var metaConfig *config.MetaConfig
		metaConfig, err = config.ParseConfigFromData(string(configYamlBytes.Content))
		if err != nil {
			return err
		}

		input.Values.Set("global.clusterConfiguration", metaConfig.ClusterConfig)

		if podSubnetCIDR, ok := metaConfig.ClusterConfig["podSubnetCIDR"]; ok {
			input.Values.Set("global.discovery.podSubnet", podSubnetCIDR)
		} else {
			return fmt.Errorf("no podSubnetCIDR field in clusterConfiguration")
		}

		if serviceSubnetCIDR, ok := metaConfig.ClusterConfig["serviceSubnetCIDR"]; ok {
			input.Values.Set("global.discovery.serviceSubnet", serviceSubnetCIDR)
		} else {
			return fmt.Errorf("no serviceSubnetCIDR field in clusterConfiguration")
		}

		if serviceSubnetCIDR, ok := metaConfig.ClusterConfig["clusterDomain"]; ok {
			input.Values.Set("global.discovery.clusterDomain", serviceSubnetCIDR)
		} else {
			return fmt.Errorf("no clusterDomain field in clusterConfiguration")
		}
	}

	return nil
}
