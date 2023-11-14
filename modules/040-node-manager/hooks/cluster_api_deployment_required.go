/*
Copyright 2023 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: staticInstancesNodeGroupFilter,
		},
		{
			Name:       "machine_deployment",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "MachineDeployment",
			FilterFunc: clusterAPIMachineDeploymentFilter,
		},
		{
			Name:       "machine_set",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "MachineSet",
			FilterFunc: clusterAPIMachineSetFilter,
		},
		{
			Name:       "machine",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
			FilterFunc: clusterAPIMachineFilter,
		},
		{
			Name:       "cluster",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Cluster",
			NameSelector: &types.NameSelector{
				MatchNames: []string{clusterAPIStaticClusterName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterAPINamespace},
				},
			},
			FilterFunc: clusterAPIClusterFilter,
		},
	},
}, handleClusterAPIDeploymentRequired)

func staticInstancesNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return nodeGroupWithStaticInstances{
		Name:               ng.Name,
		HasStaticInstances: ng.Spec.StaticInstances != nil,
		DeletionTimestamp:  obj.GetDeletionTimestamp(),
	}, nil
}

func clusterAPIMachineDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return clusterAPIResourceForNodeGroup{
		NodeGroup: obj.GetLabels()["node-group"],
	}, nil
}

func clusterAPIMachineSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return clusterAPIResourceForNodeGroup{
		NodeGroup: obj.GetLabels()["cluster.x-k8s.io/deployment-name"],
	}, nil
}

func clusterAPIMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return clusterAPIResourceForNodeGroup{
		NodeGroup: obj.GetLabels()["node-group"],
	}, nil
}

func clusterAPIClusterFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return struct{}{}, nil
}

func handleClusterAPIDeploymentRequired(input *go_hook.HookInput) error {
	var hasStaticInstancesField bool

	nodeGroupSnapshots := input.Snapshots["node_group"]
	for _, nodeGroupSnapshot := range nodeGroupSnapshots {
		hasStaticInstancesField = nodeGroupSnapshot.(nodeGroupWithStaticInstances).HasStaticInstances
		if hasStaticInstancesField {
			break // we need at least one NodeGroup with staticInstances field
		}
	}

	//nodeGroupSnapshots := input.Snapshots["node_group"]
	//for _, nodeGroupSnapshot := range nodeGroupSnapshots {
	//	nodeGroup := nodeGroupSnapshot.(nodeGroupWithStaticInstances)
	//
	//	if nodeGroup.HasStaticInstances {
	//		if nodeGroup.DeletionTimestamp != nil && !nodeGroup.DeletionTimestamp.IsZero() {
	//
	//			if clusterAPIHasMachineDeploymentForNodeGroup(nodeGroup.Name, input) {
	//				patch := map[string]interface{}{
	//					"spec": map[string]interface{}{
	//						"replicas": 0,
	//					},
	//				}
	//
	//				input.PatchCollector.MergePatch(patch, "cluster.x-k8s.io/v1beta1", "MachineDeployment", clusterAPINamespace, nodeGroup.Name)
	//			}
	//
	//			//patch := map[string]interface{}{
	//			//	"spec": map[string]interface{}{
	//			//		"staticInstances": map[string]interface{}{
	//			//			"count": 0,
	//			//		},
	//			//	},
	//			//}
	//
	//			//input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "NodeGroup", "", "static-worker")
	//
	//			//if clusterAPIHasResourcesForNodeGroup(nodeGroup.NodeGroup, input) {
	//			//	hasStaticInstancesField = true
	//			//
	//			//	break
	//			//}
	//		}
	//	}
	//}

	if hasStaticInstancesField {
		input.Values.Set("nodeManager.internal.capiControllerManagerEnabled", true)
		input.Values.Set("nodeManager.internal.capsControllerManagerEnabled", true)
		input.Values.Set("nodeManager.internal.capsControllerManagerClusterEnabled", true)
	} else if len(input.Snapshots["machine"]) == 0 {
		input.Values.Remove("nodeManager.internal.capsControllerManagerClusterEnabled")

		if len(input.Snapshots["cluster"]) == 0 {
			input.Values.Remove("nodeManager.internal.capiControllerManagerEnabled")
			input.Values.Remove("nodeManager.internal.capsControllerManagerEnabled")
		}
	}

	return nil
}

type clusterAPIResourceForNodeGroup struct {
	NodeGroup string
}

type nodeGroupWithStaticInstances struct {
	Name               string
	HasStaticInstances bool
	DeletionTimestamp  *metav1.Time
}

func clusterAPIHasResourcesForNodeGroup(nodeGroupName string, input *go_hook.HookInput) bool {
	//snapshots := input.Snapshots["machine_deployment"]
	snapshots := input.Snapshots["machine_set"]
	snapshots = append(snapshots, input.Snapshots["machine"]...)

	for _, snapshot := range snapshots {
		resource := snapshot.(clusterAPIResourceForNodeGroup)

		if resource.NodeGroup == nodeGroupName {
			return true
		}
	}

	return false
}

func clusterAPIHasMachineDeploymentForNodeGroup(nodeGroupName string, input *go_hook.HookInput) bool {
	snapshots := input.Snapshots["machine_deployment"]

	for _, snapshot := range snapshots {
		resource := snapshot.(clusterAPIResourceForNodeGroup)

		if resource.NodeGroup == nodeGroupName {
			return true
		}
	}

	return false
}
