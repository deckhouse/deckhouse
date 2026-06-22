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

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

func TestListMatchingNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := deckhousev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhouse scheme: %v", err)
	}

	namespaces := []corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ns-1", Labels: map[string]string{"foo": "bar"}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ns-2", Labels: map[string]string{"foo": "bar", "env": "prod"}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ns-3", Labels: map[string]string{"foo": "baz"}},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(toObjects(namespaces)...).
		Build()

	reconciler := &ContainerdIntegrityPolicyReconciler{Client: cl, Scheme: scheme}
	policy := &deckhousev1alpha1.ContainerdIntegrityPolicy{
		Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{
			ProtectedNamespaces: deckhousev1alpha1.ProtectedNamespacesSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
		},
	}

	got, err := reconciler.listMatchingNamespaces(context.Background(), policy)
	if err != nil {
		t.Fatalf("listMatchingNamespaces() error = %v", err)
	}

	want := []string{"ns-1", "ns-2"}
	if len(got) != len(want) {
		t.Fatalf("listMatchingNamespaces() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("listMatchingNamespaces() = %v, want %v", got, want)
		}
	}
}

func toObjects(namespaces []corev1.Namespace) []client.Object {
	objects := make([]client.Object, 0, len(namespaces))
	for i := range namespaces {
		objects = append(objects, &namespaces[i])
	}
	return objects
}
