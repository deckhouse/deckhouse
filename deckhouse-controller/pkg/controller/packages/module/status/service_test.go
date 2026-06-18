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

package status

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func cond(t status.ConditionType, s metav1.ConditionStatus, reason, msg string) status.Condition {
	return status.Condition{Type: t, Status: s, Reason: status.ConditionReason(reason), Message: msg}
}

func TestComputeModuleStatus(t *testing.T) {
	tests := []struct {
		name      string
		in        status.Status
		wantPhase string
		wantReady bool
		wantReas  string
	}{
		{
			name: "fully ready",
			in: status.Status{Conditions: []status.Condition{
				cond(status.ConditionRequirementsMet, metav1.ConditionTrue, "", ""),
				cond(status.ConditionLoaded, metav1.ConditionTrue, "", ""),
				cond(status.ConditionCRDsEnsured, metav1.ConditionTrue, "", ""),
				cond(status.ConditionConfigured, metav1.ConditionTrue, "", ""),
				cond(status.ConditionHooksProcessed, metav1.ConditionTrue, "", ""),
				cond(status.ConditionManifestsApplied, metav1.ConditionTrue, "", ""),
				cond(status.ConditionScaled, metav1.ConditionTrue, "", ""),
			}},
			wantPhase: v1alpha1.ModulePhaseReady,
			wantReady: true,
		},
		{
			name: "ready when workloads not yet observed",
			in: status.Status{Conditions: []status.Condition{
				cond(status.ConditionHooksProcessed, metav1.ConditionTrue, "", ""),
				cond(status.ConditionManifestsApplied, metav1.ConditionTrue, "", ""),
				cond(status.ConditionScaled, metav1.ConditionUnknown, "", ""),
			}},
			wantPhase: v1alpha1.ModulePhaseReady,
			wantReady: true,
		},
		{
			name: "error on failed condition with reason",
			in: status.Status{Conditions: []status.Condition{
				cond(status.ConditionLoaded, metav1.ConditionTrue, "", ""),
				cond(status.ConditionCRDsEnsured, metav1.ConditionFalse, "EnsureCRDsFailed", "boom"),
			}},
			wantPhase: v1alpha1.ModulePhaseError,
			wantReady: false,
			wantReas:  "EnsureCRDsFailed",
		},
		{
			name: "reconciling while pipeline in progress",
			in: status.Status{Conditions: []status.Condition{
				cond(status.ConditionLoaded, metav1.ConditionTrue, "", ""),
				cond(status.ConditionConfigured, metav1.ConditionTrue, "", ""),
				cond(status.ConditionManifestsApplied, metav1.ConditionUnknown, "", ""),
			}},
			wantPhase: v1alpha1.ModulePhaseReconciling,
			wantReady: false,
			wantReas:  v1alpha1.ModuleReasonReconciling,
		},
		{
			name: "scaled false keeps module not ready",
			in: status.Status{Conditions: []status.Condition{
				cond(status.ConditionHooksProcessed, metav1.ConditionTrue, "", ""),
				cond(status.ConditionManifestsApplied, metav1.ConditionTrue, "", ""),
				cond(status.ConditionScaled, metav1.ConditionFalse, "", ""),
			}},
			wantPhase: v1alpha1.ModulePhaseReconciling,
			wantReady: false,
			wantReas:  v1alpha1.ModuleReasonReconciling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, ready, reason, _ := computeModuleStatus(tt.in)
			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
			if ready != tt.wantReady {
				t.Errorf("ready = %v, want %v", ready, tt.wantReady)
			}
			if tt.wantReas != "" && reason != tt.wantReas {
				t.Errorf("reason = %q, want %q", reason, tt.wantReas)
			}
		})
	}
}
