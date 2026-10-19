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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

func TestDesiredNodeAgentCondition(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.October, 15, 12, 0, 0, 0, time.UTC)
	nodeConfig := func(systemType internalv1alpha1.SystemType, lastReport time.Time) *internalv1alpha1.NodeConfig {
		config := &internalv1alpha1.NodeConfig{Spec: internalv1alpha1.NodeSpec{SystemType: systemType}}
		config.Status.LastReconcileTime = metav1.NewTime(lastReport)
		return config
	}
	reporting := deckhousev1alpha2.InstanceCondition{
		Type:    deckhousev1alpha2.InstanceConditionTypeNodeAgentReady,
		Status:  metav1.ConditionTrue,
		Reason:  nodeAgentReportingReason,
		Message: nodeAgentReportingMessage,
	}

	tests := []struct {
		name          string
		conditions    []deckhousev1alpha2.InstanceCondition
		config        *internalv1alpha1.NodeConfig
		expectPatch   bool
		expectStatus  metav1.ConditionStatus
		expectReason  string
		expectMessage string
	}{
		{
			name:          "silent agent of an Immutable node",
			config:        nodeConfig(internalv1alpha1.SystemTypeImmutable, now.Add(-11*time.Minute)),
			expectPatch:   true,
			expectStatus:  metav1.ConditionUnknown,
			expectReason:  nodeAgentNoReportReason,
			expectMessage: "the node agent has not reported since 2026-10-15T11:49:00Z",
		},
		{
			name:          "reporting agent of an Immutable node",
			config:        nodeConfig(internalv1alpha1.SystemTypeImmutable, now.Add(-time.Minute)),
			expectPatch:   true,
			expectStatus:  metav1.ConditionTrue,
			expectReason:  nodeAgentReportingReason,
			expectMessage: nodeAgentReportingMessage,
		},
		{
			name:          "agent that fell silent after reporting",
			conditions:    []deckhousev1alpha2.InstanceCondition{reporting},
			config:        nodeConfig(internalv1alpha1.SystemTypeImmutable, now.Add(-11*time.Minute)),
			expectPatch:   true,
			expectStatus:  metav1.ConditionUnknown,
			expectReason:  nodeAgentNoReportReason,
			expectMessage: "the node agent has not reported since 2026-10-15T11:49:00Z",
		},
		{
			name:        "unchanged condition is not patched",
			conditions:  []deckhousev1alpha2.InstanceCondition{reporting},
			config:      nodeConfig(internalv1alpha1.SystemTypeImmutable, now.Add(-time.Minute)),
			expectPatch: false,
		},
		{
			name:          "silent agent of a bashible node",
			config:        nodeConfig(internalv1alpha1.SystemTypeMutable, now.Add(-11*time.Minute)),
			expectPatch:   true,
			expectStatus:  metav1.ConditionUnknown,
			expectReason:  nodeAgentNoReportReason,
			expectMessage: "the node agent has not reported since 2026-10-15T11:49:00Z",
		},
		{
			name:        "agent that has not finished a pass has nothing to age",
			config:      nodeConfig(internalv1alpha1.SystemTypeImmutable, time.Time{}),
			expectPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			desired, shouldPatch := desiredNodeAgentCondition(tt.conditions, tt.config, now)
			if shouldPatch != tt.expectPatch {
				t.Fatalf("expected shouldPatch=%v, got %v", tt.expectPatch, shouldPatch)
			}
			if !tt.expectPatch {
				return
			}

			if desired.Status != tt.expectStatus {
				t.Fatalf("expected status %q, got %q", tt.expectStatus, desired.Status)
			}
			if desired.Reason != tt.expectReason {
				t.Fatalf("expected reason %q, got %q", tt.expectReason, desired.Reason)
			}
			if desired.Message != tt.expectMessage {
				t.Fatalf("expected message %q, got %q", tt.expectMessage, desired.Message)
			}
			if desired.LastTransitionTime == nil {
				t.Fatal("expected lastTransitionTime to be set")
			}
		})
	}
}
