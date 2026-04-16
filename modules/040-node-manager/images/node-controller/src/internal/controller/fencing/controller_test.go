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

package fencing

import (
	"context"
	"testing"
	"time"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := coordinationv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add coordinationv1 scheme: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithIndex(&corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return nil
			}
			return []string{pod.Spec.NodeName}
		}).
		Build()
	return &Reconciler{Base: register.Base{Client: cl}}
}

func reconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func reconcileWithError(t *testing.T, r *Reconciler, name string) (ctrl.Result, error) {
	t.Helper()
	return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
}

func nodeExists(r *Reconciler, name string) bool {
	node := &corev1.Node{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node)
	if errors.IsNotFound(err) {
		return false
	}
	// Fake client may set DeletionTimestamp instead of removing immediately
	return node.DeletionTimestamp == nil || node.DeletionTimestamp.IsZero()
}

func TestReconcile_FreshLease_RequeuesOnly(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{fencingEnabledLabel: ""},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now())
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	res := reconcile(t, r, "node-1")

	if res.RequeueAfter != requeueInterval {
		t.Fatalf("expected requeue after %v, got %v", requeueInterval, res.RequeueAfter)
	}
	if !nodeExists(r, "node-1") {
		t.Fatal("node should not be deleted with fresh lease")
	}
}

func TestReconcile_ExpiredLease_DeletesNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{fencingEnabledLabel: ""},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	reconcile(t, r, "node-1")

	if nodeExists(r, "node-1") {
		t.Fatal("node should be deleted with expired lease")
	}
}

func TestReconcile_MaintenanceAnnotation_SkipsNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{fencingEnabledLabel: ""},
			Annotations: map[string]string{"update.node.deckhouse.io/approved": ""},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	res := reconcile(t, r, "node-1")

	if res.RequeueAfter != requeueInterval {
		t.Fatalf("expected requeue, got %v", res.RequeueAfter)
	}
	if !nodeExists(r, "node-1") {
		t.Fatal("node with maintenance annotation should not be deleted")
	}
}

func TestReconcile_NoFencingLabel_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
	}

	r := newReconciler(t, node)
	res := reconcile(t, r, "node-1")

	if res.RequeueAfter != 0 {
		t.Fatalf("expected no requeue for node without fencing label, got %v", res.RequeueAfter)
	}
	if !nodeExists(r, "node-1") {
		t.Fatal("node without fencing label should not be deleted")
	}
}

func TestReconcile_LeaseNotFound_Requeues(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{fencingEnabledLabel: ""},
		},
	}

	r := newReconciler(t, node)
	res := reconcile(t, r, "node-1")

	if res.RequeueAfter != requeueInterval {
		t.Fatalf("expected requeue when lease not found, got %v", res.RequeueAfter)
	}
	if !nodeExists(r, "node-1") {
		t.Fatal("node should not be deleted when lease is missing")
	}
}

func TestReconcile_FencingDisableAnnotation_SkipsNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{fencingEnabledLabel: ""},
			Annotations: map[string]string{"node-manager.deckhouse.io/fencing-disable": ""},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	res := reconcile(t, r, "node-1")

	if res.RequeueAfter != requeueInterval {
		t.Fatalf("expected requeue, got %v", res.RequeueAfter)
	}
	if !nodeExists(r, "node-1") {
		t.Fatal("node with fencing-disable annotation should not be deleted")
	}
}

func TestReconcile_NotifyMode_DeletesPodsButPreservesNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				fencingEnabledLabel: "",
				fencingModeLabel:    notifyMode,
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{NodeName: "node-1"},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, pod, lease)
	reconcile(t, r, "node-1")

	if !nodeExists(r, "node-1") {
		t.Fatal("node in Notify mode should NOT be deleted")
	}

	podObj := &corev1.Pod{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: "pod-1", Namespace: "default"}, podObj)
	if !errors.IsNotFound(err) {
		t.Fatal("pod should be deleted even in Notify mode")
	}
}

func TestReconcile_StaticNode_DeletesPodsButPreservesNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				fencingEnabledLabel: "",
				nodeTypeLabel:       string(v1.NodeTypeStatic),
			},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	reconcile(t, r, "node-1")

	if !nodeExists(r, "node-1") {
		t.Fatal("Static node should NOT be deleted")
	}
}

func TestReconcile_CloudStaticNode_DeletesPodsButPreservesNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				fencingEnabledLabel: "",
				nodeTypeLabel:       string(v1.NodeTypeCloudStatic),
			},
		},
	}
	renewTime := metav1.NewMicroTime(time.Now().Add(-2 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &renewTime,
		},
	}

	r := newReconciler(t, node, lease)
	reconcile(t, r, "node-1")

	if !nodeExists(r, "node-1") {
		t.Fatal("CloudStatic node should NOT be deleted")
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "nonexistent")
}
