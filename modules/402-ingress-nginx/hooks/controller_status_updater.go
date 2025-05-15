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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
	},
}, handleStatusUpdater)

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

func handleStatusUpdater(input *go_hook.HookInput) error {
	daemonSetSnapshots := input.Snapshots["ingress-nginx-daemonset"]
	for _, snap := range daemonSetSnapshots {
		daemonSetInfo := snap.(DaemonSetFilterResult)

		var appliedVersion = "unknown"
		var conditions map[string]any
		now := time.Now().Format(time.RFC3339)
		if daemonSetInfo.NumberReady == daemonSetInfo.DesiredNumber && daemonSetInfo.UpdatedNumber == daemonSetInfo.DesiredNumber {
			conditions = map[string]any{
				"type":           "Ready",
				"status":         "True",
				"lastUpdateTime": now,
				"reason":         "AllPodsReady",
				"message":        "All controller pods are ready",
			}

			appliedVersion = daemonSetInfo.ControllerVersion
			input.Values.Set("ingressNginx.internal.appliedControllerVersion", appliedVersion)
		} else {
			conditions = map[string]any{
				"type":           "Ready",
				"status":         "False",
				"lastUpdateTime": now,
				"reason":         "PodsNotReady",
				"message":        "Controller pods are not ready",
			}

			if val, ok := input.Values.GetOk("ingressNginx.internal.appliedControllerVersion"); ok {
				appliedVersion = val.String()
			}
		}

		statusPatch := map[string]any{
			"status": map[string]any{
				"version":    appliedVersion,
				"conditions": []map[string]any{conditions},
			},
		}

		input.PatchCollector.MergePatch(
			statusPatch,
			"deckhouse.io/v1",
			"IngressNginxController",
			"",
			strings.TrimPrefix(daemonSetInfo.Name, "controller-"),
			object_patch.WithSubresource("/status"),
		)
	}
	return nil
}
