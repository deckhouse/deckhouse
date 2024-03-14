// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type ClusterConfigurationYaml struct {
	Content []byte
}

func applyClusterConfigurationYamlFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
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
			FilterFunc:        applyClusterConfigurationYamlFilter,
		},
	},
}, clusterConfiguration)

func clusterConfiguration(input *go_hook.HookInput) error {
	currentConfig, ok := input.Snapshots["clusterConfiguration"]

	// no cluster configuration — unset global value if there is one.
	if !ok {
		if input.Values.Exists("global.clusterConfiguration") {
			input.Values.Remove("global.clusterConfiguration")
		}
	}

	if ok && len(currentConfig) > 0 {
		// FilterResult is a YAML encoded as a JSON string. Unmarshal it.
		configYamlBytes := currentConfig[0].(*ClusterConfigurationYaml)

		var metaConfig *config.MetaConfig
		metaConfig, err := config.ParseConfigFromData(string(configYamlBytes.Content))
		if err != nil {
			return err
		}

		kubernetesVersionFromMetaConfig, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
		if err != nil {
			return err
		}

		if kubernetesVersionFromMetaConfig == "Automatic" {
			b, _ := json.Marshal(config.DefaultKubernetesVersion)
			metaConfig.ClusterConfig["kubernetesVersion"] = b
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

		if clusterDomain, ok := metaConfig.ClusterConfig["clusterDomain"]; ok {
			input.Values.Set("global.discovery.clusterDomain", clusterDomain)
		} else {
			return fmt.Errorf("no clusterDomain field in clusterConfiguration")
		}

		err = maxNodesAmountMetric(input, metaConfig.ClusterConfig["podSubnetCIDR"], metaConfig.ClusterConfig["podSubnetNodeCIDRPrefix"])
		if err != nil {
			return err
		}
	}

	return nil
}

func maxNodesAmountMetric(input *go_hook.HookInput, podSubnetCIDR json.RawMessage, podSubnetNodeCIDRPrefix json.RawMessage) error {
	var res string
	err := json.Unmarshal(podSubnetCIDR, &res)
	if err != nil {
		return fmt.Errorf("cannot unmarshal %v", podSubnetCIDR)
	}

	_, ipnet, err := net.ParseCIDR(res)
	if err != nil {
		return fmt.Errorf("cannot parse CIDR from podSubnetCIDR %s: %v", res, err)
	}

	podSubnetMaskSize, _ := ipnet.Mask.Size()

	err = json.Unmarshal(podSubnetNodeCIDRPrefix, &res)
	if err != nil {
		return fmt.Errorf("cannot unmarshal %v", podSubnetNodeCIDRPrefix)
	}

	nodeMaskSize, err := strconv.Atoi(res)
	if err != nil {
		return fmt.Errorf("cannot convert to integer podSubnetNodeCIDRPrefix %s: %v", res, err)
	}

	diff := nodeMaskSize - podSubnetMaskSize
	if diff < 0 {
		return fmt.Errorf("node mask size:%d must be bigger than pod subnet mask size:%d", nodeMaskSize, podSubnetMaskSize)
	}

	maxNodesAmount := 1 << diff

	input.MetricsCollector.Set("d8_max_nodes_amount_by_pod_cidr", float64(maxNodesAmount), nil)
	return nil
}

func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	b, err := message.MarshalJSON()
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}
