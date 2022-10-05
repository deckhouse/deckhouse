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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/update_approval",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "nodes",
			ApiVersion:             "v1",
			Kind:                   "Node",
			WaitForSynchronization: pointer.BoolPtr(false),
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: updateApprovalFilterNode,
		},
		{
			Name:                   "control_plane_manager",
			ApiVersion:             "v1",
			Kind:                   "Pod",
			WaitForSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{"d8-control-plane-manager"},
					},
				},
			},
			FilterFunc: updateApprovalFilterPod,
		},
	},
}, handleUpdateApproval)

func updateApprovalFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	var isReady bool
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	var isApproved bool
	_, ok := node.Annotations["control-plane-manger.deckhouse.io/approved"]
	if ok {
		isApproved = true
	}

	var isWaiting bool
	_, ok = node.Annotations["control-plane-manger.deckhouse.io/waiting-for-approval"]
	if ok {
		isWaiting = true
	}

	anode := approvedNode{
		Name:                 node.Name,
		IsApproved:           isApproved,
		IsWaitingForApproval: isWaiting,
		IsReady:              isReady,
		IsUnschedulable:      node.Spec.Unschedulable,
	}

	return anode, nil
}

func updateApprovalFilterPod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}
	var isReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	apod := approvedPod{
		IsReady:  isReady,
		NodeName: pod.Spec.NodeName,
	}

	return apod, nil
}

type approvedNode struct {
	Name       string
	IsApproved bool

	IsWaitingForApproval bool
	IsReady              bool
	IsUnschedulable      bool
}

type approvedPod struct {
	IsReady  bool
	NodeName string
}

func handleUpdateApproval(input *go_hook.HookInput) error {
	nodeMap := make(map[string]approvedNode)
	snap := input.Snapshots["nodes"]
	for _, s := range snap {
		node := s.(approvedNode)
		nodeMap[node.Name] = node
	}

	// Remove approved annotations if pod is ready and node has annotation
	snap = input.Snapshots["control_plane_manager"]
	for _, s := range snap {
		pod := s.(approvedPod)
		if !pod.IsReady {
			continue
		}

		node, ok := nodeMap[pod.NodeName]
		if !ok {
			input.LogEntry.Warnf("Node %s not found", pod.NodeName)
			continue
		}
		if node.IsApproved {
			input.PatchCollector.MergePatch(removeApprovedPatch, "v1", "Node", "", node.Name)
			return nil
		}
	}

	for _, node := range nodeMap {
		//  Skip, if already has approved nodes
		if node.IsApproved {
			return nil
		}
	}

	for _, node := range nodeMap {
		// Approve one node
		if node.IsWaitingForApproval && node.IsReady && !node.IsUnschedulable {
			input.PatchCollector.MergePatch(approvedPatch, "v1", "Node", "", node.Name)
			return nil
		}
	}

	return nil
}

var (
	removeApprovedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"control-plane-manger.deckhouse.io/approved": nil,
			},
		},
	}

	approvedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"control-plane-manger.deckhouse.io/approved":             "",
				"control-plane-manger.deckhouse.io/waiting-for-approval": nil,
			},
		},
	}
)
