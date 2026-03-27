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
	"time"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDesiredBashibleHeartbeatCondition(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stale := metav1.NewTime(now.Add(-11 * time.Minute))
	staleWaiting := metav1.NewTime(now.Add(-21 * time.Minute))

	tests := []struct {
		name         string
		conditions   []deckhousev1alpha2.InstanceCondition
		expectPatch  bool
		expectType   string
		expectState  metav1.ConditionStatus
		expectReason string
	}{
		{
			name: "do not override explicit bashible error with heartbeat unknown",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:            metav1.ConditionFalse,
					Reason:            "StepsFailed",
					Message:           "step failed",
					LastHeartbeatTime: &stale,
				},
			},
			expectPatch: false,
		},
		{
			name: "set heartbeat unknown for stale successful bashible state",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:            metav1.ConditionTrue,
					Reason:            "StepsCompleted",
					Message:           "ok",
					LastHeartbeatTime: &stale,
				},
			},
			expectPatch:  true,
			expectType:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			expectState:  metav1.ConditionUnknown,
			expectReason: bashibleHeartbeatReason,
		},
		{
			name: "do not apply waiting approval heartbeat when bashible is in error",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:            metav1.ConditionFalse,
					Reason:            "StepsFailed",
					Message:           "step failed",
					LastHeartbeatTime: &staleWaiting,
				},
				{
					Type:   deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status: metav1.ConditionTrue,
				},
			},
			expectPatch: false,
		},
		{
			name: "waiting approval heartbeat applies for non-error bashible state",
			conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:              deckhousev1alpha2.InstanceConditionTypeBashibleReady,
					Status:            metav1.ConditionTrue,
					Reason:            "StepsCompleted",
					Message:           "ok",
					LastHeartbeatTime: &staleWaiting,
				},
				{
					Type:   deckhousev1alpha2.InstanceConditionTypeWaitingApproval,
					Status: metav1.ConditionTrue,
				},
			},
			expectPatch:  true,
			expectType:   deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			expectState:  metav1.ConditionUnknown,
			expectReason: bashibleHeartbeatWaitingApprovalReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			updated, shouldPatch := desiredBashibleHeartbeatCondition(tt.conditions, now)
			if shouldPatch != tt.expectPatch {
				t.Fatalf("expected shouldPatch=%v, got %v", tt.expectPatch, shouldPatch)
			}
			if !tt.expectPatch {
				if updated != nil {
					t.Fatalf("expected nil updated condition, got %#v", *updated)
				}
				return
			}

			if updated == nil {
				t.Fatal("expected updated condition, got nil")
			}
			if updated.Type != tt.expectType {
				t.Fatalf("expected type %q, got %q", tt.expectType, updated.Type)
			}
			if updated.Status != tt.expectState {
				t.Fatalf("expected status %q, got %q", tt.expectState, updated.Status)
			}
			if updated.Reason != tt.expectReason {
				t.Fatalf("expected reason %q, got %q", tt.expectReason, updated.Reason)
			}
		})
	}
}
