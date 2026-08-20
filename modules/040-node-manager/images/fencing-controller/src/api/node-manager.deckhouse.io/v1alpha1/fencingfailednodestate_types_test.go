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

package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStatusSectionsAreIsolated checks a nil status section is not serialized,
// so the controller cannot wipe the sections owned by the agents.
func TestStatusSectionsAreIsolated(t *testing.T) {
	tests := []struct {
		name    string
		status  FencingFailedNodeStateStatus
		present string
		absent  string
	}{
		{
			name: "failed writer does not serialize fallback",
			status: FencingFailedNodeStateStatus{
				Failed: &FencingFailedNodeStateFailed{
					DetectedAt: metav1.NewTime(time.Unix(0, 0).UTC()),
					DetectedBy: "worker-1",
					Reason:     FailedReasonMemberlistDead,
					AliveCount: 3,
					QuorumSize: 3,
				},
			},
			present: `"failed"`,
			absent:  `"fallback"`,
		},
		{
			name: "fallback writer does not serialize failed",
			status: FencingFailedNodeStateStatus{
				Fallback: &FencingFailedNodeStateFallback{
					Active:                   true,
					HeartbeatIntervalSeconds: 1,
				},
			},
			present: `"fallback"`,
			absent:  `"failed"`,
		},
		{
			name: "controller writer serializes neither agent section",
			status: FencingFailedNodeStateStatus{
				Phase: PhaseSuspected,
			},
			present: `"phase"`,
			absent:  `"failed"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.status)
			if err != nil {
				t.Fatalf("marshal status: %v", err)
			}

			got := string(raw)
			if !strings.Contains(got, tt.present) {
				t.Errorf("expected %s in %s", tt.present, got)
			}

			if strings.Contains(got, tt.absent) {
				t.Errorf("unexpected %s in %s", tt.absent, got)
			}
		})
	}
}

// TestZeroValuedFieldsSurvive checks zero-valued fields stay on the wire:
// aliveCount 0 means every peer is gone, and active false means fallback ended.
func TestZeroValuedFieldsSurvive(t *testing.T) {
	raw, err := json.Marshal(FencingFailedNodeStateStatus{
		Failed:   &FencingFailedNodeStateFailed{AliveCount: 0, QuorumSize: 3},
		Fallback: &FencingFailedNodeStateFallback{Active: false},
	})
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}

	for _, want := range []string{`"aliveCount":0`, `"active":false`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("expected %s in %s", want, raw)
		}
	}
}

// TestDeepCopyIsIndependent checks DeepCopy does not alias the pointer sections.
// The client hands out a copy of the cached object, so aliasing would corrupt the
// shared informer cache.
func TestDeepCopyIsIndependent(t *testing.T) {
	original := &FencingFailedNodeState{
		Spec: FencingFailedNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: ProfileRef{Name: ProfileCritical},
		},
		Status: FencingFailedNodeStateStatus{
			Failed:   &FencingFailedNodeStateFailed{DetectedBy: "worker-1", AliveCount: 3},
			Fallback: &FencingFailedNodeStateFallback{Active: true, HeartbeatIntervalSeconds: 1},
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
			},
		},
	}

	copied := original.DeepCopy()
	copied.Status.Failed.DetectedBy = "worker-2"
	copied.Status.Failed.AliveCount = 0
	copied.Status.Fallback.Active = false
	copied.Status.Conditions[0].Type = "NotReady"

	if original.Status.Failed.DetectedBy != "worker-1" || original.Status.Failed.AliveCount != 3 {
		t.Errorf("failed section aliased: %+v", original.Status.Failed)
	}

	if !original.Status.Fallback.Active {
		t.Error("fallback section aliased")
	}

	if original.Status.Conditions[0].Type != "Ready" {
		t.Errorf("conditions aliased: %+v", original.Status.Conditions)
	}
}

// TestProfileObjectNameIsLowerCase pins the mapping between the capitalized enum
// value in spec.profileRef.name and the lower-cased FencingSLAProfile object name.
func TestProfileObjectNameIsLowerCase(t *testing.T) {
	want := map[ProfileName]string{
		ProfileCritical: "critical",
		ProfileMedium:   "medium",
		ProfileModerate: "moderate",
		ProfileSlow:     "slow",
	}

	profiles := ProfileNames()
	if len(profiles) != len(want) {
		t.Fatalf("ProfileNames returned %d profiles, want %d", len(profiles), len(want))
	}

	for _, profile := range profiles {
		if got := profile.ObjectName(); got != want[profile] {
			t.Errorf("%s: object name %q, want %q", profile, got, want[profile])
		}
	}
}
