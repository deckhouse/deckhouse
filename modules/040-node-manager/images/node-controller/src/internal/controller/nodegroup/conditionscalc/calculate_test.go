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

package conditions

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const nowTime = "2021-01-01T13:30:00Z"

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return v
}

func statusOf(conds []NodeGroupCondition, typ NodeGroupConditionType) (ConditionStatus, bool) {
	for _, c := range conds {
		if c.Type == typ {
			return c.Status, true
		}
	}
	return "", false
}

func TestNodeToConditionsNode(t *testing.T) {
	created := mustParse(t, "2020-12-31T00:00:00Z")

	tests := []struct {
		name string
		node *corev1.Node
		want Node
	}{
		{
			name: "ready node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(created)},
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				}},
			},
			want: Node{Ready: true, CreationTimestamp: created},
		},
		{
			name: "not ready node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				}},
			},
			want: Node{Ready: false},
		},
		{
			name: "to be deleted taint marks ShouldDeleted",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{Taints: []corev1.Taint{
					{Key: "ToBeDeletedByClusterAutoscaler"},
				}},
			},
			want: Node{ShouldDeleted: true},
		},
		{
			name: "disruption required but not approved waits for approval and updating",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					"update.node.deckhouse.io/disruption-required": "",
				}},
			},
			want: Node{WaitingDisruptiveApproval: true, Updating: true},
		},
		{
			name: "disruption required and approved is not waiting but updating",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					"update.node.deckhouse.io/disruption-required": "",
					"update.node.deckhouse.io/disruption-approved": "",
				}},
			},
			want: Node{WaitingDisruptiveApproval: false, Updating: true},
		},
		{
			name: "generic update annotation marks updating",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					"update.node.deckhouse.io/approved": "",
				}},
			},
			want: Node{Updating: true},
		},
		{
			name: "unschedulable",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{Unschedulable: true},
			},
			want: Node{Unschedulable: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NodeToConditionsNode(tt.node)
			if got.Ready != tt.want.Ready ||
				got.ShouldDeleted != tt.want.ShouldDeleted ||
				got.Unschedulable != tt.want.Unschedulable ||
				got.Updating != tt.want.Updating ||
				got.WaitingDisruptiveApproval != tt.want.WaitingDisruptiveApproval ||
				!got.CreationTimestamp.Equal(tt.want.CreationTimestamp) {
				t.Fatalf("NodeToConditionsNode() = %#v, want %#v", *got, tt.want)
			}
		})
	}
}

func TestCalculateNodeGroupConditions(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)
	now := mustParse(t, nowTime)

	tests := []struct {
		name              string
		ng                NodeGroup
		nodes             []*Node
		currentConditions []NodeGroupCondition
		errors            []string
		minPerAllZone     int
		wantReady         ConditionStatus
		wantUpdating      ConditionStatus
		wantWaiting       ConditionStatus
		wantError         ConditionStatus
		wantErrorMsg      string
		wantScalingSet    bool
		wantScaling       ConditionStatus
	}{
		{
			name:           "cloud ephemeral ready when ready nodes meet min",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 2, Instances: 2},
			nodes:          []*Node{{Ready: true}, {Ready: true}},
			minPerAllZone:  2,
			wantReady:      ConditionTrue,
			wantUpdating:   ConditionFalse,
			wantWaiting:    ConditionFalse,
			wantError:      ConditionFalse,
			wantScalingSet: true,
			wantScaling:    ConditionFalse,
		},
		{
			name:           "cloud ephemeral not ready when below min",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 3, Instances: 3},
			nodes:          []*Node{{Ready: true}},
			minPerAllZone:  2,
			wantReady:      ConditionFalse,
			wantScalingSet: true,
			wantScaling:    ConditionTrue, // desired(3) > len(nodes)(1) => upscale
		},
		{
			name:           "grace period counts unready node as ready",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1},
			nodes:          []*Node{{Ready: false, CreationTimestamp: now.Add(-1 * time.Minute)}},
			minPerAllZone:  1,
			wantReady:      ConditionTrue,
			wantScalingSet: true,
		},
		{
			name:           "unschedulable node never counts as ready",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1},
			nodes:          []*Node{{Ready: true, Unschedulable: true}},
			minPerAllZone:  1,
			wantReady:      ConditionFalse,
			wantScalingSet: true,
		},
		{
			name:           "updating and waiting approval propagate",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1},
			nodes:          []*Node{{Ready: true, Updating: true, WaitingDisruptiveApproval: true}},
			minPerAllZone:  1,
			wantReady:      ConditionTrue,
			wantUpdating:   ConditionTrue,
			wantWaiting:    ConditionTrue,
			wantScalingSet: true,
		},
		{
			name:           "downscale via ToBeDeleted taint sets scaling",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1},
			nodes:          []*Node{{Ready: true, ShouldDeleted: true}},
			minPerAllZone:  1,
			wantReady:      ConditionTrue,
			wantScalingSet: true,
			wantScaling:    ConditionTrue,
		},
		{
			name:           "downscale when desired below instances",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 3},
			nodes:          []*Node{{Ready: true}},
			minPerAllZone:  1,
			wantReady:      ConditionTrue,
			wantScalingSet: true,
			wantScaling:    ConditionTrue,
		},
		{
			name:           "error condition set when errors present",
			ng:             NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1},
			nodes:          []*Node{{Ready: true}},
			errors:         []string{"boom", "bang"},
			minPerAllZone:  1,
			wantReady:      ConditionTrue,
			wantError:      ConditionTrue,
			wantErrorMsg:   "boom|bang",
			wantScalingSet: true,
		},
		{
			name:           "static ready when ready equals desired",
			ng:             NodeGroup{Type: NodeTypeStatic, Desired: 2},
			nodes:          []*Node{{Ready: true}, {Ready: true}},
			minPerAllZone:  5,
			wantReady:      ConditionTrue,
			wantScalingSet: false,
		},
		{
			name:           "static not ready when ready below desired",
			ng:             NodeGroup{Type: NodeTypeStatic, Desired: 3},
			nodes:          []*Node{{Ready: true}, {Ready: true}},
			minPerAllZone:  0,
			wantReady:      ConditionFalse,
			wantScalingSet: false,
		},
		{
			name:           "static no desired ready when all nodes ready",
			ng:             NodeGroup{Type: NodeTypeStatic, Desired: 0},
			nodes:          []*Node{{Ready: true}, {Ready: true}},
			minPerAllZone:  0,
			wantReady:      ConditionTrue,
			wantScalingSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateNodeGroupConditions(tt.ng, tt.nodes, tt.currentConditions, tt.errors, tt.minPerAllZone)

			if s, _ := statusOf(got, NodeGroupConditionTypeReady); s != tt.wantReady {
				t.Errorf("Ready = %q, want %q", s, tt.wantReady)
			}
			if tt.wantUpdating != "" {
				if s, _ := statusOf(got, NodeGroupConditionTypeUpdating); s != tt.wantUpdating {
					t.Errorf("Updating = %q, want %q", s, tt.wantUpdating)
				}
			}
			if tt.wantWaiting != "" {
				if s, _ := statusOf(got, NodeGroupConditionTypeWaitingForDisruptiveApproval); s != tt.wantWaiting {
					t.Errorf("Waiting = %q, want %q", s, tt.wantWaiting)
				}
			}
			if tt.wantError != "" {
				if s, _ := statusOf(got, NodeGroupConditionTypeError); s != tt.wantError {
					t.Errorf("Error = %q, want %q", s, tt.wantError)
				}
			}
			if tt.wantErrorMsg != "" {
				for _, c := range got {
					if c.Type == NodeGroupConditionTypeError && c.Message != tt.wantErrorMsg {
						t.Errorf("Error message = %q, want %q", c.Message, tt.wantErrorMsg)
					}
				}
			}

			_, scalingSet := statusOf(got, NodeGroupConditionTypeScaling)
			if scalingSet != tt.wantScalingSet {
				t.Errorf("Scaling present = %v, want %v", scalingSet, tt.wantScalingSet)
			}
			if tt.wantScalingSet && tt.wantScaling != "" {
				if s, _ := statusOf(got, NodeGroupConditionTypeScaling); s != tt.wantScaling {
					t.Errorf("Scaling = %q, want %q", s, tt.wantScaling)
				}
			}

			for _, c := range got {
				if c.LastTransitionTime.IsZero() {
					t.Errorf("condition %s has zero LastTransitionTime", c.Type)
				}
			}
		})
	}
}

func TestCalculateNodeGroupConditions_TransitionTimePreserved(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)
	earlier := metav1.NewTime(mustParse(t, "2020-01-01T00:00:00Z"))

	current := []NodeGroupCondition{
		{Type: NodeGroupConditionTypeReady, Status: ConditionTrue, LastTransitionTime: earlier},
	}
	ng := NodeGroup{Type: NodeTypeStatic, Desired: 1}
	nodes := []*Node{{Ready: true}}

	got := CalculateNodeGroupConditions(ng, nodes, current, nil, 0)

	for _, c := range got {
		if c.Type == NodeGroupConditionTypeReady {
			if !c.LastTransitionTime.Equal(&earlier) {
				t.Fatalf("expected preserved transition time %v, got %v", earlier, c.LastTransitionTime)
			}
		}
	}
}

func TestCalcErrorCondition_FrozenKeepsPreviousError(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)
	current := []NodeGroupCondition{
		{Type: NodeGroupConditionTypeError, Status: ConditionTrue, Message: "previous failure"},
	}
	ng := NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1, HasFrozenMachineDeployment: true}
	nodes := []*Node{{Ready: true}}

	got := CalculateNodeGroupConditions(ng, nodes, current, nil, 1)

	for _, c := range got {
		if c.Type == NodeGroupConditionTypeError {
			if c.Status != ConditionTrue || c.Message != "previous failure" {
				t.Fatalf("expected frozen NG to keep previous error, got %#v", c)
			}
		}
	}
}

func TestCalcErrorCondition_MachineGeneralError(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)

	tests := []struct {
		name       string
		current    []NodeGroupCondition
		wantStatus ConditionStatus
		wantMsg    string
	}{
		{
			name:       "no previous error returns general error",
			current:    nil,
			wantStatus: ConditionTrue,
			wantMsg:    machineGeneralError,
		},
		{
			name: "previous false error overridden by general error",
			current: []NodeGroupCondition{
				{Type: NodeGroupConditionTypeError, Status: ConditionFalse, Message: ""},
			},
			wantStatus: ConditionTrue,
			wantMsg:    machineGeneralError,
		},
		{
			name: "previous real error kept over general error",
			current: []NodeGroupCondition{
				{Type: NodeGroupConditionTypeError, Status: ConditionTrue, Message: "real failure"},
			},
			wantStatus: ConditionTrue,
			wantMsg:    "real failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := NodeGroup{Type: NodeTypeCloudEphemeral, Desired: 1, Instances: 1}
			nodes := []*Node{{Ready: true}}
			got := CalculateNodeGroupConditions(ng, nodes, tt.current, []string{machineGeneralError}, 1)

			for _, c := range got {
				if c.Type == NodeGroupConditionTypeError {
					if c.Status != tt.wantStatus || c.Message != tt.wantMsg {
						t.Fatalf("error condition = {%s, %q}, want {%s, %q}", c.Status, c.Message, tt.wantStatus, tt.wantMsg)
					}
				}
			}
		})
	}
}

func TestNodeGroupConditionDeepCopy(t *testing.T) {
	var nilCond *NodeGroupCondition
	if nilCond.DeepCopy() != nil {
		t.Fatal("DeepCopy of nil should be nil")
	}

	orig := &NodeGroupCondition{Type: NodeGroupConditionTypeError, Status: ConditionTrue, Message: "m"}
	cp := orig.DeepCopy()
	if cp == orig {
		t.Fatal("DeepCopy returned same pointer")
	}
	if cp.Type != orig.Type || cp.Status != orig.Status || cp.Message != orig.Message {
		t.Fatalf("DeepCopy mismatch: %#v vs %#v", cp, orig)
	}
}
