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

package spottermination

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := deckhousev1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhousev1alpha2 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func doReconcile(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
}

func instanceExists(t *testing.T, r *Reconciler, name string) bool {
	t.Helper()
	inst := &deckhousev1alpha2.Instance{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, inst)
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		return false
	}
	t.Fatalf("get instance %s: %v", name, err)
	return false
}

func node(name string, labels, annotations map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        name,
		Labels:      labels,
		Annotations: annotations,
	}}
}

func instance(name string) *deckhousev1alpha2.Instance {
	return &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

var (
	termLabel    = map[string]string{nodecommon.TerminationInProgressLabel: "true"}
	drainedAnnot = map[string]string{nodecommon.DrainedAnnotation: ""}
)

// Without the termination label the Instance is left alone.
func TestReconcile_NoTerminationLabel_KeepsInstance(t *testing.T) {
	r := newReconciler(t, node("node-1", nil, drainedAnnot), instance("node-1"))
	doReconcile(t, r, "node-1")

	if !instanceExists(t, r, "node-1") {
		t.Fatal("Instance should remain without the termination label")
	}
}

// Termination label set but node not drained yet: keep the Instance.
func TestReconcile_TerminationLabelButNotDrained_KeepsInstance(t *testing.T) {
	r := newReconciler(t, node("node-1", termLabel, nil), instance("node-1"))
	doReconcile(t, r, "node-1")

	if !instanceExists(t, r, "node-1") {
		t.Fatal("Instance should remain until the node is drained")
	}
}

// The label present but not equal to "true" does not trigger deletion.
func TestReconcile_TerminationLabelNotTrue_KeepsInstance(t *testing.T) {
	r := newReconciler(t,
		node("node-1", map[string]string{nodecommon.TerminationInProgressLabel: "false"}, drainedAnnot),
		instance("node-1"))
	doReconcile(t, r, "node-1")

	if !instanceExists(t, r, "node-1") {
		t.Fatal("Instance should remain when the label value is not \"true\"")
	}
}

// Label + drained annotation: the Instance is deleted.
func TestReconcile_TerminationLabelAndDrained_DeletesInstance(t *testing.T) {
	r := newReconciler(t, node("node-1", termLabel, drainedAnnot), instance("node-1"))
	doReconcile(t, r, "node-1")

	if instanceExists(t, r, "node-1") {
		t.Fatal("Instance should be deleted for a drained spot-terminated node")
	}
}

// A drained annotation with a non-empty source value still triggers deletion.
func TestReconcile_DrainedWithSource_DeletesInstance(t *testing.T) {
	r := newReconciler(t,
		node("node-1", termLabel, map[string]string{nodecommon.DrainedAnnotation: "bashible"}),
		instance("node-1"))
	doReconcile(t, r, "node-1")

	if instanceExists(t, r, "node-1") {
		t.Fatal("Instance should be deleted regardless of the drained source value")
	}
}

// Deletion is idempotent: no error when the Instance is already gone.
func TestReconcile_InstanceAlreadyGone_NoError(t *testing.T) {
	r := newReconciler(t, node("node-1", termLabel, drainedAnnot))
	doReconcile(t, r, "node-1")
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r, "nonexistent")
}
