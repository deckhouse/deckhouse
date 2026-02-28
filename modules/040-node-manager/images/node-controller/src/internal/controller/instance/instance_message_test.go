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
)

func TestMessageFromConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conditions []deckhousev1alpha2.InstanceCondition
		expected   string
	}{
		{
			name: "machine ready message has highest priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Message: "bashible"},
				{Type: deckhousev1alpha2.InstanceConditionTypeMachineReady, Message: "machine"},
			},
			expected: "machine: machine",
		},
		{
			name: "bashible ready is second priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeMachineReady, Message: ""},
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Message: "bashible"},
			},
			expected: "bashible: bashible",
		},
		{
			name: "disruption approval not required reason is third priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingApproval, Message: "waiting approval"},
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval, Reason: conditionReasonDisruptionApprovalNotRequired, Message: "no disruption approval"},
			},
			expected: "bashible: no disruption approval",
		},
		{
			name: "waiting approval is fourth priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingApproval, Message: "waiting approval"},
			},
			expected: "bashible: waiting approval",
		},
		{
			name: "disruption approval reason on another condition type is ignored",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:    deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Reason:  conditionReasonDisruptionApprovalNotRequired,
					Message: "waiting approval",
				},
			},
			expected: "bashible: waiting approval",
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
