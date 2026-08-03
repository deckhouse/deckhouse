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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	desiredVersionConfigMapName      = "d8-cluster-kubernetes"
	desiredVersionConfigMapNamespace = "kube-system"

	maxUsedK8sVersionSecretKey = "maxUsedControlPlaneKubernetesVersion"
	maxK8sVersionLabelKey      = "max-k8s-version"

	defaultVersionDriftMetricGroup = "D8ControlPlaneDefaultVersionDrift"
	defaultVersionDriftMetricName  = "d8_control_plane_default_version_drift"

	// automaticKubernetesVersion is the sentinel meaning "let Deckhouse pick the version".
	automaticKubernetesVersion = "Automatic"
)

type ClusterConfigurationYaml struct {
	Content []byte
	// MaxUsed is maxUsedControlPlaneKubernetesVersion from the same Secret (baseline for soft-guard).
	MaxUsed string
}

// clusterKubernetesSnapshot carries freeze inputs + the raw data.spec for no-op detection.
// Labels/status are owned by update-observer; this hook only writes data.spec.
type clusterKubernetesSnapshot struct {
	MaxUsed        string
	CurrentVersion string
	DesiredVersion string
	SpecYAML       string
}

// configMapSpec mirrors the Spec struct update-observer reads/writes for the ConfigMap "spec" key.
type configMapSpec struct {
	DesiredVersion string `json:"desiredVersion"`
	UpdateMode     string `json:"updateMode"`
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

func applyClusterKubernetesConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	snap := clusterKubernetesSnapshot{
		MaxUsed:  cm.Labels[maxK8sVersionLabelKey],
		SpecYAML: cm.Data["spec"],
	}

	var status struct {
		CurrentVersion string `json:"currentVersion"`
	}
	if err := yaml.Unmarshal([]byte(cm.Data["status"]), &status); err == nil {
		snap.CurrentVersion = status.CurrentVersion
	}

	var spec configMapSpec
	if err := yaml.Unmarshal([]byte(cm.Data["spec"]), &spec); err == nil {
		snap.DesiredVersion = spec.DesiredVersion
	}

	return snap, nil
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
				NameSelector: &types.NameSelector{MatchNames: []string{desiredVersionConfigMapNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{desiredVersionConfigMapName}},
			FilterFunc:   applyClusterKubernetesConfigMapFilter,
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

	var cmSnap clusterKubernetesSnapshot
	cmSnaps, err := sdkobjectpatch.UnmarshalToStruct[clusterKubernetesSnapshot](input.Snapshots, clusterKubernetesConfigMapSnapshot)
	if err != nil {
		input.Logger.Warn(
			"failed to unmarshal snapshot",
			slog.String("snapshot", clusterKubernetesConfigMapSnapshot),
			dlog.Err(err),
		)
	} else if len(cmSnaps) > 0 {
		cmSnap = cmSnaps[0]
	}

	ccRawVersion := ""
	secretMaxUsed := ""

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
		secretMaxUsed = configYamlBytes.MaxUsed

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
			ccRawVersion = kubernetesVersionFromMetaConfig

			// Keep substituting Automatic → Default into global.clusterConfiguration for backward
			// compatibility during the ClusterConfiguration.kubernetesVersion deprecation window.
			// Declared target lives in global.discovery.targetKubernetesVersion instead.
			//
			// TODO(kubernetesVersion-deprecation): T+1 remove — this substitution goes with the CC field.
			if kubernetesVersionFromMetaConfig == automaticKubernetesVersion {
				b, _ := json.Marshal(hooks.DefaultKubernetesVersion)
				metaConfig.ClusterConfig["kubernetesVersion"] = b
			}
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

	// Soft-guard (§C.2): only Automatic. Manual pins are admission-filtered.
	// When Default is below the maxUsed−1 window, FREEZE the digit (previous desired, else current)
	// but keep isAutomatic=true / updateMode=Automatic and raise the drift metric.
	publishedTarget := target
	if isAutomatic {
		maxUsed := cmSnap.MaxUsed
		if maxUsed == "" {
			maxUsed = secretMaxUsed
		}
		if maxUsed != "" {
			inWindow, err := kubernetesVersionInMaxUsedWindow(target, maxUsed)
			if err == nil && !inWindow {
				frozen := cmSnap.DesiredVersion
				if frozen == "" {
					frozen = cmSnap.CurrentVersion
				}
				if frozen != "" {
					publishedTarget = frozen
				}
				input.MetricsCollector.Set(
					defaultVersionDriftMetricName,
					1,
					map[string]string{},
					metrics.WithGroup(defaultVersionDriftMetricGroup),
				)
			}
		}
		// No baseline → fail-open: publish Default + Automatic.
	}

	input.Values.Set("global.discovery.targetKubernetesVersion", publishedTarget)
	input.Values.Set("global.discovery.kubernetesVersionIsAutomatic", isAutomatic)

	return publishDesiredKubernetesVersionSpec(input, publishedTarget, isAutomatic, cmSnap.SpecYAML)
}

// publishDesiredKubernetesVersionSpec is the single writer of data.spec on
// kube-system/d8-cluster-kubernetes. It never touches status/labels (update-observer owns those).
func publishDesiredKubernetesVersionSpec(input *go_hook.HookInput, desired string, isAutomatic bool, existingSpecYAML string) error {
	if desired == "" {
		return nil
	}

	updateMode := "Manual"
	if isAutomatic {
		updateMode = "Automatic"
	}

	specBytes, err := yaml.Marshal(configMapSpec{DesiredVersion: desired, UpdateMode: updateMode})
	if err != nil {
		return err
	}
	specYAML := string(specBytes)

	if existingSpecYAML == specYAML {
		return nil
	}

	input.PatchCollector.CreateIfNotExists(&v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredVersionConfigMapName,
			Namespace: desiredVersionConfigMapNamespace,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
		Data: map[string]string{"spec": specYAML},
	})

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"spec": specYAML,
		},
	}
	input.PatchCollector.PatchWithMerge(
		patch,
		"v1",
		"ConfigMap",
		desiredVersionConfigMapNamespace,
		desiredVersionConfigMapName,
		object_patch.WithIgnoreMissingObject(),
	)

	return nil
}

// resolveTargetKubernetesVersion returns the operator-declared Kubernetes version and whether the
// cluster is in Automatic mode (tracking the Deckhouse default).
//
// The ModuleConfig setting wins whenever it is present, including when it holds "Automatic" —
// presence of the field, not its value, decides which document owns the version.
//
// Only when ModuleConfig says nothing at all does the deprecated ClusterConfiguration field apply;
// "Automatic" there is not a pin either and falls through to the Deckhouse default.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — drop CC fallback branch
// (isPinnedKubernetesVersion / ccVersion). After T+1 only MC → Default.
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

// TODO(kubernetesVersion-deprecation): T+1 remove — dies together with the ClusterConfiguration field.
func isPinnedKubernetesVersion(version string) bool {
	return version != "" && version != automaticKubernetesVersion
}

// kubernetesVersionInMaxUsedWindow reports whether target is within the maxUsed−1 floor window
// (same formula as admission rejectKubernetesVersionBelowMaxUsed).
//
// NOTE(kubernetesVersion-deprecation): keep — floor survives CC field removal.
func kubernetesVersionInMaxUsedWindow(target, maxUsed string) (bool, error) {
	targetV, err := semver.NewVersion(target)
	if err != nil {
		return false, err
	}
	maxUsedV, err := semver.NewVersion(maxUsed)
	if err != nil {
		return false, err
	}

	switch {
	case targetV.Major() > maxUsedV.Major():
		return true, nil
	case targetV.Major() == maxUsedV.Major() && targetV.Minor()+1 >= maxUsedV.Minor():
		return true, nil
	default:
		return false, nil
	}
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
