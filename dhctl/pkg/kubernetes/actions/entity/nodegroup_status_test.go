// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
)

// TestNodeGroupStatusMessages: a master replica that the cloud refuses to create leaves the wait
// counting "Nodes Ready 1 of 3" for half an hour. The refusal is in the NodeGroup the whole time.
func TestNodeGroupStatusMessages(t *testing.T) {
	tests := []struct {
		name   string
		status v1.NodeGroupStatus
		want   []string
	}{
		{
			name: "the provider refused to create the machine",
			status: v1.NodeGroupStatus{
				LastMachineFailures: []v1.MachineFailure{{
					Name: "master-2",
					LastOperation: v1.MachineOperation{
						Description: "Cloud provider message - quota 'CORES' exceeded",
						State:       "Failed",
					},
				}},
			},
			want: []string{"master-2: Cloud provider message - quota 'CORES' exceeded"},
		},
		{
			name: "the group's own error comes first",
			status: v1.NodeGroupStatus{
				Error: "InstanceClass not found",
				LastMachineFailures: []v1.MachineFailure{{
					Name:          "master-1",
					LastOperation: v1.MachineOperation{Description: "no capacity in zone ru-central1-a"},
				}},
			},
			want: []string{
				"InstanceClass not found",
				"master-1: no capacity in zone ru-central1-a",
			},
		},
		{
			// The summary is bland next to a real error, so it only speaks when nothing
			// more specific did.
			name: "the summary fills in when nothing else does",
			status: v1.NodeGroupStatus{
				ConditionSummary: v1.ConditionSummary{Ready: "False", StatusMessage: "Machines are not ready"},
			},
			want: []string{"Machines are not ready"},
		},
		{
			name: "a healthy group says nothing",
			status: v1.NodeGroupStatus{
				Ready:            3,
				ConditionSummary: v1.ConditionSummary{Ready: "True", StatusMessage: ""},
			},
			want: []string{},
		},
		{
			// An empty description carries no information; printing "machine: " is worse
			// than printing nothing.
			name: "a failure with no description is dropped",
			status: v1.NodeGroupStatus{
				LastMachineFailures: []v1.MachineFailure{{Name: "master-2"}},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nodeGroupStatusMessages(tt.status))
		})
	}
}

func TestDescribeTroubles(t *testing.T) {
	t.Run("nothing to report renders nothing", func(t *testing.T) {
		assert.Empty(t, describeTroubles("NodeGroups report:", nil))
	})

	t.Run("each message is its own line", func(t *testing.T) {
		got := describeTroubles("NodeGroups report:", []nodeGroupTrouble{
			{name: "master", messages: []string{"quota exceeded", "master-3: no capacity"}},
		})

		assert.Equal(t, "NodeGroups report:\n* master | quota exceeded\n* master | master-3: no capacity", got)
	})
}

// TestNodeNotReadyReason: "NotReady" repeated for half an hour, while kubelet's own explanation
// sits in the condition.
func TestNodeNotReadyReason(t *testing.T) {
	tests := []struct {
		name string
		node corev1.Node
		want string
	}{
		{
			name: "kubelet explains itself",
			node: node(corev1.NodeCondition{
				Type:    corev1.NodeReady,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeletNotReady",
				Message: "container runtime network not ready: cni config uninitialized",
			}),
			want: "container runtime network not ready: cni config uninitialized",
		},
		{
			name: "only a reason is available",
			node: node(corev1.NodeCondition{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionUnknown,
				Reason: "NodeStatusUnknown",
			}),
			want: "NodeStatusUnknown",
		},
		{
			name: "no Ready condition at all",
			node: node(corev1.NodeCondition{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse}),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nodeNotReadyReason(tt.node))
		})
	}
}

func node(conditions ...corev1.NodeCondition) corev1.Node {
	return corev1.Node{Status: corev1.NodeStatus{Conditions: conditions}}
}

// TestInstanceStatusMessage: the wait counts Ready nodes, and a machine that boots but never
// finishes bashible is simply absent from that count. What went wrong is in the Instance the node
// controller keeps for it, written there by the failing step itself.
func TestInstanceStatusMessage(t *testing.T) {
	instance := func(conditions ...map[string]any) map[string]any {
		raw := make([]any, 0, len(conditions))
		for _, condition := range conditions {
			raw = append(raw, condition)
		}
		return map[string]any{"status": map[string]any{"conditions": raw}}
	}

	tests := []struct {
		name   string
		object map[string]any
		want   string
	}{
		{
			name: "bashible failed on a step",
			object: instance(map[string]any{
				"type":    "BashibleReady",
				"status":  "False",
				"reason":  "StepsFailed",
				"message": "step 031_install_kubernetes_api_proxy.sh failed: bb-pkg: action 'install' is not supported",
			}),
			want: "BashibleReady is StepsFailed: step 031_install_kubernetes_api_proxy.sh failed: bb-pkg: action 'install' is not supported",
		},
		{
			// The machine never came up, so bashible never ran and its condition is absent.
			name: "the machine itself is the problem",
			object: instance(map[string]any{
				"type":    "MachineReady",
				"status":  "False",
				"reason":  "Error",
				"message": "Cloud provider message - InvalidParameterValue: image not found",
			}),
			want: "MachineReady is Error: Cloud provider message - InvalidParameterValue: image not found",
		},
		{
			// Both are unhappy: bashible is what the operator acts on, and repeating the
			// machine's condition underneath adds nothing.
			name: "bashible outranks the machine",
			object: instance(
				map[string]any{"type": "MachineReady", "status": "False", "reason": "Progressing"},
				map[string]any{"type": "BashibleReady", "status": "False", "reason": "StepsFailed", "message": "no space left on device"},
			),
			want: "BashibleReady is StepsFailed: no space left on device",
		},
		{
			name: "a condition with no message still names its reason",
			object: instance(map[string]any{
				"type": "BashibleReady", "status": "False", "reason": "WaitingForApproval",
			}),
			want: "BashibleReady is WaitingForApproval",
		},
		{
			name: "a healthy instance says nothing",
			object: instance(
				map[string]any{"type": "MachineReady", "status": "True"},
				map[string]any{"type": "BashibleReady", "status": "True", "reason": "StepsCompleted"},
			),
			want: "",
		},
		{
			name:   "an instance with no conditions yet",
			object: map[string]any{"status": map[string]any{}},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, instanceStatusMessage(tt.object))
		})
	}
}
