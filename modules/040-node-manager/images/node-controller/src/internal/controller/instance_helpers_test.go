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

package controller

import (
	"testing"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBashibleStatusFactory_FromConditions(t *testing.T) {
	t.Parallel()

	factory := NewBashibleStatusFactory()

	tests := []struct {
		name       string
		conditions []deckhousev1alpha2.InstanceCondition
		expected   deckhousev1alpha2.BashibleStatus
	}{
		{
			name: "waiting approval has highest priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionFalse},
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingApproval, Status: metav1.ConditionTrue},
			},
			expected: deckhousev1alpha2.BashibleStatusWaitingApproval,
		},
		{
			name: "waiting disruption approval has highest priority",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionTrue},
				{Type: deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval, Status: metav1.ConditionTrue},
			},
			expected: deckhousev1alpha2.BashibleStatusWaitingApproval,
		},
		{
			name: "bashible ready true",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionTrue},
			},
			expected: deckhousev1alpha2.BashibleStatusReady,
		},
		{
			name: "bashible ready false",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionFalse},
			},
			expected: deckhousev1alpha2.BashibleStatusError,
		},
		{
			name: "bashible ready unknown",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{Type: deckhousev1alpha2.InstanceConditionTypeBashibleReady, Status: metav1.ConditionUnknown},
			},
			expected: deckhousev1alpha2.BashibleStatusUnknown,
		},
		{
			name:       "bashible ready missing",
			conditions: nil,
			expected:   deckhousev1alpha2.BashibleStatusUnknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := factory.FromConditions(tt.conditions)
			if actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
