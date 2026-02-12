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

package bashiblecleanup

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl}}
}

func getNode(t *testing.T, r *Reconciler, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node
}

func reconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func TestReconcile_NodeWithLabelAndTaint_RemovesBoth(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
				"other-label":                "keep",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
				{Key: "other-taint", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")

	if _, exists := updated.Labels[bashibleFirstRunFinishedLabel]; exists {
		t.Fatal("expected bashible label to be removed")
	}
	if updated.Labels["other-label"] != "keep" {
		t.Fatal("expected other label to be preserved")
	}

	for _, taint := range updated.Spec.Taints {
		if taint.Key == bashibleUninitializedTaintKey {
			t.Fatal("expected bashible taint to be removed")
		}
	}
	if len(updated.Spec.Taints) != 1 || updated.Spec.Taints[0].Key != "other-taint" {
		t.Fatalf("expected only other-taint to remain, got %+v", updated.Spec.Taints)
	}
}

func TestReconcile_NodeWithLabelOnly_RemovesLabel(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Labels[bashibleFirstRunFinishedLabel]; exists {
		t.Fatal("expected bashible label to be removed")
	}
}

func TestReconcile_NodeWithoutLabel_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"some-label": "value"},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	// Taint should remain since label was not present
	hasTaint := false
	for _, taint := range updated.Spec.Taints {
		if taint.Key == bashibleUninitializedTaintKey {
			hasTaint = true
		}
	}
	if !hasTaint {
		t.Fatal("expected taint to remain when label is not present")
	}
}

func TestReconcile_NodeWithOnlyBashibleTaint_SetsTaintsToNil(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: bashibleUninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if len(updated.Spec.Taints) != 0 {
		t.Fatalf("expected taints to be empty, got %+v", updated.Spec.Taints)
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "nonexistent")
}
