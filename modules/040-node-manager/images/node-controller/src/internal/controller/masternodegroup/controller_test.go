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

package masternodegroup

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
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
		Build()
	return &Reconciler{
		Base:      register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)},
		apiReader: cl,
	}
}

func clusterConfigSecret(clusterType string) *corev1.Secret {
	yaml := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterType: " + clusterType + "\n"
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace},
		Data:       map[string][]byte{clusterConfigKey: []byte(yaml)},
	}
}

func doReconcile(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
}

func getMaster(t *testing.T, r *Reconciler) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(deckhousev1.GroupVersion.WithKind("NodeGroup"))
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: masterNodeGroupName}, u); err != nil {
		t.Fatalf("get master NodeGroup: %v", err)
	}
	return u
}

func nodeType(t *testing.T, u *unstructured.Unstructured) string {
	t.Helper()
	v, _, _ := unstructured.NestedString(u.Object, "spec", "nodeType")
	return v
}

// Fresh cloud cluster: master NodeGroup is created as CloudPermanent.
func TestReconcile_FreshCloud_Creates(t *testing.T) {
	r := newReconciler(t, clusterConfigSecret("Cloud"))
	doReconcile(t, r, masterNodeGroupName)

	if got := nodeType(t, getMaster(t, r)); got != "CloudPermanent" {
		t.Fatalf("expected nodeType CloudPermanent, got %q", got)
	}
}

// Static cluster: master NodeGroup nodeType is Static.
func TestReconcile_Static_Creates(t *testing.T) {
	r := newReconciler(t, clusterConfigSecret("Static"))
	doReconcile(t, r, masterNodeGroupName)

	if got := nodeType(t, getMaster(t, r)); got != "Static" {
		t.Fatalf("expected nodeType Static, got %q", got)
	}
}

// An existing master NodeGroup is never patched (user changes preserved).
func TestReconcile_Existing_NotPatched(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]interface{}{"name": masterNodeGroupName},
		"spec":       map[string]interface{}{"nodeType": "Static"},
	}}
	r := newReconciler(t, existing, clusterConfigSecret("Cloud"))
	doReconcile(t, r, masterNodeGroupName)

	if got := nodeType(t, getMaster(t, r)); got != "Static" {
		t.Fatalf("existing master must be preserved, got nodeType %q", got)
	}
}

// A request for any other NodeGroup name is a no-op.
func TestReconcile_OtherName_Noop(t *testing.T) {
	r := newReconciler(t, clusterConfigSecret("Cloud"))
	doReconcile(t, r, "worker")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(deckhousev1.GroupVersion.WithKind("NodeGroup"))
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: masterNodeGroupName}, u); err == nil {
		t.Fatalf("master NodeGroup must not be created for a non-master request")
	}
}

// SetupWatches registers the master-only predicate and a startup raw source.
func TestSetupWatches_PredicateAndStartupSource(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if w.predicate == nil {
		t.Fatalf("expected an event filter predicate")
	}
	if !w.predicate.Create(createEvent(masterNodeGroupName)) {
		t.Fatalf("predicate must pass the master NodeGroup")
	}
	if w.predicate.Create(createEvent("worker")) {
		t.Fatalf("predicate must drop non-master NodeGroups")
	}
	if w.rawSources != 1 {
		t.Fatalf("expected 1 startup raw source, got %d", w.rawSources)
	}
}

func createEvent(name string) event.CreateEvent {
	return event.CreateEvent{Object: &deckhousev1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: name}}}
}

type captureWatcher struct {
	predicate  predicate.Predicate
	rawSources int
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption)                        {}
func (w *captureWatcher) Watches(_ client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {}
func (w *captureWatcher) WatchesRawSource(_ source.Source)      { w.rawSources++ }
func (w *captureWatcher) WithEventFilter(p predicate.Predicate) { w.predicate = p }
