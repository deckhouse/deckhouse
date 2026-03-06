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

package instance

import (
	"testing"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMessageFromConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conditions []deckhousev1alpha2.InstanceCondition
		expected   string
	}{
		{
			name: "machine problem has highest priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:  metav1.ConditionFalse,
					Message: "machine message",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionTrue,
					Message: "last successful step",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval,
					Status:  metav1.ConditionTrue,
					Reason:  "DisruptionApprovalRequired",
					Message: "100_disable-ntp-on-node.sh requires disruption approval",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status:  metav1.ConditionTrue,
					Message: "waiting for approval",
				},
			},
			expected: "machine: machine message",
		},
		{
			name: "machine not selected when true ready",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:  metav1.ConditionTrue,
					Reason:  "Ready",
					Message: "machine is ready",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionFalse,
					Message: "bashible failed",
				},
			},
			expected: "bashible: bashible failed",
		},
		{
			name: "bashible problem wins over waiting disruption",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionFalse,
					Message: "No Bashible reconciliation for 5m: waiting for approval",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval,
					Status:  metav1.ConditionTrue,
					Message: "requires disruption approval",
				},
			},
			expected: "bashible: No Bashible reconciliation for 5m: waiting for approval",
		},
		{
			name: "waiting disruption true wins over waiting approval true",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval,
					Status:  metav1.ConditionTrue,
					Message: "requires disruption approval",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status:  metav1.ConditionTrue,
					Message: "waiting for approval",
				},
			},
			expected: "bashible: requires disruption approval",
		},
		{
			name: "waiting approval true wins over bashible ready true",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionTrue,
					Message: "last successful step",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status:  metav1.ConditionTrue,
					Message: "waiting for approval",
				},
			},
			expected: "bashible: waiting for approval",
		},
		{
			name: "bashible ready fallback when no problems",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeMachineReady, Status: metav1.ConditionTrue, Reason: "Ready"},
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionTrue, Message: "bashible"},
			},
			expected: "bashible: bashible",
		},
		{
			name: "waiting approval is selected only when true",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingApproval, Status: metav1.ConditionFalse, Message: "waiting approval"},
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionTrue, Message: "bashible ok"},
			},
			expected: "bashible: bashible ok",
		},
		{
			name: "machine message skipped when empty then bashible message used",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:  metav1.ConditionFalse,
					Message: "  ",
				},
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:  metav1.ConditionTrue,
					Message: "bashible ok",
				},
			},
			expected: "bashible: bashible ok",
		},
		{
			name: "empty messages return empty result",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeMachineReady, Message: "  "},
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Message: ""},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := messageFromConditions(tt.conditions)
			if actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
