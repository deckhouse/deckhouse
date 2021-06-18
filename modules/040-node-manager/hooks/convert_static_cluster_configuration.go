package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func applyStaticClusterConfigurationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return secret.Data["static-cluster-configuration.yaml"], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "static_cluster_configuration",
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
			FilterFunc: applyStaticClusterConfigurationFilter,
		},
	},
}, convertStaticClusterConfigurationHandler)

func convertStaticClusterConfigurationHandler(input *go_hook.HookInput) error {
	secret := input.Snapshots["static_cluster_configuration"]
	if len(secret) == 0 {
		return nil
	}

	staticConfiguration, ok := secret[0].([]byte)
	if !ok {
		return fmt.Errorf("static_cluster_configuration filterFunc problem: expect []byte, got %T", staticConfiguration)
	}

	internalNetwork, err := internalNetworkFromStaticConfiguration(staticConfiguration)
	if err != nil {
		return err
	}

	input.Values.Set("nodeManager.internal.static.internalNetworkCIDRs", internalNetwork)
	return nil
}

func internalNetworkFromStaticConfiguration(data []byte) (interface{}, error) {
	var err error
	var metaConfig *config.MetaConfig

	metaConfig, err = config.ParseConfigFromData(string(data))
	if err != nil {
		return nil, err
	}

	intNet := metaConfig.StaticClusterConfig["internalNetworkCIDRs"]
	if intNet == nil {
		return []interface{}{}, nil
	}
	return intNet, nil
}
