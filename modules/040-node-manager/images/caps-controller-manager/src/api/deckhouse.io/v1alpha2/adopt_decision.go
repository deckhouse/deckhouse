/*
Copyright 2026 Flant JSC

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

package v1alpha2

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// NodeGroupLabel is the label that binds a Node to its NodeGroup.
const NodeGroupLabel = "node.deckhouse.io/group"

// AdoptAction tells the StaticMachine controller what to do with a StaticInstance whose
// bootstrap phase may have to be skipped.
type AdoptAction string

const (
	// AdoptActionBootstrap means the instance has to be bootstrapped as usual.
	AdoptActionBootstrap AdoptAction = "Bootstrap"

	// AdoptActionAdopt means the Node behind spec.address has to be taken under management
	// as-is, without bootstrapping it.
	AdoptActionAdopt AdoptAction = "Adopt"

	// AdoptActionWait means the Node behind spec.address is in a transient state and the
	// decision has to be made again later.
	AdoptActionWait AdoptAction = "Wait"

	// AdoptActionReject means the Node behind spec.address does not match the StaticInstance
	// and cannot be adopted until an operator resolves the mismatch.
	AdoptActionReject AdoptAction = "Reject"
)

// Reasons behind an AdoptDecision. They are reported as condition and event reasons, so they
// have to stay valid Kubernetes reason strings.
const (
	AdoptReasonNotRequested          = "AdoptionNotRequested"
	AdoptReasonRequestedImperatively = "AdoptionRequestedImperatively"
	AdoptReasonNoNodeWithAddress     = "NoNodeWithSameAddress"
	AdoptReasonNodeIsAdoptable       = "NodeIsAdoptable"
	AdoptReasonNodeIsBeingDeleted    = "NodeIsBeingDeleted"
	AdoptReasonNodeIsNotReady        = "NodeIsNotReady"
	AdoptReasonNodeHasNoProviderID   = "NodeHasNoProviderID"
	AdoptReasonNodeInOtherNodeGroup  = "NodeIsInAnotherNodeGroup"
)

// AdoptDecision is the outcome of StaticInstance.ResolveAdoption.
type AdoptDecision struct {
	Action   AdoptAction
	NodeName string
	Reason   string
	Message  string
}

// adoptionDecisionForNode decides whether an existing Node may be taken under management
// without bootstrapping it.
//
// CAPS does not own the Node object, so at any moment the Node may disagree with the rest of
// the cluster: cleanup may have finished while the Node is still registered, or the Node may
// be registered but not configured yet. Transient disagreements are waited out; a settled
// Node that does not match the StaticInstance is neither adopted nor bootstrapped over,
// because both would damage a node that is already part of the cluster.
func adoptionDecisionForNode(node *corev1.Node, nodeGroup string) AdoptDecision {
	switch {
	case node.DeletionTimestamp != nil:
		return waitForNode(node, AdoptReasonNodeIsBeingDeleted,
			"is being deleted; waiting for it to leave the cluster")
	case !nodeIsReady(node):
		return waitForNode(node, AdoptReasonNodeIsNotReady,
			"is not ready; waiting for it to settle")
	case node.Spec.ProviderID == "":
		return rejectNode(node, AdoptReasonNodeHasNoProviderID,
			"has an empty spec.providerID, so it is not a fully registered static node")
	case !nodeBelongsToNodeGroup(node, nodeGroup):
		return rejectNode(node, AdoptReasonNodeInOtherNodeGroup,
			fmt.Sprintf("belongs to node group %q instead of %q",
				node.Labels[NodeGroupLabel], nodeGroup))
	default:
		return AdoptDecision{
			Action:   AdoptActionAdopt,
			NodeName: node.Name,
			Reason:   AdoptReasonNodeIsAdoptable,
			Message:  fmt.Sprintf("Node %q already exists and can be adopted", node.Name),
		}
	}
}

func waitForNode(node *corev1.Node, reason, detail string) AdoptDecision {
	return AdoptDecision{
		Action:   AdoptActionWait,
		NodeName: node.Name,
		Reason:   reason,
		Message:  fmt.Sprintf("Node %q %s", node.Name, detail),
	}
}

func rejectNode(node *corev1.Node, reason, detail string) AdoptDecision {
	return AdoptDecision{
		Action:   AdoptActionReject,
		NodeName: node.Name,
		Reason:   reason,
		Message:  fmt.Sprintf("Node %q %s; adoption requires manual intervention", node.Name, detail),
	}
}

func nodeIsReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// nodeBelongsToNodeGroup reports whether the Node is a member of the given node group. An
// unknown desired node group is not reported as a mismatch: the check is about the Node, and
// there is nothing to compare it against.
func nodeBelongsToNodeGroup(node *corev1.Node, nodeGroup string) bool {
	if nodeGroup == "" {
		return true
	}

	return node.Labels[NodeGroupLabel] == nodeGroup
}
