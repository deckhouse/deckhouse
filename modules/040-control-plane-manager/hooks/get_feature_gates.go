/*
Copyright 2025 Flant JSC

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
	"context"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, getFeatureGatesHandler)

type featureGatesResult struct {
	APIServer             []string `json:"apiserver"`
	KubeControllerManager []string `json:"kubeControllerManager"`
	KubeScheduler         []string `json:"kubeScheduler"`
	Kubelet               []string `json:"kubelet"`
}

type KubernetesVersion string

func (v KubernetesVersion) Normalize() KubernetesVersion {
	parts := strings.Split(string(v), ".")
	if len(parts) >= 2 {
		return KubernetesVersion(parts[0] + "." + parts[1])
	}
	return v
}

func (v KubernetesVersion) IsGreaterThan(other KubernetesVersion) bool {
	parts1 := strings.Split(string(v), ".")
	parts2 := strings.Split(string(other), ".")

	if len(parts1) < 2 || len(parts2) < 2 {
		return false
	}

	major1, err := strconv.Atoi(parts1[0])
	if err != nil {
		return false
	}
	major2, err := strconv.Atoi(parts2[0])
	if err != nil {
		return false
	}

	if major1 != major2 {
		return major1 > major2
	}

	minor1, err := strconv.Atoi(parts1[1])
	if err != nil {
		return false
	}
	minor2, err := strconv.Atoi(parts2[1])
	if err != nil {
		return false
	}

	return minor1 > minor2
}

func isFeatureGateDeprecatedInFutureVersions(currentVersion KubernetesVersion, featureName string) (bool, KubernetesVersion) {
	for version, features := range FeatureGatesMap {
		v := KubernetesVersion(version)
		if v.IsGreaterThan(currentVersion) {
			if features.IsDeprecated(featureName) {
				return true, v
			}
		}
	}
	return false, ""
}

func getFeatureGatesHandler(_ context.Context, input *go_hook.HookInput) error {
	k8sVersionStr := input.Values.Get("global.clusterConfiguration.kubernetesVersion").String()

	result := featureGatesResult{
		APIServer:             []string{},
		KubeControllerManager: []string{},
		KubeScheduler:         []string{},
		Kubelet:               []string{},
	}

	if k8sVersionStr == "" {
		input.Values.Set("controlPlaneManager.internal.allowedFeatureGates", result)
		return nil
	}

	currentVersion := KubernetesVersion(k8sVersionStr).Normalize()

	userFeatureGates := input.Values.Get("controlPlaneManager.enabledFeatureGates").Array()

	deprecatedFeatureGates := make(map[string]KubernetesVersion)
	currentlyDeprecatedFeatureGates := make(map[string]bool)
	currentlyForbiddenFeatureGates := make(map[string]bool)
	unknownFeatureGates := make(map[string]bool)

	currentFeatures, ok := FeatureGatesMap[string(currentVersion)]
	if !ok {
		input.Values.Set("controlPlaneManager.internal.allowedFeatureGates", result)
		return nil
	}

	for _, fg := range userFeatureGates {
		featureName := fg.String()
		if featureName == "" {
			continue
		}

		if currentFeatures.IsForbidden(featureName) {
			currentlyForbiddenFeatureGates[featureName] = true
			continue
		}

		if currentFeatures.IsDeprecated(featureName) {
			currentlyDeprecatedFeatureGates[featureName] = true
			continue
		}

		components := []string{"apiserver", "kubeControllerManager", "kubeScheduler", "kubelet"}
		featureExistsInAnyComponent := false
		for _, component := range components {
			info := currentFeatures.GetFeatureGateInfo(component, featureName)

			if info.Exists {
				featureExistsInAnyComponent = true
				switch component {
				case "apiserver":
					result.APIServer = append(result.APIServer, featureName)
				case "kubeControllerManager":
					result.KubeControllerManager = append(result.KubeControllerManager, featureName)
				case "kubeScheduler":
					result.KubeScheduler = append(result.KubeScheduler, featureName)
				case "kubelet":
					result.Kubelet = append(result.Kubelet, featureName)
				}
			}
		}

		if !featureExistsInAnyComponent {
			unknownFeatureGates[featureName] = true
			continue
		}

		isDeprecatedInFuture, deprecatedVersion := isFeatureGateDeprecatedInFutureVersions(currentVersion, featureName)
		if isDeprecatedInFuture {
			deprecatedFeatureGates[featureName] = deprecatedVersion
		}
	}

	input.Values.Set("controlPlaneManager.internal.allowedFeatureGates", result)

	for featureName := range currentlyDeprecatedFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_problematic_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": string(currentVersion),
				"current_version":    string(currentVersion),
				"status":             "deprecated",
			},
		)
	}

	for featureName, deprecatedVersion := range deprecatedFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_problematic_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": string(deprecatedVersion),
				"current_version":    string(currentVersion),
				"status":             "will_be_deprecated",
			},
		)
	}

	for featureName := range currentlyForbiddenFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_problematic_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": "",
				"current_version":    string(currentVersion),
				"status":             "forbidden",
			},
		)
	}

	for featureName := range unknownFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_problematic_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": "",
				"current_version":    string(currentVersion),
				"status":             "unknown",
			},
		)
	}

	if len(deprecatedFeatureGates) == 0 && len(currentlyDeprecatedFeatureGates) == 0 && len(currentlyForbiddenFeatureGates) == 0 && len(unknownFeatureGates) == 0 {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_problematic_feature_gate",
			0.0,
			map[string]string{
				"feature_gate":       "",
				"deprecated_version": "",
				"current_version":    string(currentVersion),
				"status":             "",
			},
		)
	}

	return nil
}
