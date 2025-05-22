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
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DaemonSet struct {
	Metadata struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Status struct {
		NumberReady            int64 `json:"numberReady"`
		DesiredNumberScheduled int64 `json:"desiredNumberScheduled"`
		UpdatedNumberScheduled int64 `json:"updatedNumberScheduled"`
	} `json:"status"`
}

type DaemonSetFilterResult struct {
	ControllerVersion string
	Name              string
	NumberReady       int64
	DesiredNumber     int64
	UpdatedNumber     int64
}

type IngressNginxController struct {
	Metadata struct {
		Generation int64  `json:"generation"`
		Name       string `json:"name"`
	} `json:"metadata"`
	Status struct {
		ObservedGeneration int64 `json:"observedGeneration"`
	} `json:"status"`
}

type IngressNginxControllerFilterResult struct {
	Name               string
	Generation         int64
	ObservedGeneration int64
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress-nginx-daemonset",
			ApiVersion: "apps.kruise.io/v1alpha1",
			Kind:       "DaemonSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc: filterIngressNginxDaemonset,
		},
		{
			Name:       "ingress-nginx-controller",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "IngressNginxController",
			FilterFunc: filterIngressNginxController,
		},
	},
}, handleStatusUpdater)

func filterIngressNginxDaemonset(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds DaemonSet
	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	controllerVersion := "unknown"
	if version, exists := ds.Metadata.Annotations["ingress-nginx-controller.deckhouse.io/controller-version"]; exists {
		controllerVersion = version
	}

	return DaemonSetFilterResult{
		ControllerVersion: controllerVersion,
		Name:              ds.Metadata.Name,
		NumberReady:       ds.Status.NumberReady,
		DesiredNumber:     ds.Status.DesiredNumberScheduled,
		UpdatedNumber:     ds.Status.UpdatedNumberScheduled,
	}, nil
}

func filterIngressNginxController(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var controller IngressNginxController
	err := sdk.FromUnstructured(obj, &controller)
	if err != nil {
		return nil, err
	}

	return IngressNginxControllerFilterResult{
		Name:               controller.Metadata.Name,
		Generation:         controller.Metadata.Generation,
		ObservedGeneration: controller.Status.ObservedGeneration,
	}, nil
}

func handleStatusUpdater(input *go_hook.HookInput) error {
	// Save IngressNginxController data to cache
	if !input.Values.Exists("ingressNginx.internal.appliedControllerVersion") {
		input.Values.Set("ingressNginx.internal.appliedControllerVersion", map[string]any{})
	}
	if !input.Values.Exists("ingressNginx.internal.controllerState") {
		input.Values.Set("ingressNginx.internal.controllerState", map[string]any{})
	}

	controllerSnapshots := input.Snapshots["ingress-nginx-controller"]
	for _, snap := range controllerSnapshots {
		controllerInfo := snap.(IngressNginxControllerFilterResult)
		controllerState := map[string]any{
			"generation":         controllerInfo.Generation,
			"observedGeneration": controllerInfo.ObservedGeneration,
		}
		keyValueForState := fmt.Sprintf("ingressNginx.internal.controllerState.%s", controllerInfo.Name)
		input.Values.Set(keyValueForState, controllerState)
	}

	// Handling DaemonSet state changes
	daemonSetSnapshots := input.Snapshots["ingress-nginx-daemonset"]
	for _, snap := range daemonSetSnapshots {
		daemonSetInfo := snap.(DaemonSetFilterResult)
		controllerName := strings.TrimPrefix(daemonSetInfo.Name, "controller-")
		keyValueForVersion := fmt.Sprintf("ingressNginx.internal.appliedControllerVersion.%s", controllerName)
		keyValueForState := fmt.Sprintf("ingressNginx.internal.controllerState.%s", controllerName)

		var generationState int64
		var observedGenerationState int64
		if val, ok := input.Values.GetOk(keyValueForState); ok {
			if item, ok := val.Map()["generation"]; ok {
				generationState = item.Int()
			}
			if item, ok := val.Map()["observedGeneration"]; ok {
				observedGenerationState = item.Int()
			}
		}

		var appliedVersion = "unknown"
		var conditions map[string]any
		var observedGeneration int64

		isReady := daemonSetInfo.DesiredNumber > 0 && daemonSetInfo.NumberReady == daemonSetInfo.DesiredNumber
		isUpdated := daemonSetInfo.DesiredNumber > 0 && daemonSetInfo.UpdatedNumber == daemonSetInfo.DesiredNumber
		if isReady && isUpdated {
			conditions = map[string]any{
				"type":           "Ready",
				"status":         "True",
				"lastUpdateTime": time.Now().Format(time.RFC3339),
				"reason":         "AllPodsReady",
				"message":        "All controller pods are ready",
			}
			observedGeneration = generationState

			appliedVersion = daemonSetInfo.ControllerVersion
			input.Values.Set(keyValueForVersion, appliedVersion)
		} else {
			conditions = map[string]any{
				"type":           "Ready",
				"status":         "False",
				"lastUpdateTime": time.Now().Format(time.RFC3339),
				"reason":         "PodsNotReady",
				"message":        "Controller pods are not ready",
			}
			observedGeneration = observedGenerationState

			if val, ok := input.Values.GetOk(keyValueForVersion); ok {
				appliedVersion = val.String()
			}
		}

		statusPatch := map[string]any{
			"status": map[string]any{
				"version":            appliedVersion,
				"observedGeneration": observedGeneration,
				"conditions":         []map[string]any{conditions},
			},
		}

		input.PatchCollector.PatchWithMerge(
			statusPatch,
			"deckhouse.io/v1",
			"IngressNginxController",
			"",
			controllerName,
			object_patch.WithSubresource("/status"),
		)
	}
	return nil
}
