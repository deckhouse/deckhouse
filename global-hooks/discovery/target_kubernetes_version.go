// Copyright 2026 Flant JSC
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

// This hook owns the *declared* Kubernetes version: it resolves ModuleConfig → ClusterConfiguration
// → Deckhouse default, publishes global.discovery.targetKubernetesVersion, and is the single writer
// of data.spec on ConfigMap kube-system/d8-cluster-kubernetes.
//
// Three different "Kubernetes versions" exist in this system; do not confuse them:
//
//	global.discovery.kubernetesVersion                    actual, polled from apiservers
//	                                                      (kubernetes_version.go — NOT this file)
//	global.discovery.targetKubernetesVersion              declared goal (this file)
//	controlPlaneManager.internal.effectiveKubernetesVersion  throttled, one minor at a time
//	                                                      (control-plane-manager/hooks)
//
// Split out of cluster_configuration.go on purpose: that hook returns early on any malformed or
// incomplete ClusterConfiguration field (podSubnetCIDR, serviceSubnetCIDR, clusterDomain, the
// pod-CIDR metric). While the two lived together, an unrelated CIDR typo stopped the version from
// being published at all and left control-plane-manager unable to converge.

package hooks

import (
	"context"
	"fmt"
	"log/slog"
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

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Own snapshot of the same Secret cluster_configuration.go watches. Snapshot names are
			// hook-scoped, and four global hooks already bind this Secret, so this adds no new
			// coupling — it only keeps this hook independent of the other one's failure modes.
			Name:              targetVersionClusterConfigSnapshot,
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
}, targetKubernetesVersion)

const (
	targetVersionClusterConfigSnapshot      = "clusterConfigurationForTargetVersion"
	controlPlaneManagerModuleConfigSnapshot = "controlPlaneManagerModuleConfig"
	clusterKubernetesConfigMapSnapshot      = "clusterKubernetesConfigMap"

	desiredVersionConfigMapName      = "d8-cluster-kubernetes"
	desiredVersionConfigMapNamespace = "kube-system"

	maxUsedK8sVersionSecretKey = "maxUsedControlPlaneKubernetesVersion"
	maxK8sVersionLabelKey      = "max-k8s-version"

	defaultVersionDriftMetricGroup = "D8ControlPlaneDefaultVersionDrift"
	defaultVersionDriftMetricName  = "d8_control_plane_default_version_drift"

	// automaticKubernetesVersion is the ClusterConfiguration sentinel and a deprecated
	// ModuleConfig alias of Default ("track Deckhouse default").
	//
	// TODO(kubernetesVersion-deprecation): T+1 remove — drop the Automatic alias everywhere
	// (MC enum, this constant, isTrackDefaultKubernetesVersion). After that only Default
	// remains as the ModuleConfig track-default sentinel; CC field itself is also gone.
	automaticKubernetesVersion = "Automatic"
	// defaultKubernetesVersionSentinel is the ModuleConfig-recommended name for "track Deckhouse default".
	// Prefer this over Automatic in new configs and docs.
	defaultKubernetesVersionSentinel = "Default"
)

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

func targetKubernetesVersion(_ context.Context, input *go_hook.HookInput) error {
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
	ccSnaps, err := sdkobjectpatch.UnmarshalToStruct[ClusterConfigurationYaml](input.Snapshots, targetVersionClusterConfigSnapshot)
	if err != nil {
		// Deliberately not fatal: a broken ClusterConfiguration must not stop the version from
		// being published. cluster_configuration.go is the hook that reports that document's health.
		input.Logger.Warn(
			"failed to unmarshal snapshot",
			slog.String("snapshot", targetVersionClusterConfigSnapshot),
			dlog.Err(err),
		)
	} else if len(ccSnaps) > 0 {
		ccRawVersion = ccSnaps[0].KubernetesVersion
		secretMaxUsed = ccSnaps[0].MaxUsed
	}

	target, isDefault := resolveTargetKubernetesVersion(mcVersion, ccRawVersion, hooks.DefaultKubernetesVersion)

	// TODO(E2E-KV): temporary stand debug logs — remove before final PR (`rg E2E-KV`).
	input.Logger.Info("E2E-KV resolve",
		slog.String("mc", mcVersion),
		slog.String("cc", ccRawVersion),
		slog.String("target", target),
		slog.Bool("isDefault", isDefault),
	)

	// Soft-guard: only track-default mode (MC Default, or deprecated Automatic alias, or
	// unset→Default). Manual pins are admission-filtered and skip this block.
	// When Default is below the maxUsed−1 window, FREEZE the digit (previous desired, else current)
	// but keep isDefault=true / CM updateMode=Automatic and raise the drift metric.
	//
	// NOTE(kubernetesVersion-deprecation): keep — soft-guard survives after the Automatic alias
	// is dropped; the flag/Values key still mean "tracking Deckhouse default" (Default only).
	publishedTarget := target
	if isDefault {
		// Secret maxUsedControlPlaneKubernetesVersion is the canonical baseline (same source
		// admission uses). ConfigMap label max-k8s-version is a fallback when the Secret key
		// is still absent.
		maxUsed := secretMaxUsed
		if maxUsed == "" {
			maxUsed = cmSnap.MaxUsed
		}
		froze := false
		if maxUsed != "" {
			inWindow, err := kubernetesVersionInMaxUsedWindow(target, maxUsed)
			if err == nil && !inWindow {
				// Freeze memory, most authoritative first. The first two live inside the very
				// ConfigMap this hook recreates, so `kubectl delete cm d8-cluster-kubernetes` wipes
				// both at once — while maxUsed survives in the Secret. Without the third source the
				// guard would know the window is violated and still publish the lower Default.
				frozen := cmSnap.DesiredVersion
				if frozen == "" {
					frozen = cmSnap.CurrentVersion
				}
				if frozen == "" {
					frozen = input.Values.Get("global.discovery.targetKubernetesVersion").String()
				}
				if frozen != "" {
					publishedTarget = frozen
					froze = true
				}
				// Two distinct states, one alert: froze=true means the digit is held; froze=false
				// means the window is violated and there is nothing left to hold it at, so the
				// version is about to move down. Do not collapse them into a single signal.
				input.MetricsCollector.Set(
					defaultVersionDriftMetricName,
					1,
					map[string]string{"frozen": strconv.FormatBool(froze)},
					metrics.WithGroup(defaultVersionDriftMetricGroup),
				)
			}
		}
		// No baseline → fail-open: publish Default + track-default mode.

		// TODO(E2E-KV): temporary stand debug logs — remove before final PR (`rg E2E-KV`).
		input.Logger.Info("E2E-KV soft-guard",
			slog.String("secretMaxUsed", secretMaxUsed),
			slog.String("cmLabelMaxUsed", cmSnap.MaxUsed),
			slog.String("maxUsedChosen", maxUsed),
			slog.String("defaultTarget", target),
			slog.String("publishedTarget", publishedTarget),
			slog.Bool("froze", froze),
			slog.String("freezeFromDesired", cmSnap.DesiredVersion),
			slog.String("freezeFromCurrent", cmSnap.CurrentVersion),
		)
	} else {
		// TODO(E2E-KV): temporary stand debug logs — remove before final PR (`rg E2E-KV`).
		input.Logger.Info("E2E-KV soft-guard",
			slog.String("skipped", "manual-pin"),
			slog.String("publishedTarget", publishedTarget),
		)
	}

	input.Values.Set("global.discovery.targetKubernetesVersion", publishedTarget)
	// kubernetesVersionIsDefault means "tracking the Deckhouse default" — MC Default, the
	// deprecated Automatic alias, or nothing pinned anywhere. Named after Default, not the alias:
	// the alias goes away on T+1 (see TODO on automaticKubernetesVersion) and this key is new in
	// this change, so there is no older name to stay compatible with.
	input.Values.Set("global.discovery.kubernetesVersionIsDefault", isDefault)

	return publishDesiredKubernetesVersionSpec(input, publishedTarget, isDefault, cmSnap.SpecYAML)
}

// publishDesiredKubernetesVersionSpec is the single writer of data.spec on
// kube-system/d8-cluster-kubernetes. It never touches status/labels (update-observer owns those).
func publishDesiredKubernetesVersionSpec(input *go_hook.HookInput, desired string, isDefault bool, existingSpecYAML string) error {
	if desired == "" {
		return nil
	}

	updateMode := "Manual"
	if isDefault {
		// CM protocol value: UpdateMode Automatic = "follow Deckhouse default".
		// Unrelated to the deprecated MC enum alias "Automatic"; this string stays after T+1.
		updateMode = "Automatic"
	}

	specBytes, err := yaml.Marshal(configMapSpec{DesiredVersion: desired, UpdateMode: updateMode})
	if err != nil {
		return err
	}
	specYAML := string(specBytes)

	if existingSpecYAML == specYAML {
		// TODO(E2E-KV): temporary stand debug logs — remove before final PR (`rg E2E-KV`).
		input.Logger.Info("E2E-KV publish CM.spec",
			slog.String("desired", desired),
			slog.Bool("isDefault", isDefault),
			slog.Bool("noop", true),
		)
		return nil
	}

	// TODO(E2E-KV): temporary stand debug logs — remove before final PR (`rg E2E-KV`).
	input.Logger.Info("E2E-KV publish CM.spec",
		slog.String("desired", desired),
		slog.Bool("isDefault", isDefault),
		slog.Bool("noop", false),
	)

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
// cluster is tracking the Deckhouse default (MC Default, or deprecated Automatic alias).
//
// The ModuleConfig setting wins whenever it is present, including when it holds Default or
// Automatic — presence of the field, not its value, decides which document owns the version.
// Prefer Default in new ModuleConfigs; Automatic is accepted only as a deprecated alias.
//
// Only when ModuleConfig says nothing at all does the deprecated ClusterConfiguration field apply;
// "Automatic" / empty there is not a pin either and falls through to the Deckhouse default.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — drop CC fallback branch
// (isPinnedKubernetesVersion / ccVersion). After T+1 only MC → Default (no Automatic alias).
func resolveTargetKubernetesVersion(mcVersion, ccVersion, defaultVersion string) (string, bool) {
	switch {
	case isTrackDefaultKubernetesVersion(mcVersion):
		return defaultVersion, true
	case mcVersion != "":
		return mcVersion, false
	case isPinnedKubernetesVersion(ccVersion):
		return ccVersion, false
	default:
		return defaultVersion, true
	}
}

// isTrackDefaultKubernetesVersion reports Default or its deprecated Automatic alias.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — drop Automatic from this helper once the
// MC enum alias and CC field are gone; keep only Default.
func isTrackDefaultKubernetesVersion(version string) bool {
	return version == defaultKubernetesVersionSentinel || version == automaticKubernetesVersion
}

// isPinnedKubernetesVersion reports a concrete minor pin (not empty, not track-default).
//
// TODO(kubernetesVersion-deprecation): T+1 remove — dies together with the ClusterConfiguration field.
func isPinnedKubernetesVersion(version string) bool {
	return version != "" && !isTrackDefaultKubernetesVersion(version)
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
