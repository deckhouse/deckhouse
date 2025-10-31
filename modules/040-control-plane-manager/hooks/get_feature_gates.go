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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/candi/feature_gates"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue + "/feature_gates",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, getFeatureGatesHandler)

type featureGatesResult struct {
	APIServer             []string `json:"apiServer"`
	KubeControllerManager []string `json:"kubeControllerManager"`
	KubeScheduler         []string `json:"kubeScheduler"`
	Kubelet               []string `json:"kubelet"`
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

	for _, fg := range userFeatureGates {
		featureName := fg.String()
		if featureName == "" {
			continue
		}

		components := []string{"apiserver", "kube-controller-manager", "kube-scheduler", "kubelet"}
		for _, component := range components {
			info := feature_gates.GetFeatureGateInfo(normalizedVersion, component, featureName)

			if info.IsForbidden {
				continue
			}

			if info.Exists {
				switch component {
				case "apiserver":
					result.APIServer = append(result.APIServer, featureName)
				case "kube-controller-manager":
					result.KubeControllerManager = append(result.KubeControllerManager, featureName)
				case "kube-scheduler":
					result.KubeScheduler = append(result.KubeScheduler, featureName)
				case "kubelet":
					result.Kubelet = append(result.Kubelet, featureName)
				}
			}
		}
	}

	input.Values.Set("controlPlaneManager.internal.enabledFeatureGates", result)

	return nil
}
