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
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

// maxUsedK8sVersionSecretKey is Deckhouse's own bookkeeping in the d8-cluster-configuration Secret,
// not a ClusterConfiguration field. It lives next to its only reader, the FilterFunc below; the
// value it carries is consumed by the soft guard in target_kubernetes_version.go.
const maxUsedK8sVersionSecretKey = "maxUsedControlPlaneKubernetesVersion"

type ClusterConfigurationYaml struct {
	Content []byte
	// MaxUsed is maxUsedControlPlaneKubernetesVersion from the same Secret (baseline for soft-guard).
	MaxUsed string
	// KubernetesVersion is the raw ClusterConfiguration.kubernetesVersion, read with a plain
	// unmarshal so target_kubernetes_version.go does not need config.ParseConfigFromData: schema
	// validation of the whole document is this hook's job, not the version hook's.
	KubernetesVersion string
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
	if v, ok := secret.Data[maxUsedK8sVersionSecretKey]; ok {
		cc.MaxUsed = string(v)
	}

	// Best-effort: a malformed document is this hook's problem to report, and the version hook
	// must stay independent of it. Same shape helm.go and both Python webhooks read.
	var ccDoc struct {
		KubernetesVersion string `json:"kubernetesVersion"`
	}
	if err := yaml.Unmarshal(ccYaml, &ccDoc); err == nil {
		cc.KubernetesVersion = ccDoc.KubernetesVersion
	}

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
		{
			// The network parameters are being migrated into this ModuleConfig, so an operator
			// patching it must re-run the hook immediately: do not set ExecuteHookOnEvents: false.
			// Own snapshot rather than the one target_kubernetes_version.go uses — the two hooks read
			// different settings and must not inherit each other's failure modes.
			Name:       networkModuleConfigSnapshot,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"control-plane-manager"},
			},
			FilterFunc: applyControlPlaneManagerNetworkFilter,
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
		// we use dummy validator because no provider validation is needed here from cloud providers
		// we use only ClusterConfiguration here
		metaConfig, err = config.ParseConfigFromData(ctx, string(configYamlBytes.Content), config.DummyValidatorProvider(), nil)
		if err != nil {
			return fmt.Errorf("parse config from data: %w", err)
		}

		if raw, ok := metaConfig.ClusterConfig["kubernetesVersion"]; ok && len(raw) > 0 && string(raw) != "null" {
			kubernetesVersionFromMetaConfig, err := rawMessageToString(raw)
			if err != nil {
				return err
			}

			// Keep substituting Automatic → Default into global.clusterConfiguration for backward
			// compatibility during the ClusterConfiguration.kubernetesVersion deprecation window.
			// Declared target lives in global.discovery.targetKubernetesVersion instead.
			if kubernetesVersionFromMetaConfig == automaticKubernetesVersion {
				// ClusterConfig values are json.RawMessage, so the version has to go back in encoded.
				defaultKubernetesVersionJSON, _ := json.Marshal(hooks.DefaultKubernetesVersion)
				metaConfig.ClusterConfig["kubernetesVersion"] = defaultKubernetesVersionJSON
			}
		}

		// The three network parameters are resolved "ModuleConfig, otherwise ClusterConfiguration" and
		// published into both key families. The ModuleConfig read is fail-open (see
		// readNetworkModuleConfig); the Secret above stays fail-closed.
		ccNetwork := networkSettings{
			PodSubnetCIDR:           clusterConfigString(metaConfig.ClusterConfig, "podSubnetCIDR"),
			ServiceSubnetCIDR:       clusterConfigString(metaConfig.ClusterConfig, "serviceSubnetCIDR"),
			PodSubnetNodeCIDRPrefix: clusterConfigString(metaConfig.ClusterConfig, "podSubnetNodeCIDRPrefix"),
		}
		network := resolveNetwork(readNetworkModuleConfig(input), ccNetwork)
		logNetworkFallback(input, network)

		// Neither CIDR has a default, so a value missing from both documents is a real error: an empty
		// string here would reach --cluster-cidr and --service-cluster-ip-range. The prefix cannot be
		// empty at this point — resolveNetwork falls back to 24.
		if network.PodSubnetCIDR == "" {
			return fmt.Errorf("podSubnetCIDR is set neither in ModuleConfig control-plane-manager (settings.network) nor in ClusterConfiguration")
		}
		if network.ServiceSubnetCIDR == "" {
			return fmt.Errorf("serviceSubnetCIDR is set neither in ModuleConfig control-plane-manager (settings.network) nor in ClusterConfiguration")
		}

		// Substituted back so every template reading global.clusterConfiguration.* — the CPM
		// $tpl_context, _envs_for_proxy.tpl, 61_proxy.sh.tpl — sees the resolved value without being
		// touched. Written before global.clusterConfiguration is published, so the two agree.
		setClusterConfigString(metaConfig.ClusterConfig, "podSubnetCIDR", network.PodSubnetCIDR)
		setClusterConfigString(metaConfig.ClusterConfig, "serviceSubnetCIDR", network.ServiceSubnetCIDR)
		setClusterConfigString(metaConfig.ClusterConfig, "podSubnetNodeCIDRPrefix", network.PodSubnetNodeCIDRPrefix)

		input.Values.Set("global.clusterConfiguration", metaConfig.ClusterConfig)

		input.Values.Set("global.discovery.podSubnet", network.PodSubnetCIDR)
		input.Values.Set("global.discovery.serviceSubnet", network.ServiceSubnetCIDR)
		input.Values.Set("global.discovery.podSubnetNodeCIDRPrefix", network.PodSubnetNodeCIDRPrefix)

		if clusterDomain, ok := metaConfig.ClusterConfig["clusterDomain"]; ok {
			input.Values.Set("global.discovery.clusterDomain", clusterDomain)
		} else {
			return fmt.Errorf("no clusterDomain field in clusterConfiguration")
		}

		err = maxNodesAmountMetric(input, network.PodSubnetCIDR, network.PodSubnetNodeCIDRPrefix)
		if err != nil {
			return err
		}
	}

	return nil
}

// clusterConfigString reads a string field out of the parsed ClusterConfiguration. A missing field
// and a field of another type both come back empty, which is what the resolver treats as "not set
// here". The schema keeps all three of these strings.
func clusterConfigString(cfg map[string]json.RawMessage, key string) string {
	raw, ok := cfg[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v
}

func setClusterConfigString(cfg map[string]json.RawMessage, key, value string) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	cfg[key] = encoded
}

// Both arguments are resolved values (ModuleConfig or ClusterConfiguration), so the metric describes
// the network the cluster actually runs with rather than whatever the deprecated document still says.
func maxNodesAmountMetric(input *go_hook.HookInput, podSubnetCIDR string, podSubnetNodeCIDRPrefix string) error {
	_, ipnet, err := net.ParseCIDR(podSubnetCIDR)
	if err != nil {
		return fmt.Errorf("cannot parse CIDR from podSubnetCIDR %s: %v", podSubnetCIDR, err)
	}

	podSubnetMaskSize, _ := ipnet.Mask.Size()

	nodeMaskSize, err := strconv.Atoi(podSubnetNodeCIDRPrefix)
	if err != nil {
		return fmt.Errorf("cannot convert to integer podSubnetNodeCIDRPrefix %s: %v", podSubnetNodeCIDRPrefix, err)
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
		return result, fmt.Errorf("marshal json: %w", err)
	}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return result, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}
