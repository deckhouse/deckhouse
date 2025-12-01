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
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/update_approval",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "control_plane_nodes",
			ApiVersion:             "v1",
			Kind:                   "Node",
			WaitForSynchronization: ptr.To(false),
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
			Name:                   "etcd_only_nodes",
			ApiVersion:             "v1",
			Kind:                   "Node",
			WaitForSynchronization: ptr.To(false),
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node-role.deckhouse.io/etcd-only",
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
			WaitForSynchronization: ptr.To(false),
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
		{
			Name:                   "control_plane_manager_etcd_only",
			ApiVersion:             "v1",
			Kind:                   "Pod",
			WaitForSynchronization: ptr.To(false),
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
						Values:   []string{"d8-control-plane-manager-etcd-only"},
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
	_, ok := node.Annotations["control-plane-manager.deckhouse.io/approved"]
	if ok {
		isApproved = true
	}

	var isWaiting bool
	_, ok = node.Annotations["control-plane-manager.deckhouse.io/waiting-for-approval"]
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

func handleUpdateApproval(_ context.Context, input *go_hook.HookInput) error {
	nodeMap := make(map[string]approvedNode)

	snaps := input.Snapshots.Get("control_plane_nodes")
	for node, err := range sdkobjectpatch.SnapshotIter[approvedNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %v", err)
		}

		nodeMap[node.Name] = node
	}

	snaps = input.Snapshots.Get("etcd_only_nodes")
	for node, err := range sdkobjectpatch.SnapshotIter[approvedNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'etcd_only_nodes' snapshots: %v", err)
		}

		nodeMap[node.Name] = node
	}

	// Remove approved annotations if pod is ready and node has annotation
	snaps = input.Snapshots.Get("control_plane_manager")
	for pod, err := range sdkobjectpatch.SnapshotIter[approvedPod](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'control_plane_manager' snapshots: %v", err)
		}

		if !pod.IsReady {
			continue
		}

		node, ok := nodeMap[pod.NodeName]
		if !ok {
			input.Logger.Warn("Node not found", slog.String("name", pod.NodeName))
			continue
		}
		if node.IsApproved {
			input.PatchCollector.PatchWithMerge(removeApprovedPatch, "v1", "Node", "", node.Name)
			return nil
		}
	}

	snaps = input.Snapshots.Get("control_plane_manager_etcd_only")
	for pod, err := range sdkobjectpatch.SnapshotIter[approvedPod](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'control_plane_manager_etcd_only' snapshots: %v", err)
		}

		if !pod.IsReady {
			continue
		}

		node, ok := nodeMap[pod.NodeName]
		if !ok {
			input.Logger.Warn("Node not found", slog.String("name", pod.NodeName))
			continue
		}
		if node.IsApproved {
			input.PatchCollector.PatchWithMerge(removeApprovedPatch, "v1", "Node", "", node.Name)
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
			input.PatchCollector.PatchWithMerge(approvedPatch, "v1", "Node", "", node.Name)
			return nil
		}
	}

	return nil
}

var (
	removeApprovedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"control-plane-manager.deckhouse.io/approved": nil,
			},
		},
	}

	approvedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"control-plane-manager.deckhouse.io/approved":             "",
				"control-plane-manager.deckhouse.io/waiting-for-approval": nil,
			},
		},
	}
)
