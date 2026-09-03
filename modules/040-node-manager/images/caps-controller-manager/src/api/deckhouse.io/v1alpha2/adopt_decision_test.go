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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAdoptionDecisionForNode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		breakNode func(*corev1.Node)
		nodeGroup string
		action    AdoptAction
		reason    string
	}{
		"a settled node of the right node group is adopted": {
			nodeGroup: testNodeGroup,
			action:    AdoptActionAdopt,
			reason:    AdoptReasonNodeIsAdoptable,
		},
		"a node that is being deleted is waited out": {
			breakNode: func(node *corev1.Node) {
				node.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
			},
			nodeGroup: testNodeGroup,
			action:    AdoptActionWait,
			reason:    AdoptReasonNodeIsBeingDeleted,
		},
		"a not ready node is waited out": {
			breakNode: func(node *corev1.Node) {
				node.Status.Conditions[0].Status = corev1.ConditionFalse
			},
			nodeGroup: testNodeGroup,
			action:    AdoptActionWait,
			reason:    AdoptReasonNodeIsNotReady,
		},
		"a node without a readiness condition is waited out": {
			breakNode: func(node *corev1.Node) { node.Status.Conditions = nil },
			nodeGroup: testNodeGroup,
			action:    AdoptActionWait,
			reason:    AdoptReasonNodeIsNotReady,
		},
		"a node without a providerID is rejected": {
			breakNode: func(node *corev1.Node) { node.Spec.ProviderID = "" },
			nodeGroup: testNodeGroup,
			action:    AdoptActionReject,
			reason:    AdoptReasonNodeHasNoProviderID,
		},
		"a node of another node group is rejected": {
			breakNode: func(node *corev1.Node) {
				node.Labels[NodeGroupLabel] = "another-group"
			},
			nodeGroup: testNodeGroup,
			action:    AdoptActionReject,
			reason:    AdoptReasonNodeInOtherNodeGroup,
		},
		"a node without a node group label is rejected": {
			breakNode: func(node *corev1.Node) { node.Labels = nil },
			nodeGroup: testNodeGroup,
			action:    AdoptActionReject,
			reason:    AdoptReasonNodeInOtherNodeGroup,
		},
		"an unknown desired node group does not reject the node": {
			nodeGroup: "",
			action:    AdoptActionAdopt,
			reason:    AdoptReasonNodeIsAdoptable,
		},
		"deletion is reported before unreadiness": {
			breakNode: func(node *corev1.Node) {
				node.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
				node.Status.Conditions[0].Status = corev1.ConditionFalse
			},
			nodeGroup: testNodeGroup,
			action:    AdoptActionWait,
			reason:    AdoptReasonNodeIsBeingDeleted,
		},
		"a node that is still registering is waited out, not rejected": {
			breakNode: func(node *corev1.Node) {
				node.Status.Conditions[0].Status = corev1.ConditionFalse
				node.Spec.ProviderID = ""
			},
			nodeGroup: testNodeGroup,
			action:    AdoptActionWait,
			reason:    AdoptReasonNodeIsNotReady,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			node := adoptableNode("existing")
			if tt.breakNode != nil {
				tt.breakNode(node)
			}

			decision := adoptionDecisionForNode(node, tt.nodeGroup)

			require.Equal(t, tt.action, decision.Action)
			require.Equal(t, tt.reason, decision.Reason)
			require.Equal(t, "existing", decision.NodeName)
			require.NotEmpty(t, decision.Message)
		})
	}
}
