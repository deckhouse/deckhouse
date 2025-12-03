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

package kubernetes

import (
	"fmt"

	ngv1 "node-group-exporter/internal/v1"
	"node-group-exporter/pkg/entity"

	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	NodeGroupLabelKey = "node.deckhouse.io/group"
)

func isNodeReady(node *v1.Node) float64 {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return 1.0
		}
	}
	return 0.0
}

// extractNodeGroupFromNode extracts node group name from node labels
func extractNodeGroupFromNode(node *v1.Node) string {
	if nodeGroup, exists := node.Labels[NodeGroupLabelKey]; exists {
		return nodeGroup
	}
	return ""
}

func ToNodeMetricsData(node *v1.Node) entity.NodeData {
	nodeGroup := extractNodeGroupFromNode(node)
	return entity.NodeData{
		Name:      node.Name,
		NodeGroup: nodeGroup,
		IsReady:   isNodeReady(node),
	}
}

func isHasErrors(nodeGroup *ngv1.NodeGroup) float64 {
	for _, condition := range nodeGroup.Status.Conditions {
		if condition.Type == ngv1.NodeGroupConditionTypeError && condition.Status == ngv1.ConditionTrue {
			return 1.0
		}
	}
	return 0.0
}

func ToNodeGroupMetricsData(nodeGroup *ngv1.NodeGroup) entity.NodeGroupData {
	return entity.NodeGroupData{
		Name:      nodeGroup.Name,
		NodeType:  nodeGroup.Spec.NodeType.String(),
		Nodes:     nodeGroup.Status.Nodes,
		Ready:     nodeGroup.Status.Ready,
		Max:       nodeGroup.Status.Max,
		Instances: nodeGroup.Status.Instances,
		Desired:   nodeGroup.Status.Desired,
		Min:       nodeGroup.Status.Min,
		UpToDate:  nodeGroup.Status.UpToDate,
		Standby:   nodeGroup.Status.Standby,
		HasErrors: isHasErrors(nodeGroup),
	}
}

// ConvertToNodeGroup converts a runtime.Object to NodeGroup
func ConvertToNodeGroup(obj any) (*entity.NodeGroupData, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to convert obj to unstructured: %T", obj)
	}
	var ng ngv1.NodeGroup
	err := sdk.FromUnstructured(unstructuredObj, &ng)
	if err != nil {
		return nil, err
	}

	nodeGroupData := ToNodeGroupMetricsData(&ng)
	return &nodeGroupData, nil
}

// ConvertToNode converts a runtime.Object to Node
func ConvertToNode(obj any) (*entity.NodeData, error) {
	nodeObj, ok := obj.(*v1.Node)
	if !ok {
		return nil, fmt.Errorf("failed to convert obj to v1.Node: %T", obj)
	}
	nodeData := ToNodeMetricsData(nodeObj)
	return &nodeData, nil
}
