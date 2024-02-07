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
