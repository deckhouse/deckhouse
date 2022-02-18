/*
Copyright 2021 Flant JSC

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
	"errors"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/ingress-nginx/manual_daemonset_update",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controllers",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-manual-update": "true",
					"app":                         "controller",
				},
			},
			FilterFunc: filterManualDS,
		},
		{
			Name:                         "pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
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
			FilterFunc: filterManualPod,
		},
	},
}, manualControllerUpdate)

type manualDSController struct {
	Name       string
	Generation int64
}

type manualRolloutPod struct {
	Name       string
	Generation int64

	DSControllerName string

	Ready bool
}

func manualControllerUpdate(input *go_hook.HookInput) error {
	var controllers []manualDSController
	snap := input.Snapshots["controllers"]
	if len(snap) == 0 {
		return nil
	}
	for _, sn := range snap {
		controller := sn.(manualDSController)
		controllers = append(controllers, controller)
	}

	// by ds controller name
	podsMap := make(map[string][]manualRolloutPod)
	snap = input.Snapshots["pods"]
	for _, sn := range snap {
		pod := sn.(manualRolloutPod)
		podsMap[pod.DSControllerName] = append(podsMap[pod.DSControllerName], pod)
	}

	for _, controller := range controllers {
		allPodsReady := true
		var podNameForDeletion string
		for _, pod := range podsMap[controller.Name] {
			if !pod.Ready {
				allPodsReady = false
				break
			}

			if pod.Generation != controller.Generation {
				podNameForDeletion = pod.Name
			}
		}

		if allPodsReady {
			if podNameForDeletion != "" {
				input.PatchCollector.Delete("v1", "Pod", "d8-ingress-nginx", podNameForDeletion)
			}
		}
	}

	return nil
}

func filterManualDS(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return manualDSController{
		Name:       obj.GetName(),
		Generation: obj.GetGeneration(),
	}, nil
}

func filterManualPod(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	genLabel := pod.Labels["pod-template-generation"]
	if len(genLabel) == 0 {
		return nil, errors.New("pod-template-generation label missed")
	}
	gen, err := strconv.ParseInt(genLabel, 10, 64)
	if err != nil {
		return nil, err
	}

	var podReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			podReady = true
			break
		}
	}

	return manualRolloutPod{
		Name:             pod.Name,
		Generation:       gen,
		DSControllerName: pod.Labels["name"],
		Ready:            podReady,
	}, nil
}
