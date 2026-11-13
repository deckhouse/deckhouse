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
	"k8s.io/apimachinery/pkg/runtime"
)

// A nil status section must not be serialized.
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

func TestFallbackHeartbeatKeepsMicrosecondsOnTheWire(t *testing.T) {
	at := metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 2, 250000000, time.UTC))

	raw, err := json.Marshal(FencingFailedNodeStateFallback{LastHeartbeatAt: &at})
	if err != nil {
		t.Fatalf("marshal fallback: %v", err)
	}

	if want := `{"lastHeartbeatAt":"2026-06-02T15:00:02.250000Z"}`; string(raw) != want {
		t.Errorf("fallback marshaled as %s, want %s", raw, want)
	}

	var decoded FencingFailedNodeStateFallback
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal fallback: %v", err)
	}

	if !decoded.LastHeartbeatAt.Equal(&at) {
		t.Errorf("lastHeartbeatAt decoded as %v, want %v", decoded.LastHeartbeatAt, at)
	}

	wholeSeconds := []byte(`{"lastHeartbeatAt":"2026-06-02T15:00:02Z"}`)
	if err := json.Unmarshal(wholeSeconds, &decoded); err == nil {
		t.Error("a lastHeartbeatAt without fractional digits was decoded, want an error")
	}
}

// Every type must resolve to its expected GVK.
func TestSchemeRegistersEveryKind(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}

	objects := map[string]runtime.Object{
		"FencingFailedNodeState":     &FencingFailedNodeState{},
		"FencingFailedNodeStateList": &FencingFailedNodeStateList{},
		"FencingAgentNodeView":       &FencingAgentNodeView{},
		"FencingAgentNodeViewList":   &FencingAgentNodeViewList{},
		"FencingSLAProfile":          &FencingSLAProfile{},
		"FencingSLAProfileList":      &FencingSLAProfileList{},
	}

	for wantKind, obj := range objects {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			t.Errorf("%s: object kinds: %v", wantKind, err)
			continue
		}

		if len(gvks) != 1 {
			t.Errorf("%s: expected exactly one GVK, got %v", wantKind, gvks)
			continue
		}

		if gvks[0].GroupVersion() != GroupVersion || gvks[0].Kind != wantKind {
			t.Errorf("%s: registered as %s, want %s", wantKind, gvks[0], GroupVersion.WithKind(wantKind))
		}
	}
}

// DeepCopy must not alias the pointer sections.
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
