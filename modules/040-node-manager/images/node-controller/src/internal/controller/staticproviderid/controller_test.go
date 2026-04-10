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

package staticproviderid

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

func TestReconcile_StaticNodeWithoutProviderID_SetsIt(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "static-1",
			Labels: map[string]string{nodeTypeLabel: nodeTypeStatic},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "static-1")

	updated := getNode(t, r, "static-1")
	if updated.Spec.ProviderID != staticProviderIDValue {
		t.Fatalf("expected providerID %q, got %q", staticProviderIDValue, updated.Spec.ProviderID)
	}
}

func TestReconcile_StaticNodeWithProviderID_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "static-1",
			Labels: map[string]string{nodeTypeLabel: nodeTypeStatic},
		},
		Spec: corev1.NodeSpec{ProviderID: "aws://existing"},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "static-1")

	updated := getNode(t, r, "static-1")
	if updated.Spec.ProviderID != "aws://existing" {
		t.Fatalf("expected providerID unchanged, got %q", updated.Spec.ProviderID)
	}
}

func TestReconcile_NonStaticNode_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cloud-1",
			Labels: map[string]string{nodeTypeLabel: "CloudEphemeral"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "cloud-1")

	updated := getNode(t, r, "cloud-1")
	if updated.Spec.ProviderID != "" {
		t.Fatalf("expected empty providerID for non-static node, got %q", updated.Spec.ProviderID)
	}
}

func TestReconcile_StaticNodeWithUninitializedTaint_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "static-1",
			Labels: map[string]string{nodeTypeLabel: nodeTypeStatic},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: uninitializedTaintKey, Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "static-1")

	updated := getNode(t, r, "static-1")
	if updated.Spec.ProviderID != "" {
		t.Fatalf("expected empty providerID for node with uninitialized taint, got %q", updated.Spec.ProviderID)
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "nonexistent")
}
