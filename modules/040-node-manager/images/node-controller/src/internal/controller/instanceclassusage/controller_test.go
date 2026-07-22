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

package instanceclassusage

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

const testKind = "YandexInstanceClass"

func icGVK(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: instanceClassGroup, Version: instanceClassVersion, Kind: kind}
}

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := deckhousev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhousev1 scheme: %v", err)
	}
	scheme.AddKnownTypeWithName(icGVK(testKind), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(icGVK(testKind+"List"), &unstructured.UnstructuredList{})
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func doReconcile(t *testing.T, r *Reconciler) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func providerSecret(kind string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName},
		Data:       map[string][]byte{instanceClassKindKey: []byte(kind)},
	}
}

func cloudEphemeralNG(name, classKind, className string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				ClassReference: deckhousev1.ClassReference{Kind: classKind, Name: className},
			},
		},
	}
}

func instanceClass(kind, name string, consumers []string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(icGVK(kind))
	u.SetName(name)
	if consumers != nil {
		_ = unstructured.SetNestedStringSlice(u.Object, consumers, "status", consumersField)
	}
	return u
}

func getConsumers(t *testing.T, r *Reconciler, kind, name string) []string {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(icGVK(kind))
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, u); err != nil {
		t.Fatalf("get instanceclass %s: %v", name, err)
	}
	got, _, _ := unstructured.NestedStringSlice(u.Object, "status", consumersField)
	return got
}

// A CloudEphemeral NodeGroup referencing a class of the active kind records that
// NodeGroup as a consumer of the class.
func TestReconcile_SingleConsumer(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("ng-a", testKind, "worker"),
		instanceClass(testKind, "worker", nil),
	)
	doReconcile(t, r)

	if got := getConsumers(t, r, testKind, "worker"); len(got) != 1 || got[0] != "ng-a" {
		t.Fatalf("expected [ng-a], got %v", got)
	}
}

// Multiple NodeGroups on the same class are recorded as a sorted list.
func TestReconcile_MultipleConsumersSorted(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("zeta", testKind, "worker"),
		cloudEphemeralNG("alpha", testKind, "worker"),
		instanceClass(testKind, "worker", nil),
	)
	doReconcile(t, r)

	got := getConsumers(t, r, testKind, "worker")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("expected [alpha zeta], got %v", got)
	}
}

// A class nobody references keeps an empty consumer list.
func TestReconcile_UnusedClassEmpty(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("ng-a", testKind, "worker"),
		instanceClass(testKind, "worker", nil),
		instanceClass(testKind, "spare", nil),
	)
	doReconcile(t, r)

	if got := getConsumers(t, r, testKind, "spare"); len(got) != 0 {
		t.Fatalf("expected empty for unused class, got %v", got)
	}
}

// A NodeGroup referencing a class of a different kind is ignored.
func TestReconcile_DifferentKindIgnored(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("ng-a", "AWSInstanceClass", "worker"),
		instanceClass(testKind, "worker", nil),
	)
	doReconcile(t, r)

	if got := getConsumers(t, r, testKind, "worker"); len(got) != 0 {
		t.Fatalf("expected empty (wrong kind), got %v", got)
	}
}

// Non-CloudEphemeral NodeGroups never consume a cloud InstanceClass.
func TestReconcile_NonCloudEphemeralIgnored(t *testing.T) {
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "static-ng"},
		Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic},
	}
	r := newReconciler(t,
		providerSecret(testKind),
		ng,
		instanceClass(testKind, "worker", nil),
	)
	doReconcile(t, r)

	if got := getConsumers(t, r, testKind, "worker"); len(got) != 0 {
		t.Fatalf("expected empty (static ng), got %v", got)
	}
}

// A stale consumer list is corrected to the current set.
func TestReconcile_StaleConsumersCorrected(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("ng-a", testKind, "worker"),
		instanceClass(testKind, "worker", []string{"gone", "ng-a"}),
	)
	doReconcile(t, r)

	got := getConsumers(t, r, testKind, "worker")
	if len(got) != 1 || got[0] != "ng-a" {
		t.Fatalf("expected [ng-a] after correction, got %v", got)
	}
}

// A class whose consumers are already correct is left untouched (idempotent).
func TestReconcile_AlreadyCorrect_Idempotent(t *testing.T) {
	r := newReconciler(t,
		providerSecret(testKind),
		cloudEphemeralNG("ng-a", testKind, "worker"),
		instanceClass(testKind, "worker", []string{"ng-a"}),
	)
	doReconcile(t, r)

	if got := getConsumers(t, r, testKind, "worker"); len(got) != 1 || got[0] != "ng-a" {
		t.Fatalf("expected [ng-a], got %v", got)
	}
}

// Without the cloud-provider Secret there is no active kind, so reconcile is a no-op.
func TestReconcile_NoSecret_NoOp(t *testing.T) {
	r := newReconciler(t,
		cloudEphemeralNG("ng-a", testKind, "worker"),
		instanceClass(testKind, "worker", []string{"stale"}),
	)
	doReconcile(t, r)

	// Nothing is listed/patched because kindInUse is empty; stale value survives.
	if got := getConsumers(t, r, testKind, "worker"); len(got) != 1 || got[0] != "stale" {
		t.Fatalf("expected untouched [stale], got %v", got)
	}
}

func TestSlicesEqual(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{nil, []string{}, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
	}
	for _, c := range cases {
		if got := slicesEqual(c.a, c.b); got != c.want {
			t.Fatalf("slicesEqual(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
