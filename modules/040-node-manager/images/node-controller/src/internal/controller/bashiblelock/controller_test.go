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

package bashiblelock

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appsv1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func deployment(generation, observed, replicas, updated, available int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       bashibleDeploymentName,
			Namespace:  bashibleNamespace,
			Generation: int64(generation),
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: int64(observed),
			Replicas:           replicas,
			UpdatedReplicas:    updated,
			AvailableReplicas:  available,
		},
	}
}

func contextSecret(annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        contextSecretName,
			Namespace:   bashibleNamespace,
			Annotations: annotations,
		},
	}
}

func reconcile(t *testing.T, r *Reconciler) {
	t.Helper()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: bashibleDeploymentName, Namespace: bashibleNamespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func lockValue(t *testing.T, r *Reconciler) (string, bool) {
	t.Helper()
	s := &corev1.Secret{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: contextSecretName, Namespace: bashibleNamespace}, s); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	v, ok := s.Annotations[lockAnnotation]
	return v, ok
}

// rolloutComplete: an in-progress rollout (updated < replicas) is not complete.
func TestRolloutComplete_InProgress(t *testing.T) {
	if rolloutComplete(deployment(2, 2, 2, 1, 1)) {
		t.Fatal("rollout with updatedReplicas < replicas must not be complete")
	}
}

// rolloutComplete: a settled Deployment is complete.
func TestRolloutComplete_Settled(t *testing.T) {
	if !rolloutComplete(deployment(2, 2, 2, 2, 2)) {
		t.Fatal("settled Deployment must be complete")
	}
}

// rolloutComplete: right after the spec bump the status still reflects the old
// generation; the ObservedGeneration guard must reject it.
func TestRolloutComplete_StaleStatus(t *testing.T) {
	if rolloutComplete(deployment(3, 2, 2, 2, 2)) {
		t.Fatal("status from a previous generation must not read complete")
	}
}

// rolloutComplete: all replicas updated but not yet available is not complete.
func TestRolloutComplete_NotAvailable(t *testing.T) {
	if rolloutComplete(deployment(2, 2, 2, 2, 1)) {
		t.Fatal("updated but unavailable replicas must not be complete")
	}
}

// A rollout in progress locks the context Secret and sets the metric to 1.
func TestReconcile_RolloutLocks(t *testing.T) {
	r := newReconciler(t, deployment(2, 2, 2, 1, 1), contextSecret(nil))
	reconcile(t, r)

	if v, ok := lockValue(t, r); !ok || v != "true" {
		t.Fatalf("expected lock annotation true, got %q (present=%v)", v, ok)
	}
	if got := testutil.ToFloat64(bashibleLocked); got != 1 {
		t.Fatalf("expected metric 1, got %v", got)
	}
}

// A completed rollout removes the lock annotation and sets the metric to 0.
func TestReconcile_CompleteUnlocks(t *testing.T) {
	r := newReconciler(t, deployment(2, 2, 2, 2, 2), contextSecret(map[string]string{lockAnnotation: "true"}))
	reconcile(t, r)

	if v, ok := lockValue(t, r); ok {
		t.Fatalf("expected lock annotation removed, still present=%q", v)
	}
	if got := testutil.ToFloat64(bashibleLocked); got != 0 {
		t.Fatalf("expected metric 0, got %v", got)
	}
}

// A missing context Secret is tolerated (no error), matching WithIgnoreMissingObject.
func TestReconcile_MissingSecret_NoError(t *testing.T) {
	r := newReconciler(t, deployment(2, 2, 2, 1, 1))
	reconcile(t, r)
	if got := testutil.ToFloat64(bashibleLocked); got != 1 {
		t.Fatalf("expected metric 1 even without the Secret, got %v", got)
	}
}

// A request for a Deployment other than bashible-apiserver is a no-op.
func TestReconcile_OtherDeployment_Noop(t *testing.T) {
	r := newReconciler(t, contextSecret(map[string]string{lockAnnotation: "true"}))
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "other", Namespace: bashibleNamespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if v, ok := lockValue(t, r); !ok || v != "true" {
		t.Fatalf("unrelated Deployment must not touch the lock, got %q (present=%v)", v, ok)
	}
}

// The event filter passes only the bashible-apiserver Deployment.
func TestSetupWatches_Predicate(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if w.predicate == nil {
		t.Fatal("expected an event filter predicate")
	}
	pass := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: bashibleDeploymentName, Namespace: bashibleNamespace}}
	if !w.predicate.Create(createEvent(pass)) {
		t.Fatal("predicate must pass the bashible-apiserver Deployment")
	}
	wrongName := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: bashibleNamespace}}
	if w.predicate.Create(createEvent(wrongName)) {
		t.Fatal("predicate must drop other Deployments")
	}
	wrongNs := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: bashibleDeploymentName, Namespace: "default"}}
	if w.predicate.Create(createEvent(wrongNs)) {
		t.Fatal("predicate must drop Deployments in other namespaces")
	}
}

func createEvent(obj client.Object) event.CreateEvent { return event.CreateEvent{Object: obj} }

type captureWatcher struct {
	predicate predicate.Predicate
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption)                              {}
func (w *captureWatcher) Watches(_ client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {}
func (w *captureWatcher) WatchesRawSource(_ source.Source)      {}
func (w *captureWatcher) WithEventFilter(p predicate.Predicate) { w.predicate = p }
