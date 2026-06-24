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

package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	conditionscalc "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditionscalc"
)

func TestNodeToNodeGroup_NoLabelAndWrongType(t *testing.T) {
	// node without the group label -> no requests
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1"}}
	if reqs := NodeToNodeGroup(context.Background(), node); reqs != nil {
		t.Fatalf("expected nil for unlabeled node, got %#v", reqs)
	}

	// non-node object -> no requests
	if reqs := NodeToNodeGroup(context.Background(), &corev1.Pod{}); reqs != nil {
		t.Fatalf("expected nil for non-node object, got %#v", reqs)
	}
}

func TestMachineToNodeGroup(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{name: "deckhouse group label", labels: map[string]string{NodeGroupLabel: "ng-a"}, want: "ng-a"},
		{name: "node-group fallback label", labels: map[string]string{"node-group": "ng-b"}, want: "ng-b"},
		{name: "no labels", labels: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mcmv1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: tt.labels}}
			reqs := MachineToNodeGroup(context.Background(), m)
			if tt.want == "" {
				if reqs != nil {
					t.Fatalf("expected nil, got %#v", reqs)
				}
				return
			}
			if len(reqs) != 1 || reqs[0].Name != tt.want {
				t.Fatalf("unexpected requests: %#v", reqs)
			}
		})
	}
}

func TestMachineDeploymentToNodeGroup(t *testing.T) {
	withLabel := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-group": "ng-md"}}}
	reqs := MachineDeploymentToNodeGroup(context.Background(), withLabel)
	if len(reqs) != 1 || reqs[0].Name != "ng-md" {
		t.Fatalf("unexpected requests: %#v", reqs)
	}

	noLabel := &corev1.Node{ObjectMeta: metav1.ObjectMeta{}}
	if reqs := MachineDeploymentToNodeGroup(context.Background(), noLabel); reqs != nil {
		t.Fatalf("expected nil for object without node-group label, got %#v", reqs)
	}
}

func TestNodeHasGroupLabelPredicate(t *testing.T) {
	p := NodeHasGroupLabelPredicate()

	withLabel := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{NodeGroupLabel: "x"}}}
	if !p.Create(event.CreateEvent{Object: withLabel}) {
		t.Error("expected predicate to accept node with group label")
	}

	without := &corev1.Node{ObjectMeta: metav1.ObjectMeta{}}
	if p.Create(event.CreateEvent{Object: without}) {
		t.Error("expected predicate to reject node without group label")
	}
}

func TestNewUnstructured(t *testing.T) {
	u := NewUnstructured(MCMMachineDeploymentGVK)
	if u.GroupVersionKind() != MCMMachineDeploymentGVK {
		t.Fatalf("unexpected GVK: %v", u.GroupVersionKind())
	}
}

func TestEnsureNonNilMachineFailures(t *testing.T) {
	if got := EnsureNonNilMachineFailures(nil); got == nil || len(got) != 0 {
		t.Fatalf("expected non-nil empty slice, got %#v", got)
	}
	in := []v1.MachineFailure{{Name: "m1"}}
	got := EnsureNonNilMachineFailures(in)
	if len(got) != 1 || got[0].Name != "m1" {
		t.Fatalf("expected input slice returned, got %#v", got)
	}
}

func TestConvertConditions_UnknownStatusDefaultsFalse(t *testing.T) {
	in := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionUnknown, Message: "?"},
	}
	calc := ConvertToCalcConditions(in)
	if len(calc) != 1 || calc[0].Status != conditionscalc.ConditionFalse {
		t.Fatalf("unknown metav1 status should map to ConditionFalse, got %#v", calc)
	}

	back := ConvertFromCalcConditions([]conditionscalc.NodeGroupCondition{
		{Type: "Ready", Status: conditionscalc.ConditionStatus("weird")},
	})
	if len(back) != 1 || back[0].Status != metav1.ConditionFalse {
		t.Fatalf("unknown calc status should map to metav1.ConditionFalse, got %#v", back)
	}
}
