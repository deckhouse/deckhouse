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
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

type ClusterConfigurationYaml struct {
	Content []byte
}

func applyClusterConfigurationYamlFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	cc := &ClusterConfigurationYaml{}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	cc.Content = ccYaml

	return cc, nil
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

func clusterConfiguration(ctx context.Context, input *go_hook.HookInput) error {
	currentConfig, err := sdkobjectpatch.UnmarshalToStruct[ClusterConfigurationYaml](input.Snapshots, "clusterConfiguration")
	if err != nil {
		return fmt.Errorf("failed to unmarshal clusterConfiguration snapshot: %w", err)
	}

	// no cluster configuration — unset global value if there is one.
	if len(currentConfig) == 0 {
		if input.Values.Exists("global.clusterConfiguration") {
			input.Values.Remove("global.clusterConfiguration")
		}
	}

	if len(currentConfig) > 0 {
		// FilterResult is a YAML encoded as a JSON string. Unmarshal it.
		configYamlBytes := currentConfig[0]

		var metaConfig *config.MetaConfig
		// we use dummy preparator because we do not need any preparation and validation from cloud providers
		// we use only ClusterConfiguration here
		metaConfig, err = config.ParseConfigFromData(ctx, string(configYamlBytes.Content), config.DummyPreparatorProvider(), nil)
		if err != nil {
			return fmt.Errorf("parse config from data: %w", err)
		}

		kubernetesVersionFromMetaConfig, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
		if err != nil {
			return err
		}

		if kubernetesVersionFromMetaConfig == "Automatic" {
			b, _ := json.Marshal(hooks.DefaultKubernetesVersion)
			metaConfig.ClusterConfig["kubernetesVersion"] = b
		}

		input.Values.Set("global.clusterConfiguration", metaConfig.ClusterConfig)

		if podSubnetCIDR, ok := metaConfig.ClusterConfig["podSubnetCIDR"]; ok {
			podSubnetValue, err := joinDualStackCIDR(podSubnetCIDR, metaConfig.ClusterConfig["podSubnetCIDRIPv6"])
			if err != nil {
				return fmt.Errorf("build podSubnet value: %w", err)
			}
			input.Values.Set("global.discovery.podSubnet", podSubnetValue)
		} else {
			return fmt.Errorf("no podSubnetCIDR field in clusterConfiguration")
		}

		if serviceSubnetCIDR, ok := metaConfig.ClusterConfig["serviceSubnetCIDR"]; ok {
			serviceSubnetValue, err := joinDualStackCIDR(serviceSubnetCIDR, metaConfig.ClusterConfig["serviceSubnetCIDRIPv6"])
			if err != nil {
				return fmt.Errorf("build serviceSubnet value: %w", err)
			}
			input.Values.Set("global.discovery.serviceSubnet", serviceSubnetValue)
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

// joinDualStackCIDR returns a JSON-encoded string equal to v4, optionally followed by
// ",<v6>" when v6 is non-empty. The result is always a json.RawMessage holding a JSON string,
// so it can be passed to input.Values.Set directly. Returns an error when v4 is missing or
// cannot be parsed; v6 parse errors are reported as a hard error to avoid silently dropping
// IPv6 subnets that an operator explicitly configured.
func joinDualStackCIDR(v4, v6 json.RawMessage) (json.RawMessage, error) {
	var v4Str string
	if err := json.Unmarshal(v4, &v4Str); err != nil {
		return nil, fmt.Errorf("unmarshal IPv4 CIDR: %w", err)
	}
	if v4Str == "" {
		return nil, fmt.Errorf("IPv4 CIDR is empty")
	}
	if len(v6) == 0 {
		return v4, nil
	}
	var v6Str string
	if err := json.Unmarshal(v6, &v6Str); err != nil {
		return nil, fmt.Errorf("unmarshal IPv6 CIDR: %w", err)
	}
	if v6Str == "" {
		return v4, nil
	}
	res, err := json.Marshal(v4Str + "," + v6Str)
	if err != nil {
		return nil, fmt.Errorf("marshal joined CIDR: %w", err)
	}
	return res, nil
}

func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	b, err := message.MarshalJSON()
	if err != nil {
		return result, fmt.Errorf("marshal json: %w", err)
	}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return result, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}
