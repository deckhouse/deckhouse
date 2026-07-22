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

package nodeusercleanup

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
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

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := deckhousev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhousev1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&deckhousev1.NodeUser{}).
		Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func doReconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func getNodeUser(t *testing.T, r *Reconciler, name string) *deckhousev1.NodeUser {
	t.Helper()
	nu := &deckhousev1.NodeUser{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, nu); err != nil {
		t.Fatalf("get nodeuser %s: %v", name, err)
	}
	return nu
}

func groupNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
		},
	}
}

func nodeUserWithErrors(name string, errs map[string]string) *deckhousev1.NodeUser {
	return &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       deckhousev1.NodeUserSpec{UID: 1100},
		Status:     deckhousev1.NodeUserStatus{Errors: errs},
	}
}

// An error keyed by a node that no longer exists is dropped.
func TestReconcile_StaleError_Cleared(t *testing.T) {
	nu := nodeUserWithErrors("nu-1", map[string]string{"gone-node": "boom"})
	r := newReconciler(t, nu)
	doReconcile(t, r, "nu-1")

	if got := getNodeUser(t, r, "nu-1").Status.Errors; len(got) != 0 {
		t.Fatalf("expected stale error cleared, got %v", got)
	}
}

// An error keyed by a still-existing group node is preserved.
func TestReconcile_LiveError_Kept(t *testing.T) {
	nu := nodeUserWithErrors("nu-1", map[string]string{"live-node": "boom"})
	r := newReconciler(t, nu, groupNode("live-node"))
	doReconcile(t, r, "nu-1")

	got := getNodeUser(t, r, "nu-1").Status.Errors
	if got["live-node"] != "boom" {
		t.Fatalf("expected live error kept, got %v", got)
	}
}

// A node without the group label is treated as non-existent, so its error is dropped.
func TestReconcile_NodeWithoutGroupLabel_Cleared(t *testing.T) {
	nu := nodeUserWithErrors("nu-1", map[string]string{"bare-node": "boom"})
	bare := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "bare-node"}}
	r := newReconciler(t, nu, bare)
	doReconcile(t, r, "nu-1")

	if got := getNodeUser(t, r, "nu-1").Status.Errors; len(got) != 0 {
		t.Fatalf("expected error for unlabeled node cleared, got %v", got)
	}
}

// Only stale keys are removed; live keys remain untouched.
func TestReconcile_Mixed_OnlyStaleCleared(t *testing.T) {
	nu := nodeUserWithErrors("nu-1", map[string]string{"live-node": "a", "gone-node": "b"})
	r := newReconciler(t, nu, groupNode("live-node"))
	doReconcile(t, r, "nu-1")

	got := getNodeUser(t, r, "nu-1").Status.Errors
	if len(got) != 1 || got["live-node"] != "a" {
		t.Fatalf("expected only live-node error to remain, got %v", got)
	}
}

// No errors means nothing to do.
func TestReconcile_NoErrors_Noop(t *testing.T) {
	nu := &deckhousev1.NodeUser{ObjectMeta: metav1.ObjectMeta{Name: "nu-1"}}
	r := newReconciler(t, nu)
	doReconcile(t, r, "nu-1")

	if got := getNodeUser(t, r, "nu-1").Status.Errors; len(got) != 0 {
		t.Fatalf("expected no errors, got %v", got)
	}
}

func TestReconcile_NodeUserNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r, "nonexistent")
}

// nodeToNodeUsers enqueues every NodeUser so a node deletion can strand no entry.
func TestNodeToNodeUsers_EnqueuesAll(t *testing.T) {
	r := newReconciler(t,
		&deckhousev1.NodeUser{ObjectMeta: metav1.ObjectMeta{Name: "nu-a"}},
		&deckhousev1.NodeUser{ObjectMeta: metav1.ObjectMeta{Name: "nu-b"}},
	)
	reqs := r.nodeToNodeUsers(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "gone"}})
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
	names := map[string]bool{reqs[0].Name: true, reqs[1].Name: true}
	if !names["nu-a"] || !names["nu-b"] {
		t.Fatalf("expected requests for nu-a and nu-b, got %v", reqs)
	}
}

// SetupWatches registers a Node watch that reacts to deletions only.
func TestSetupWatches_NodeDeleteOnly(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if len(w.watched) != 1 {
		t.Fatalf("expected 1 watched object, got %d", len(w.watched))
	}
	if _, ok := w.watched[0].(*corev1.Node); !ok {
		t.Fatalf("expected Node watch, got %T", w.watched[0])
	}
}

type captureWatcher struct {
	watched []client.Object
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *captureWatcher) Watches(obj client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
	w.watched = append(w.watched, obj)
}
func (w *captureWatcher) WatchesRawSource(_ source.Source)      {}
func (w *captureWatcher) WithEventFilter(_ predicate.Predicate) {}
