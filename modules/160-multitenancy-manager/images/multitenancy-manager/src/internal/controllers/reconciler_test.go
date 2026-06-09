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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/naming"
)

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper(nil)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	return m
}

func buildReconciler(t *testing.T, objs ...client.Object) *ProjectReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, storagev1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha1.AvailableClusterResource{}, &v1alpha1.ClusterResourceGrant{}, &v1alpha1.ClusterResourceGrantPolicy{}, &v1alpha1.GrantableClusterResourceDefinition{}).
		Build()
	return &ProjectReconciler{Client: cl, Mapper: testMapper(), Factory: jsonpath.NewWithCache()}
}

func TestReconcile_CatalogAndQuota(t *testing.T) {
	labels := map[string]string{naming.ProjectLabel: "team-a", "env": "prod"}
	control := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: labels}}
	workload := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: labels}}

	reg := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
			UsageReferences: []v1alpha1.UsageReference{{
				Rule:       v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
				FieldPath:  "$.spec.storageClassName",
				Countable:  true,
				Quantities: []v1alpha1.QuantityMeasure{{Name: "requests.storage", FieldPath: "$.spec.resources.requests.storage"}},
			}},
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
	pool := &v1alpha1.ClusterResourceGrant{
		ObjectMeta: metav1.ObjectMeta{Name: "objects", Namespace: "team-a"},
		Spec: v1alpha1.ClusterResourceGrantSpec{Objects: map[string]map[string]map[string]resource.Quantity{
			"storageclasses": {"*": {"requests.storage": resource.MustParse("100Gi")}},
		}},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "team-a-be"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: ptrTo("standard"),
			Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}},
		},
	}
	r := buildReconciler(t, control, workload, reg, grant, sc, pool, pvc)
	ctx := context.Background()

	for _, ns := range []string{"team-a", "team-a-be"} {
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ns}}); err != nil {
			t.Fatalf("reconcile %s: %v", ns, err)
		}
	}

	// Catalog: AvailableClusterResource in the workload namespace lists "standard" as default.
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

	// Pool status: project total usage for standard/requests.storage = 10Gi.
	gotPool := &v1alpha1.ClusterResourceGrant{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a", Name: "objects"}, gotPool); err != nil {
		t.Fatal(err)
	}
	if !hasMeasure(gotPool.Status.Objects, "storageclasses", "standard", "requests.storage", "10Gi") {
		t.Fatalf("pool status missing usage: %+v", gotPool.Status.Objects)
	}

	// Rendered ClusterResourceGrant in the workload namespace carries this-namespace used + project totals.
	rendered := &v1alpha1.ClusterResourceGrant{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a-be", Name: "objects"}, rendered); err != nil {
		t.Fatalf("get rendered ClusterResourceGrant: %v", err)
	}
	if !hasMeasure(rendered.Status.Objects, "storageclasses", "standard", "requests.storage", "10Gi") {
		t.Fatalf("rendered status missing usage: %+v", rendered.Status.Objects)
	}
}

// TestReconcile_NonProjectNamespace verifies that a namespace without the project label never gets a
// catalog — even when a registration's defaultAvailability is All — and that a stale AvailableClusterResource
// left in such a namespace is cleaned up.
func TestReconcile_NonProjectNamespace(t *testing.T) {
	// default namespace: no project label.
	plain := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	reg := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			UsageReferences: []v1alpha1.UsageReference{{
				Rule:      v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
				FieldPath: "$.spec.storageClassName",
			}},
		},
	}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	// A stale catalog object that must be cleaned up.
	stale := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: "storageclasses", Namespace: "default"}}
	r := buildReconciler(t, plain, reg, sc, stale)
	ctx := context.Background()

	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "default"}}); err != nil {
		t.Fatalf("reconcile default: %v", err)
	}

	ar := &v1alpha1.AvailableClusterResource{}
	err := r.Get(ctx, types.NamespacedName{Namespace: "default", Name: "storageclasses"}, ar)
	if err == nil {
		t.Fatalf("AvailableClusterResource must not exist in a non-project namespace, got %+v", ar)
	}
	if !k8serrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func hasMeasure(list []v1alpha1.ClusterResourceGrantMeasureStatus, res, name, measure, used string) bool {
	for _, m := range list {
		if m.Resource == res && m.Name == name && m.Measure == measure && m.Used.String() == used {
			return true
		}
	}
	return false
}

func ptrTo[T any](v T) *T { return &v }
