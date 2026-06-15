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

package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
	"controller/internal/naming"
)

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "storage.k8s.io", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	return m
}

func buildClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, storagev1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(
			&v1alpha1.AvailableClusterResource{},
			&v1alpha1.GrantableClusterResourceDefinition{},
			&v1alpha1.GrantableClusterResourceReference{},
		).
		Build()
}

func TestReconcile_Catalog(t *testing.T) {
	labels := map[string]string{naming.ProjectLabel: "team-a", "env": "prod"}
	control := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: labels}}
	workload := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: labels}}

	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
		},
	}
	grant := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}, Default: "standard"}},
		},
	}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	r := &ProjectReconciler{Client: buildClient(t, control, workload, def, grant, sc), Mapper: testMapper()}
	ctx := context.Background()

	for _, ns := range []string{"team-a", "team-a-be"} {
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ns}}); err != nil {
			t.Fatalf("reconcile %s: %v", ns, err)
		}
	}

	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a-be", Name: "storageclasses"}, ar); err != nil {
		t.Fatalf("get AvailableClusterResource: %v", err)
	}
	if len(ar.Status.Available) != 1 || ar.Status.Available[0].Name != "standard" || !ar.Status.Available[0].Default {
		t.Fatalf("unexpected catalog: %+v", ar.Status.Available)
	}
	if ar.Labels[naming.ProjectLabel] != "team-a" || ar.Labels[naming.ModuleLabel] != naming.ModuleValue {
		t.Fatalf("missing managed labels: %v", ar.Labels)
	}
}

// TestReconcile_NonProjectNamespace verifies that a namespace without the project label never gets a
// catalog and that a stale AvailableClusterResource left in such a namespace is cleaned up.
func TestReconcile_NonProjectNamespace(t *testing.T) {
	plain := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
		},
	}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	stale := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: "storageclasses", Namespace: "default"}}
	r := &ProjectReconciler{Client: buildClient(t, plain, def, sc, stale), Mapper: testMapper()}
	ctx := context.Background()

	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "default"}}); err != nil {
		t.Fatalf("reconcile default: %v", err)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "default", Name: "storageclasses"}, ar); !k8serrors.IsNotFound(err) {
		t.Fatalf("AvailableClusterResource must be cleaned up in a non-project namespace, got err=%v", err)
	}
}

func TestBindingStatus(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"}}
	boundRef := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "sc-pvc"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "storageclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.storageClassName"}},
		},
	}
	danglingRef := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "ghost"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "does-not-exist",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.x"}},
		},
	}
	cl := buildClient(t, def, boundRef, danglingRef)
	ctx := context.Background()

	refRec := &ReferenceReconciler{Client: cl}
	defRec := &DefinitionReconciler{Client: cl}

	// Reference binding status.
	for _, n := range []string{"sc-pvc", "ghost"} {
		if _, err := refRec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: n}}); err != nil {
			t.Fatalf("reference reconcile %s: %v", n, err)
		}
	}
	got := &v1alpha1.GrantableClusterResourceReference{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "sc-pvc"}, got); err != nil || !got.Status.Bound {
		t.Fatalf("sc-pvc must be bound, bound=%v err=%v", got.Status.Bound, err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "ghost"}, got); err != nil || got.Status.Bound {
		t.Fatalf("ghost must be unbound, bound=%v err=%v", got.Status.Bound, err)
	}

	// Definition reverse index.
	if _, err := defRec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "storageclasses"}}); err != nil {
		t.Fatalf("definition reconcile: %v", err)
	}
	gotDef := &v1alpha1.GrantableClusterResourceDefinition{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "storageclasses"}, gotDef); err != nil {
		t.Fatal(err)
	}
	if gotDef.Status.ReferenceCount != 1 || len(gotDef.Status.References) != 1 || gotDef.Status.References[0].Name != "sc-pvc" {
		t.Fatalf("definition references = %+v (count %d)", gotDef.Status.References, gotDef.Status.ReferenceCount)
	}
}
