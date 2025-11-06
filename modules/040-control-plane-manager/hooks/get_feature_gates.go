/*
Copyright 2024 Flant JSC

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

// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	if len(parts1) < 2 || len(parts2) < 2 {
		return 0
	}

	major1, err := strconv.Atoi(parts1[0])
	if err != nil {
		return 0
	}
	major2, err := strconv.Atoi(parts2[0])
	if err != nil {
		return 0
	}

	if major1 != major2 {
		if major1 < major2 {
			return -1
		}
		return 1
	}

	minor1, err := strconv.Atoi(parts1[1])
	if err != nil {
		return 0
	}
	minor2, err := strconv.Atoi(parts2[1])
	if err != nil {
		return 0
	}

	if minor1 < minor2 {
		return -1
	}
	if minor1 > minor2 {
		return 1
	}

	return 0
}

func isFeatureGateDeprecatedInFutureVersions(currentVersion, featureName string) (bool, string) {
	for version := range FeatureGatesMap {
		if compareVersions(version, currentVersion) > 0 {
			// passing empty string as component to check only IsDeprecated
			info := GetFeatureGateInfo(version, "", featureName)
			if info.IsDeprecated {
				return true, version
			}
		}
	}
	return false, ""
}

func getFeatureGatesHandler(_ context.Context, input *go_hook.HookInput) error {
	k8sVersion := input.Values.Get("global.discovery.kubernetesVersion").String()

	result := featureGatesResult{
		APIServer:             []string{},
		KubeControllerManager: []string{},
		KubeScheduler:         []string{},
		Kubelet:               []string{},
	}

	if k8sVersion == "" {
		input.Logger.Warn("Kubernetes version not found, skipping feature gates validation")
		input.Values.Set("controlPlaneManager.internal.enabledFeatureGates", result)
		return nil
	}

	normalizedVersion := k8sVersion
	parts := strings.Split(k8sVersion, ".")
	if len(parts) >= 2 {
		normalizedVersion = parts[0] + "." + parts[1]
	}

	userFeatureGates := input.Values.Get("controlPlaneManager.enabledFeatureGates").Array()

	// featureName -> version
	deprecatedFeatureGates := make(map[string]string)
	currentlyDeprecatedFeatureGates := make(map[string]bool)

	for _, fg := range userFeatureGates {
		featureName := fg.String()
		if featureName == "" {
			continue
		}

		currentInfo := GetFeatureGateInfo(normalizedVersion, "", featureName)
		if currentInfo.IsDeprecated {
			currentlyDeprecatedFeatureGates[featureName] = true
		}

		isDeprecated, deprecatedVersion := isFeatureGateDeprecatedInFutureVersions(normalizedVersion, featureName)
		if isDeprecated {
			deprecatedFeatureGates[featureName] = deprecatedVersion
		}

		components := []string{"apiserver", "kubeControllerManager", "kubeScheduler", "kubelet"}
		for _, component := range components {
			info := GetFeatureGateInfo(normalizedVersion, component, featureName)

			if info.IsForbidden {
				continue
			}

			if info.Exists {
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
	}

	input.Values.Set("controlPlaneManager.internal.enabledFeatureGates", result)

	// Metric for feature gates that are already deprecated in current version
	for featureName := range currentlyDeprecatedFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_deprecated_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": normalizedVersion,
				"current_version":    normalizedVersion,
				"status":             "deprecated",
			},
		)
	}

	// Metric for feature gates that will be deprecated in future versions
	for featureName, deprecatedVersion := range deprecatedFeatureGates {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_deprecated_feature_gate",
			1.0,
			map[string]string{
				"feature_gate":       featureName,
				"deprecated_version": deprecatedVersion,
				"current_version":    normalizedVersion,
				"status":             "will_be_deprecated",
			},
		)
	}

	if len(deprecatedFeatureGates) == 0 && len(currentlyDeprecatedFeatureGates) == 0 {
		input.MetricsCollector.Set(
			"d8_control_plane_manager_deprecated_feature_gate",
			0.0,
			map[string]string{
				"feature_gate":       "",
				"deprecated_version": "",
				"current_version":    normalizedVersion,
				"status":             "",
			},
		)
	}

	return nil
}
