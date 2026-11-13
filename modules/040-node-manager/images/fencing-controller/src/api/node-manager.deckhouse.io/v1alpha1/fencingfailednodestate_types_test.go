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
	heartbeat := metav1.NewMicroTime(time.Unix(0, 0).UTC())

	tests := []struct {
		name    string
		status  FencingFailedNodeStateStatus
		present string
		absent  string
	}{
		{
			name: "witness does not serialize fallback",
			status: FencingFailedNodeStateStatus{
				Attestation: &FencingFailedNodeStateAttestation{
					Witness: "worker-1",
					At:      metav1.NewMicroTime(time.Unix(0, 0).UTC()),
				},
			},
			present: `"attestation"`,
			absent:  `"fallback"`,
		},
		{
			name: "fallback writer does not serialize attestation",
			status: FencingFailedNodeStateStatus{
				Fallback: &FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat},
			},
			present: `"fallback"`,
			absent:  `"attestation"`,
		},
		{
			name: "controller writer serializes neither agent section",
			status: FencingFailedNodeStateStatus{
				Phase: PhaseSuspected,
			},
			present: `"phase"`,
			absent:  `"attestation"`,
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

// The CRD requires armed and accepts profileRef only for an armed Node.
func TestUnarmedSpecHasArmedAndNoProfile(t *testing.T) {
	raw, err := json.Marshal(FencingFailedNodeStateSpec{Source: SourceMemberlistDead})
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, `"armed":false`) {
		t.Errorf("expected \"armed\":false in %s", got)
	}

	if strings.Contains(got, `"profileRef"`) {
		t.Errorf("unexpected profileRef in %s", got)
	}
}

// TestDeepCopyIsIndependent checks DeepCopy does not alias the pointer sections.
// The client hands out a copy of the cached object, so aliasing would corrupt the
// shared informer cache.
func TestDeepCopyIsIndependent(t *testing.T) {
	heartbeat := metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 2, 0, time.UTC))

	original := &FencingFailedNodeState{
		Spec: FencingFailedNodeStateSpec{
			Armed:      true,
			ProfileRef: &ProfileRef{Name: ProfileCritical},
		},
		Status: FencingFailedNodeStateStatus{
			Attestation: &FencingFailedNodeStateAttestation{Witness: "worker-1"},
			Fallback:    &FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat},
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
			},
		},
	}

	copied := original.DeepCopy()
	copied.Spec.ProfileRef.Name = ProfileSlow
	copied.Status.Attestation.Witness = "worker-2"
	copied.Status.Fallback.LastHeartbeatAt.Time = heartbeat.Add(time.Hour)
	copied.Status.Conditions[0].Type = "NotReady"

	if original.Spec.ProfileRef.Name != ProfileCritical {
		t.Errorf("profileRef aliased: %+v", original.Spec.ProfileRef)
	}

	if original.Status.Attestation.Witness != "worker-1" {
		t.Errorf("attestation section aliased: %+v", original.Status.Attestation)
	}

	if !original.Status.Fallback.LastHeartbeatAt.Equal(&heartbeat) {
		t.Errorf("fallback section aliased: %+v", original.Status.Fallback)
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
		ProfileNormal:   "normal",
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
