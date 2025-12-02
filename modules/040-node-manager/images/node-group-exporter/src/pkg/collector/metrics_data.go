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

package collector

import (
	ngv1 "node-group-exporter/internal/v1"

	v1 "k8s.io/api/core/v1"
)

const (
	NodeGroupLabelKey = "node.deckhouse.io/group"
)

type NodeMetricsData struct {
	Name      string
	NodeGroup string
	IsReady   float64
}

type NodeGroupMetricsData struct {
	Name      string
	NodeType  string
	HasErrors float64
	Nodes     int32
	Ready     int32
	Max       int32
	Instances int32
	Desired   int32
	Min       int32
	UpToDate  int32
	Standby   int32
}

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

func ToNodeMetricsData(node *v1.Node) NodeMetricsData {
	nodeGroup := extractNodeGroupFromNode(node)
	return NodeMetricsData{
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

func ToNodeGroupMetricsData(nodeGroup *ngv1.NodeGroup) NodeGroupMetricsData {

	return NodeGroupMetricsData{
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
