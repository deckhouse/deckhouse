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
	"log/slog"
	"net"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

const (
	controlPlaneManagerModuleConfigSnapshot = "controlPlaneManagerModuleConfig"
	clusterKubernetesConfigMapSnapshot      = "clusterKubernetesConfigMap"

	defaultVersionDriftMetricGroup = "D8ControlPlaneDefaultVersionDrift"
	defaultVersionDriftMetricName  = "d8_control_plane_default_version_drift"

	// automaticKubernetesVersion is the sentinel meaning "let Deckhouse pick the version".
	automaticKubernetesVersion = "Automatic"
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

// applyControlPlaneManagerKubernetesVersionFilter returns the raw kubernetesVersion from
// ModuleConfig/control-plane-manager settings, or "" when unset.
func applyControlPlaneManagerKubernetesVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "settings", "kubernetesVersion")
	if err != nil {
		return "", fmt.Errorf("nested string kubernetesVersion: %w", err)
	}
	return version, nil
}

// applyClusterKubernetesCurrentVersionFilter returns status.currentVersion from
// ConfigMap kube-system/d8-cluster-kubernetes (update-observer bookkeeping), or "" when absent.
func applyClusterKubernetesCurrentVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return "", fmt.Errorf("from unstructured: %w", err)
	}

	var status struct {
		CurrentVersion string `json:"currentVersion"`
	}
	if err := yaml.Unmarshal([]byte(cm.Data["status"]), &status); err != nil {
		return "", nil
	}
	return status.CurrentVersion, nil
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
			// Events on this ModuleConfig must re-run the hook immediately so targetKubernetesVersion
			// updates as soon as an operator patches kubernetesVersion — do not set ExecuteHookOnEvents: false
			// (unlike enable_cni.go, which only needs a one-shot read).
			Name:       controlPlaneManagerModuleConfigSnapshot,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"control-plane-manager"},
			},
			FilterFunc: applyControlPlaneManagerKubernetesVersionFilter,
		},
		{
			Name:       clusterKubernetesConfigMapSnapshot,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-cluster-kubernetes"}},
			FilterFunc:   applyClusterKubernetesCurrentVersionFilter,
		},
	},
}, clusterConfiguration)

func clusterConfiguration(ctx context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(defaultVersionDriftMetricGroup)

	mcVersion := ""
	mcSnaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, controlPlaneManagerModuleConfigSnapshot)
	if err != nil {
		input.Logger.Warn(
			"failed to unmarshal snapshot",
			slog.String("snapshot", controlPlaneManagerModuleConfigSnapshot),
			dlog.Err(err),
		)
	} else if len(mcSnaps) > 0 {
		mcVersion = mcSnaps[0]
	}

	ccRawVersion := ""

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

		kubernetesVersionFromMetaConfig, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
		if err != nil {
			return err
		}
		ccRawVersion = kubernetesVersionFromMetaConfig

		// Keep substituting Automatic → Default into global.clusterConfiguration for backward
		// compatibility during the ClusterConfiguration.kubernetesVersion deprecation window.
		// Declared target lives in global.discovery.targetKubernetesVersion instead.
		//
		// TODO(kubernetesVersion-deprecation): T+2 remove — this substitution goes with the field.
		// Note it is not the same thing as clusterConfiguration.kubernetesVersion inside the
		// control-plane templates: there the key is a render-context value that
		// modules/040-control-plane-manager/templates/daemonset.yaml sets to the throttled
		// effective version, and it stays.
		if kubernetesVersionFromMetaConfig == automaticKubernetesVersion {
			b, _ := json.Marshal(hooks.DefaultKubernetesVersion)
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

	target, isAutomatic := resolveTargetKubernetesVersion(mcVersion, ccRawVersion, hooks.DefaultKubernetesVersion)
	input.Values.Set("global.discovery.targetKubernetesVersion", target)
	input.Values.Set("global.discovery.kubernetesVersionIsAutomatic", isAutomatic)

	// Soft drift signal only: always publish the honest target above. In Automatic mode, if that
	// target sits more than one minor below the running control plane, raise an alert — do not
	// suppress the value (effective_kubernetes_version / get_crds throttle the actual rollout).
	if isAutomatic {
		currentVersion := ""
		cmSnaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, clusterKubernetesConfigMapSnapshot)
		if err != nil {
			input.Logger.Warn(
				"failed to unmarshal snapshot",
				slog.String("snapshot", clusterKubernetesConfigMapSnapshot),
				dlog.Err(err),
			)
		} else if len(cmSnaps) > 0 {
			currentVersion = cmSnaps[0]
		}
		if currentVersion != "" {
			if drifted, err := isMoreThanOneMinorBelow(target, currentVersion); err == nil && drifted {
				input.MetricsCollector.Set(defaultVersionDriftMetricName, 1, map[string]string{}, metrics.WithGroup(defaultVersionDriftMetricGroup))
			}
		}
	}

	return nil
}

// resolveTargetKubernetesVersion returns the operator-declared Kubernetes version and whether the
// cluster is in Automatic mode (tracking the Deckhouse default).
//
// The ModuleConfig setting wins whenever it is present, including when it holds "Automatic" —
// presence of the field, not its value, decides which document owns the version. Setting
// "Automatic" there is a deliberate act meaning "let Deckhouse choose", so it must not silently
// defer to a leftover ClusterConfiguration pin from bootstrap. The field has no schema default,
// so it is never present unless someone wrote it.
//
// Only when ModuleConfig says nothing at all does the deprecated ClusterConfiguration field apply;
// "Automatic" there is not a pin either and falls through to the Deckhouse default.
// TODO(kubernetesVersion-deprecation): T+2 remove — when kubernetesVersion is removed from
// ClusterConfiguration, drop the ccVersion parameter and the isPinnedKubernetesVersion branch —
// the signature collapses to (mcVersion, defaultVersion) and an absent ModuleConfig setting simply
// means the Deckhouse default. Removing that branch is what silently retargets a cluster that
// never migrated, so the release doing it must also declare the kubernetesVersionMigrated
// requirement (modules/040-control-plane-manager/requirements/check.go).
func resolveTargetKubernetesVersion(mcVersion, ccVersion, defaultVersion string) (string, bool) {
	switch {
	case mcVersion == automaticKubernetesVersion:
		return defaultVersion, true
	case mcVersion != "":
		return mcVersion, false
	case isPinnedKubernetesVersion(ccVersion):
		return ccVersion, false
	default:
		return defaultVersion, true
	}
}

// TODO(kubernetesVersion-deprecation): T+2 remove — dies together with the ClusterConfiguration field. It is
// only ever applied to the ClusterConfiguration value — for the ModuleConfig setting presence
// decides, not pinning.
func isPinnedKubernetesVersion(version string) bool {
	return version != "" && version != automaticKubernetesVersion
}

// isMoreThanOneMinorBelow reports whether candidate is more than 1 minor version below current.
func isMoreThanOneMinorBelow(candidate, current string) (bool, error) {
	candidateV, err := semver.NewVersion(candidate)
	if err != nil {
		return false, err
	}
	currentV, err := semver.NewVersion(current)
	if err != nil {
		return false, err
	}
	if candidateV.Major() != currentV.Major() {
		return candidateV.Major() < currentV.Major(), nil
	}
	return currentV.Minor() > 0 && candidateV.Minor() < currentV.Minor()-1, nil
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
		return result, fmt.Errorf("marshal json: %w", err)
	}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return result, fmt.Errorf("unmarshal: %w", err)
	}
	return result, nil
}
