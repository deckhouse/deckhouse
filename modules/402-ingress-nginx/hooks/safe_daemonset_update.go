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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/ingress-nginx/safe_daemonset_update",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "for_delete",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(true),
			NamespaceSelector:            internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"lifecycle.apps.kruise.io/state": "PreparingDelete",
				},
			},
			FilterFunc: applyIngressPodFilter,
		},
		{
			Name:                         "all_pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"controller", "proxy-failover"},
					},
					{
						Key:      "ingress.deckhouse.io/block-deleting",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				},
			},
			FilterFunc: applyIngressPodFilter,
		},
	},
}, safeControllerUpdate)

func safeControllerUpdate(input *go_hook.HookInput) (err error) {
	snap := input.Snapshots["for_delete"]
	if len(snap) == 0 {
		return nil
	}
	podForDelete := snap[0].(ingressControllerPod)

	var failoverReady, proxyReady bool

	for _, sn := range input.Snapshots["all_pods"] {
		pod := sn.(ingressControllerPod)

		if pod.Node != podForDelete.Node {
			continue
		}

		if !(pod.ControllerName == podForDelete.ControllerName || pod.ControllerName == podForDelete.ControllerName+"-failover") {
			continue
		}

		if !pod.IsReady {
			continue
		}

		if strings.HasPrefix(pod.Name, "proxy") {
			proxyReady = true
			continue
		}

		if strings.HasPrefix(pod.Name, "controller-"+podForDelete.ControllerName+"-failover") {
			failoverReady = true
			continue
		}
	}

	if proxyReady && failoverReady {
		metadata := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"ingress.deckhouse.io/block-deleting": nil,
				},
			},
		}
		input.PatchCollector.MergePatch(metadata, "v1", "Pod", internal.Namespace, podForDelete.Name)
	}

	return nil
}

type ingressControllerPod struct {
	Name           string
	Node           string
	ControllerName string
	IsReady        bool
}

func applyIngressPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return ingressControllerPod{
		Name:           pod.Name,
		Node:           pod.Spec.NodeName,
		ControllerName: pod.Labels["name"],
		IsReady:        podIsReady(pod),
	}, nil
}

func podIsReady(pod v1.Pod) bool {
	var conditionReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			conditionReady = true
			break
		}
	}

	if conditionReady && pod.DeletionTimestamp == nil {
		return true
	}

	return false
}
