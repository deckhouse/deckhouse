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

package draining

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{
		Base:       dynctrl.Base{Client: cl, Recorder: record.NewFakeRecorder(10)},
		kubeClient: fake.NewSimpleClientset(),
	}
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

func TestReconcile_NoAnnotations_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Spec.Unschedulable {
		t.Fatal("node should not be cordoned without draining annotation")
	}
}

func TestReconcile_DrainingAnnotation_CordonsAndDrains(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")

	// Should be cordoned
	if !updated.Spec.Unschedulable {
		t.Fatal("node should be cordoned after draining")
	}

	// Draining annotation should be removed, drained should be set
	if _, exists := updated.Annotations[nodecommon.DrainingAnnotation]; exists {
		t.Fatal("draining annotation should be removed")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestReconcile_DrainedUserSchedulable_RemovesDrained(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainedAnnotation: "user"},
		},
		Spec: corev1.NodeSpec{Unschedulable: false},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Annotations[nodecommon.DrainedAnnotation]; exists {
		t.Fatal("drained annotation should be removed for schedulable node with user source")
	}
}

func TestReconcile_DrainedNonUser_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainedAnnotation: "bashible"},
		},
		Spec: corev1.NodeSpec{Unschedulable: false},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatal("drained annotation should remain for non-user source")
	}
}

func TestReconcile_AlreadyCordoned_StillDrains(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
		Spec: corev1.NodeSpec{Unschedulable: true},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Annotations[nodecommon.DrainingAnnotation]; exists {
		t.Fatal("draining annotation should be removed")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestReconcile_DrainingWithUserDrained_RemovesDrainedFirst(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{
				nodecommon.DrainingAnnotation: "bashible",
				nodecommon.DrainedAnnotation:  "user",
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation to be set to 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestGetDrainTimeout_FromNodeGroup(t *testing.T) {
	timeout := 300
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:               v1.NodeTypeStatic,
			NodeDrainTimeoutSecond: &timeout,
		},
	}

	r := newReconciler(t, ng)
	got := r.getDrainTimeout(context.Background(), "worker")

	expected := 300 * time.Second
	if got != expected {
		t.Fatalf("expected timeout %v, got %v", expected, got)
	}
}

func TestGetDrainTimeout_Default(t *testing.T) {
	r := newReconciler(t)
	got := r.getDrainTimeout(context.Background(), "nonexistent")

	if got != defaultDrainTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultDrainTimeout, got)
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "nonexistent")
}
