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

package csitaint

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add storagev1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func getNode(t *testing.T, r *Reconciler, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node
}

func doReconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func nodeWithTaint(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{{
			Key:    csiNotBootstrappedTaintKey,
			Effect: corev1.TaintEffectNoSchedule,
		}}},
	}
}

func csiNodeWithDrivers(name string, drivers ...string) *storagev1.CSINode {
	specDrivers := make([]storagev1.CSINodeDriver, 0, len(drivers))
	for _, d := range drivers {
		specDrivers = append(specDrivers, storagev1.CSINodeDriver{Name: d, NodeID: name})
	}
	return &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       storagev1.CSINodeSpec{Drivers: specDrivers},
	}
}

// A node without the taint requires no work regardless of CSINode state.
func TestReconcile_NoTaint_Noop(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	r := newReconciler(t, node)
	doReconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if len(updated.Spec.Taints) != 0 {
		t.Fatalf("expected no taints, got %v", updated.Spec.Taints)
	}
}

// The taint stays while the node is still bootstrapping (no CSINode yet).
func TestReconcile_TaintButNoCSINode_KeepsTaint(t *testing.T) {
	r := newReconciler(t, nodeWithTaint("node-1"))
	doReconcile(t, r, "node-1")

	if !hasCSITaint(getNode(t, r, "node-1")) {
		t.Fatal("taint should remain while CSINode is absent")
	}
}

// The taint stays while the CSINode exists but no driver has registered.
func TestReconcile_CSINodeWithoutDrivers_KeepsTaint(t *testing.T) {
	r := newReconciler(t, nodeWithTaint("node-1"), csiNodeWithDrivers("node-1"))
	doReconcile(t, r, "node-1")

	if !hasCSITaint(getNode(t, r, "node-1")) {
		t.Fatal("taint should remain while no CSI driver is registered")
	}
}

// Once a driver is registered the taint is removed.
func TestReconcile_CSINodeWithDrivers_RemovesTaint(t *testing.T) {
	r := newReconciler(t, nodeWithTaint("node-1"), csiNodeWithDrivers("node-1", "csi.dvp.deckhouse.io"))
	doReconcile(t, r, "node-1")

	if hasCSITaint(getNode(t, r, "node-1")) {
		t.Fatal("taint should be removed once a CSI driver is registered")
	}
}

// Only the csi-not-bootstrapped taint is stripped; other taints are preserved.
func TestReconcile_RemovesOnlyCSITaint(t *testing.T) {
	node := nodeWithTaint("node-1")
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    "dedicated",
		Value:  "gpu",
		Effect: corev1.TaintEffectNoSchedule,
	})
	r := newReconciler(t, node, csiNodeWithDrivers("node-1", "csi.dvp.deckhouse.io"))
	doReconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if hasCSITaint(updated) {
		t.Fatal("csi taint should be removed")
	}
	if len(updated.Spec.Taints) != 1 || updated.Spec.Taints[0].Key != "dedicated" {
		t.Fatalf("expected only the dedicated taint to remain, got %v", updated.Spec.Taints)
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r, "nonexistent")
}

// csiNodeToNode maps a CSINode to a reconcile request for the node of the same name.
func TestCSINodeToNode_MapsByName(t *testing.T) {
	csiNode := &storagev1.CSINode{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	reqs := csiNodeToNode(context.Background(), csiNode)

	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Name != "node-1" || reqs[0].Namespace != "" {
		t.Fatalf("expected request for node-1, got %v", reqs[0].NamespacedName)
	}
}

// SetupWatches registers a secondary watch on CSINode.
func TestSetupWatches_WatchesCSINode(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if len(w.watched) != 1 {
		t.Fatalf("expected 1 watched object, got %d", len(w.watched))
	}
	if _, ok := w.watched[0].(*storagev1.CSINode); !ok {
		t.Fatalf("expected CSINode watch, got %T", w.watched[0])
	}
}

func TestHasCSITaint(t *testing.T) {
	if hasCSITaint(&corev1.Node{}) {
		t.Fatal("empty node should not have the taint")
	}
	if !hasCSITaint(nodeWithTaint("n")) {
		t.Fatal("tainted node should report the taint")
	}
}

func TestRemoveCSITaint(t *testing.T) {
	taints := []corev1.Taint{
		{Key: csiNotBootstrappedTaintKey, Effect: corev1.TaintEffectNoSchedule},
		{Key: "other", Effect: corev1.TaintEffectNoSchedule},
	}
	got := removeCSITaint(taints)
	if len(got) != 1 || got[0].Key != "other" {
		t.Fatalf("expected only 'other' taint to remain, got %v", got)
	}
}

type captureWatcher struct {
	watched []client.Object
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *captureWatcher) Watches(obj client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
	w.watched = append(w.watched, obj)
}
func (w *captureWatcher) WatchesRawSource(_ source.Source)          {}
func (w *captureWatcher) WithEventFilter(_ predicate.Predicate)     {}
