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

package scope

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const (
	// NodeGroupLabel is the label indicating node's group membership
	NodeGroupLabel = "node.deckhouse.io/group"
)

// NodeGroupScope defines a scope for NodeGroup operations.
type NodeGroupScope struct {
	*Scope

	NodeGroup *v1.NodeGroup
	Nodes     []corev1.Node
}

// NewNodeGroupScope creates a new NodeGroup scope.
func NewNodeGroupScope(
	scope *Scope,
	nodeGroup *v1.NodeGroup,
	ctx context.Context,
) (*NodeGroupScope, error) {
	if scope == nil {
		return nil, errors.New("Scope is required when creating a NodeGroupScope")
	}
	if nodeGroup == nil {
		return nil, errors.New("NodeGroup is required when creating a NodeGroupScope")
	}

	patchHelper, err := patch.NewHelper(nodeGroup, scope.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	scope.PatchHelper = patchHelper
	scope.Logger = scope.Logger.WithValues("nodeGroup", nodeGroup.Name, "nodeType", nodeGroup.Spec.NodeType)

	ngs := &NodeGroupScope{
		Scope:     scope,
		NodeGroup: nodeGroup,
	}

	return ngs, nil
}

// LoadNodes loads all nodes belonging to this NodeGroup.
func (s *NodeGroupScope) LoadNodes(ctx context.Context) error {
	nodeList := &corev1.NodeList{}
	err := s.Client.List(ctx, nodeList, client.MatchingLabels{
		NodeGroupLabel: s.NodeGroup.Name,
	})
	if err != nil {
		return errors.Wrap(err, "failed to list nodes")
	}

	s.Nodes = nodeList.Items
	s.Logger.V(1).Info("loaded nodes", "count", len(s.Nodes))

	return nil
}

// GetReadyNodes returns the count of ready nodes.
func (s *NodeGroupScope) GetReadyNodes() int32 {
	var ready int32
	for _, node := range s.Nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				ready++
				break
			}
		}
	}
	return ready
}

// UpdateStatus updates the NodeGroup status based on nodes.
func (s *NodeGroupScope) UpdateStatus() {
	s.NodeGroup.Status.Nodes = int32(len(s.Nodes))
	s.NodeGroup.Status.Ready = s.GetReadyNodes()

	// Calculate upToDate nodes (nodes that match the current config)
	var upToDate int32
	for _, node := range s.Nodes {
		if s.isNodeUpToDate(&node) {
			upToDate++
		}
	}
	s.NodeGroup.Status.UpToDate = upToDate

	// Calculate min/max/desired for cloud instances
	if s.NodeGroup.Spec.CloudInstances != nil {
		ci := s.NodeGroup.Spec.CloudInstances
		zones := len(ci.Zones)
		if zones == 0 {
			zones = 1 // default to 1 zone if not specified
		}
		s.NodeGroup.Status.Min = ci.MinPerZone * int32(zones)
		s.NodeGroup.Status.Max = ci.MaxPerZone * int32(zones)
		s.NodeGroup.Status.Desired = s.NodeGroup.Status.Min // simplified, real logic is more complex
	}

	// Set error if there's a mismatch
	if s.NodeGroup.Status.Ready < s.NodeGroup.Status.Nodes {
		notReady := s.NodeGroup.Status.Nodes - s.NodeGroup.Status.Ready
		s.NodeGroup.Status.Error = ""
		s.Logger.V(1).Info("some nodes are not ready", "notReady", notReady)
	} else {
		s.NodeGroup.Status.Error = ""
	}

	// Update condition summary
	if s.NodeGroup.Status.Ready == s.NodeGroup.Status.Nodes && s.NodeGroup.Status.Nodes > 0 {
		s.NodeGroup.Status.ConditionSummary = &v1.ConditionSummary{
			Ready:         "True",
			StatusMessage: "All nodes are ready",
		}
	} else if s.NodeGroup.Status.Nodes == 0 {
		s.NodeGroup.Status.ConditionSummary = &v1.ConditionSummary{
			Ready:         "False",
			StatusMessage: "No nodes in group",
		}
	} else {
		s.NodeGroup.Status.ConditionSummary = &v1.ConditionSummary{
			Ready:         "False",
			StatusMessage: "Some nodes are not ready",
		}
	}
}

// isNodeUpToDate checks if node matches the current NodeGroup config.
func (s *NodeGroupScope) isNodeUpToDate(node *corev1.Node) bool {
	if s.NodeGroup.Spec.NodeTemplate == nil {
		return true
	}

	// Check labels
	for key, value := range s.NodeGroup.Spec.NodeTemplate.Labels {
		if node.Labels[key] != value {
			return false
		}
	}

	// Check annotations
	for key, value := range s.NodeGroup.Spec.NodeTemplate.Annotations {
		if node.Annotations[key] != value {
			return false
		}
	}

	// TODO: Check taints (more complex comparison needed)

	return true
}

// Patch updates the NodeGroup resource.
func (s *NodeGroupScope) Patch(ctx context.Context) error {
	err := s.PatchHelper.Patch(ctx, s.NodeGroup)
	if err != nil {
		return errors.Wrap(err, "failed to patch NodeGroup")
	}
	return nil
}

// SetCondition sets a condition on the NodeGroup.
func (s *NodeGroupScope) SetCondition(conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()

	// Find existing condition
	for i, c := range s.NodeGroup.Status.Conditions {
		if c.Type == conditionType {
			if c.Status != status || c.Reason != reason || c.Message != message {
				s.NodeGroup.Status.Conditions[i] = metav1.Condition{
					Type:               conditionType,
					Status:             status,
					LastTransitionTime: now,
					Reason:             reason,
					Message:            message,
				}
			}
			return
		}
	}

	// Add new condition
	s.NodeGroup.Status.Conditions = append(s.NodeGroup.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// Close closes the scope and patches the NodeGroup.
func (s *NodeGroupScope) Close(ctx context.Context) error {
	return s.Patch(ctx)
}
