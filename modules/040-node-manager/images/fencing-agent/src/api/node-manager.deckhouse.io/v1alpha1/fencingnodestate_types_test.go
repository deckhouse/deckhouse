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

// TestStatusSectionsAreIsolated checks a nil status section is not serialized.
func TestStatusSectionsAreIsolated(t *testing.T) {
	tests := []struct {
		name    string
		status  FencingNodeStateStatus
		present string
		absent  string
	}{
		{
			name: "failed writer does not serialize fallback",
			status: FencingNodeStateStatus{
				Failed: &FencingNodeStateFailed{
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
			status: FencingNodeStateStatus{
				Fallback: &FencingNodeStateFallback{
					Active:                   true,
					APIReachable:             true,
					HeartbeatIntervalSeconds: 1,
				},
			},
			present: `"fallback"`,
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

// TestZeroValuedFieldsSurvive checks zero-valued fields stay on the wire.
func TestZeroValuedFieldsSurvive(t *testing.T) {
	raw, err := json.Marshal(FencingNodeStateStatus{
		Failed:   &FencingNodeStateFailed{AliveCount: 0, QuorumSize: 3},
		Fallback: &FencingNodeStateFallback{Active: false, APIReachable: false},
	})
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}

	for _, want := range []string{`"aliveCount":0`, `"active":false`, `"apiReachable":false`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("expected %s in %s", want, raw)
		}
	}
}

// TestSchemeRegistersEveryKind asserts every type resolves to its expected GVK.
func TestSchemeRegistersEveryKind(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("add to scheme: %v", err)
	}

	objects := map[string]runtime.Object{
		"FencingNodeState":         &FencingNodeState{},
		"FencingNodeStateList":     &FencingNodeStateList{},
		"FencingAgentNodeView":     &FencingAgentNodeView{},
		"FencingAgentNodeViewList": &FencingAgentNodeViewList{},
		"FencingAgentPeer":         &FencingAgentPeer{},
		"FencingAgentPeerList":     &FencingAgentPeerList{},
		"FencingAgentEvent":        &FencingAgentEvent{},
		"FencingAgentEventList":    &FencingAgentEventList{},
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

// TestDeepCopyIsIndependent checks DeepCopy does not alias the pointer sections.
func TestDeepCopyIsIndependent(t *testing.T) {
	original := &FencingNodeState{
		Spec: FencingNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: ProfileRef{Name: ProfileCritical},
		},
		Status: FencingNodeStateStatus{
			Failed:   &FencingNodeStateFailed{DetectedBy: "worker-1", AliveCount: 3},
			Fallback: &FencingNodeStateFallback{Active: true, HeartbeatIntervalSeconds: 1},
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
