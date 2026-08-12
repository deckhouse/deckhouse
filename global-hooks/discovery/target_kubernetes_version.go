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
// → Deckhouse default and publishes global.discovery.targetKubernetesVersion.
//
// Three different "Kubernetes versions" exist; do not confuse them:
//
//	global.discovery.kubernetesVersion                       actual (kubernetes_version.go)
//	global.discovery.targetKubernetesVersion                 declared goal (this file)
//	controlPlaneManager.internal.effectiveKubernetesVersion  throttled, one minor at a time

package hooks

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Own snapshot of the Secret cluster_configuration.go also watches, so this hook does not
			// inherit that one's failure modes.
			Name:              targetVersionClusterConfigSnapshot,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        applyClusterConfigurationYamlFilter,
		},
		{
			// Must re-run on every event, so do not set ExecuteHookOnEvents: false.
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
				NameSelector: &types.NameSelector{MatchNames: []string{clusterKubernetesConfigMapNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{clusterKubernetesConfigMapName}},
			FilterFunc:   applyClusterKubernetesConfigMapFilter,
		},
	},
}, targetKubernetesVersion)

const (
	targetVersionClusterConfigSnapshot      = "clusterConfigurationForTargetVersion"
	controlPlaneManagerModuleConfigSnapshot = "controlPlaneManagerModuleConfig"
	clusterKubernetesConfigMapSnapshot      = "clusterKubernetesConfigMap"

	clusterKubernetesConfigMapName      = "d8-cluster-kubernetes"
	clusterKubernetesConfigMapNamespace = "kube-system"

	defaultVersionDriftMetricGroup = "D8ControlPlaneDefaultVersionDrift"
	defaultVersionDriftMetricName  = "d8_control_plane_default_version_drift"

	// ClusterConfiguration's "track Deckhouse default" sentinel; ModuleConfig accepts only Default.
	automaticKubernetesVersion       = "Automatic"
	defaultKubernetesVersionSentinel = "Default"
)

// Soft-guard inputs: the maxUsed floor and the two candidates for the frozen digit.
type clusterKubernetesSnapshot struct {
	MaxUsed        string
	CurrentVersion string
	DesiredVersion string
}

type configMapSpec struct {
	DesiredVersion string `json:"desiredVersion"`
	UpdateMode     string `json:"updateMode"`
	MaxUsedVersion string `json:"maxUsedKubernetesVersion"`
}

type moduleConfigKubernetesVersion struct {
	Version   string
	Malformed bool
}

// Never errors: that would discard the snapshot and take down the only publisher of
// targetKubernetesVersion. A non-string is reported as Malformed, not coerced — coercion drops a
// trailing zero, turning 1.40 into the nonexistent "1.4".
func applyControlPlaneManagerKubernetesVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	raw, found, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "spec", "settings", "kubernetesVersion")
	if err != nil || !found || raw == nil {
		return moduleConfigKubernetesVersion{}, nil
	}

	version, isString := raw.(string)
	if !isString {
		return moduleConfigKubernetesVersion{Malformed: true}, nil
	}

	return moduleConfigKubernetesVersion{Version: strings.TrimSpace(version)}, nil
}

func applyClusterKubernetesConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	snap := clusterKubernetesSnapshot{}

	var status struct {
		CurrentVersion string `json:"currentVersion"`
	}
	if err := yaml.Unmarshal([]byte(cm.Data["status"]), &status); err == nil {
		snap.CurrentVersion = status.CurrentVersion
	}

	var spec configMapSpec
	if err := yaml.Unmarshal([]byte(cm.Data["spec"]), &spec); err == nil {
		snap.DesiredVersion = spec.DesiredVersion
		snap.MaxUsed = strings.TrimSpace(spec.MaxUsedVersion)
	}

	return snap, nil
}

// Degrades rather than fails: the health of each document is reported by the hook that owns it.
func readSnapshot[T any](input *go_hook.HookInput, name string) T {
	var zero T

	snaps, err := sdkobjectpatch.UnmarshalToStruct[T](input.Snapshots, name)
	if err != nil {
		input.Logger.Warn("failed to unmarshal snapshot", slog.String("snapshot", name), dlog.Err(err))
		return zero
	}

	if len(snaps) == 0 {
		return zero
	}

	return snaps[0]
}

func targetKubernetesVersion(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(defaultVersionDriftMetricGroup)

	mcSnap := readSnapshot[moduleConfigKubernetesVersion](input, controlPlaneManagerModuleConfigSnapshot)
	cmSnap := readSnapshot[clusterKubernetesSnapshot](input, clusterKubernetesConfigMapSnapshot)
	ccSnap := readSnapshot[ClusterConfigurationYaml](input, targetVersionClusterConfigSnapshot)

	if mcSnap.Malformed {
		input.Logger.Warn(
			"ignoring ModuleConfig control-plane-manager kubernetesVersion: the value is not a string " +
				"(an unquoted version is parsed as a number); falling back to ClusterConfiguration or the Deckhouse default",
		)
	}

	mcVersion := usableDeclaredVersion(input, mcSnap.Version, "ModuleConfig control-plane-manager", isModuleConfigPinned)
	ccRawVersion := usableDeclaredVersion(input, strings.TrimSpace(ccSnap.KubernetesVersion), "ClusterConfiguration", isClusterConfigurationPinned)
	secretMaxUsed := ccSnap.MaxUsed

	target, isDefault := resolveTargetKubernetesVersion(mcVersion, ccRawVersion, hooks.DefaultKubernetesVersion)

	// Soft guard, track-default mode only: when the Deckhouse default falls below the maxUsed−1
	// window, freeze the digit but keep isDefault and raise the drift metric.
	publishedTarget := target
	if isDefault {
		// The ConfigMap key is the same baseline admission uses, so the two cannot disagree.
		maxUsed := cmp.Or(
			usableMaxUsedVersion(input, "cluster ConfigMap spec.maxUsedKubernetesVersion", cmSnap.MaxUsed),
			usableMaxUsedVersion(input, "ClusterConfiguration Secret maxUsedControlPlaneKubernetesVersion", secretMaxUsed),
		)
		froze := false
		if maxUsed != "" {
			// Logged: this is the last branch in which the guard can switch itself off.
			inWindow, err := kubernetesVersionInMaxUsedWindow(target, maxUsed)
			if err != nil {
				input.Logger.Warn(
					"kubernetesVersion soft guard is disabled: cannot compare the target with the maxUsed baseline",
					slog.String("target", target),
					slog.String("maxUsed", maxUsed),
					dlog.Err(err),
				)
			}
			if err == nil && !inWindow {
				// Order is load-bearing: desiredVersion is this hook's own previous output routed back
				// through the ConfigMap, so trusting it first lets a bad target become self-confirming.
				// currentVersion comes from the running Pods and cannot be poisoned that way.
				frozen := cmp.Or(
					cmSnap.CurrentVersion,
					cmSnap.DesiredVersion,
					input.Values.Get("global.discovery.targetKubernetesVersion").String(),
				)
				if frozen != "" {
					publishedTarget = frozen
					froze = true
				}
				// froze=false means nothing was left to hold the digit at, so the version moves down.
				input.MetricsCollector.Set(
					defaultVersionDriftMetricName,
					1,
					map[string]string{"frozen": strconv.FormatBool(froze)},
					metrics.WithGroup(defaultVersionDriftMetricGroup),
				)
			}
		}
		// Fail-open, but logged: a guard switching itself off looks like one finding nothing wrong.
		if maxUsed == "" {
			input.Logger.Warn(
				"kubernetesVersion soft guard is disabled: no maxUsed baseline in the cluster ConfigMap or the ClusterConfiguration Secret",
				slog.String("target", target),
			)
		}

		// The drift metric alone does not say at what version the digit is held.
		if froze {
			input.Logger.Info("holding the Kubernetes version below the Deckhouse default",
				slog.String("deckhouseDefault", target),
				slog.String("published", publishedTarget),
				slog.String("maxUsed", maxUsed),
			)
		}
	}

	input.Values.Set("global.discovery.targetKubernetesVersion", publishedTarget)
	input.Values.Set("global.discovery.kubernetesVersionIsDefault", isDefault)

	return nil
}

// Presence of the ModuleConfig field — not its value — decides which document owns the version, so
// Default there still wins over a ClusterConfiguration pin.
func resolveTargetKubernetesVersion(mcVersion, ccVersion, defaultVersion string) (string, bool) {
	switch {
	case isModuleConfigTrackDefault(mcVersion):
		return defaultVersion, true
	case mcVersion != "":
		return mcVersion, false
	case isClusterConfigurationPinned(ccVersion):
		return ccVersion, false
	default:
		return defaultVersion, true
	}
}

// ModuleConfig takes Default only; ClusterConfiguration keeps the older Automatic.
func isModuleConfigTrackDefault(version string) bool {
	return version == defaultKubernetesVersionSentinel
}

func isModuleConfigPinned(version string) bool {
	return version != "" && !isModuleConfigTrackDefault(version)
}

func isClusterConfigurationPinned(version string) bool {
	return version != "" &&
		version != automaticKubernetesVersion &&
		version != defaultKubernetesVersionSentinel
}

// usableDeclaredVersion drops a declared kubernetesVersion that is neither a sentinel nor a version:
// effective_kubernetes_version.go would feed it to semver.NewVersion and abort on every run.
// Defence in depth for objects predating the closed enums.
func usableDeclaredVersion(input *go_hook.HookInput, version, source string, isPin func(string) bool) string {
	if !isPin(version) {
		return version
	}

	if _, err := semver.NewVersion(version); err != nil {
		input.Logger.Warn(
			"ignoring the declared kubernetesVersion: not a version and not a sentinel this document accepts",
			slog.String("source", source),
			slog.String("value", version),
		)
		return ""
	}

	return version
}

// Filtering before cmp.Or chooses matters: otherwise one unusable higher-priority value shadows a
// good lower-priority one and switches the guard off entirely.
func usableMaxUsedVersion(input *go_hook.HookInput, source, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if _, err := semver.NewVersion(value); err != nil {
		input.Logger.Warn(
			"ignoring an unusable maxUsedKubernetesVersion baseline, falling through to the next source",
			slog.String("source", source),
			slog.String("value", value),
			dlog.Err(err),
		)
		return ""
	}

	return value
}

func kubernetesVersionInMaxUsedWindow(target, maxUsed string) (bool, error) {
	// Trimmed because admission's parseVersion trims too.
	targetV, err := semver.NewVersion(strings.TrimSpace(target))
	if err != nil {
		return false, err
	}
	maxUsedV, err := semver.NewVersion(strings.TrimSpace(maxUsed))
	if err != nil {
		return false, err
	}

	return !hooks.KubernetesVersionBelowFloor(targetV, maxUsedV), nil
}
