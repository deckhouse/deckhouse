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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/jsonpath"
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
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, storagev1.AddToScheme, rbacv1.AddToScheme, v1alpha1.AddToScheme, v1alpha3.AddToScheme} {
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
			&v1alpha1.ClusterResourceGrantPolicy{},
		).
		Build()
}

// buildClientWithStatus is buildClient for tests that write a policy status.
func buildClientWithStatus(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return buildClient(t, objs...)
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
	// Here everything goes, labelled as the module's own or not: no catalog belongs in this namespace.
	staleManaged := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: "ingressclasses", Namespace: "default", Labels: naming.ManagedLabels("team-a")}}
	r := &ProjectReconciler{Client: buildClient(t, plain, def, sc, stale, staleManaged), Mapper: testMapper()}
	ctx := context.Background()

	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "default"}}); err != nil {
		t.Fatalf("reconcile default: %v", err)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	for _, name := range []string{"storageclasses", "ingressclasses"} {
		if err := r.Get(ctx, types.NamespacedName{Namespace: "default", Name: name}, ar); !k8serrors.IsNotFound(err) {
			t.Fatalf("AvailableClusterResource %s must be cleaned up in a non-project namespace, got err=%v", name, err)
		}
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

	refRec := &ReferenceReconciler{Client: cl, Factory: jsonpath.NewWithCache()}
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

func TestFieldPathsValidCondition(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "priorityclasses"}}
	rule := v1alpha1.UsageRule{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs", "cronjobs"}}
	valid := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "valid", Generation: 1},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "priorityclasses",
			Rule:                         rule,
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.template.spec.priorityClassName"}},
		},
	}
	// Stored past the webhook: a path that does not compile and a hole for cronjobs.
	invalid := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "invalid", Generation: 1},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "priorityclasses",
			Rule:                         rule,
			FieldPaths:                   []v1alpha1.FieldPath{{Resources: []string{"jobs"}, Path: "$.spec.foo-bar"}},
		},
	}
	cl := buildClient(t, def, valid, invalid)
	ctx := context.Background()
	rec := &ReferenceReconciler{Client: cl, Factory: jsonpath.NewWithCache()}

	condition := func(name string) *metav1.Condition {
		t.Helper()
		if _, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
			t.Fatalf("reconcile %s: %v", name, err)
		}
		got := &v1alpha1.GrantableClusterResourceReference{}
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, got); err != nil {
			t.Fatal(err)
		}
		c := meta.FindStatusCondition(got.Status.Conditions, "FieldPathsValid")
		if c == nil {
			t.Fatalf("%s has no FieldPathsValid condition: %+v", name, got.Status.Conditions)
		}
		if c.ObservedGeneration != got.Generation {
			t.Fatalf("%s: observedGeneration %d, want %d", name, c.ObservedGeneration, got.Generation)
		}
		return c
	}

	if c := condition("valid"); c.Status != metav1.ConditionTrue || c.Reason != "Valid" {
		t.Fatalf("valid: %+v", c)
	}

	c := condition("invalid")
	if c.Status != metav1.ConditionFalse || c.Reason != "InvalidFieldPaths" {
		t.Fatalf("invalid: %+v", c)
	}
	for _, want := range []string{
		`'spec.fieldPaths[0].path' "$.spec.foo-bar" is not a valid RFC 9535 JSONPath`,
		`(apiGroup "batch", apiVersion "v1", resource "cronjobs")`,
	} {
		if !strings.Contains(c.Message, want) {
			t.Fatalf("invalid: message %q lacks %q", c.Message, want)
		}
	}

	// Fixing the spec turns the condition True.
	fixed := &v1alpha1.GrantableClusterResourceReference{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "invalid"}, fixed); err != nil {
		t.Fatal(err)
	}
	fixed.Spec.FieldPaths = []v1alpha1.FieldPath{{Path: "$.spec.template.spec.priorityClassName"}}
	if err := cl.Update(ctx, fixed); err != nil {
		t.Fatal(err)
	}
	if c := condition("invalid"); c.Status != metav1.ConditionTrue || c.Reason != "Valid" {
		t.Fatalf("fixed: %+v", c)
	}
}

// TestReconcile_CatalogOfADeletedRegistrationIsRemoved: a catalog whose GrantableClusterResourceDefinition
// is gone used to stay in every project namespace forever -- the tenant kept reading a stale list and a
// stale default, and since the catalog is read-only to everyone but the controller, nobody could remove
// it by hand either.
func TestReconcile_CatalogOfADeletedRegistrationIsRemoved(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	// The catalog of a registration that no longer exists, rendered by an earlier pass.
	orphan := &v1alpha1.AvailableClusterResource{
		ObjectMeta: metav1.ObjectMeta{Name: "ingressclasses", Namespace: "team-a", Labels: naming.ManagedLabels("team-a")},
		Status: v1alpha1.AvailableClusterResourceStatus{
			GrantedResourceKind: "IngressClass",
			Available:           []v1alpha1.AvailableObject{{Name: "nginx", Default: true}},
			Default:             "nginx",
			AvailableCount:      1,
		},
	}
	// The catalog of the registration that is still there; the sweep must not take it along.
	live := &v1alpha1.AvailableClusterResource{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses", Namespace: "team-a", Labels: naming.ManagedLabels("team-a")},
		Status: v1alpha1.AvailableClusterResourceStatus{
			GrantedResourceKind: "StorageClass",
			Available:           []v1alpha1.AvailableObject{{Name: "standard"}},
			AvailableCount:      1,
		},
	}
	// An object of the same kind without the ownership labels: not this controller's to delete.
	foreign := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "team-a"}}

	r := &ProjectReconciler{Client: buildClient(t, ns, storageClassDefinition(v1alpha1.AvailabilityAll), sc, orphan, live, foreign), Mapper: testMapper()}
	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
		t.Fatal(err)
	}

	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a", Name: "ingressclasses"}, ar); !k8serrors.IsNotFound(err) {
		t.Fatalf("the catalog of a deleted registration must be removed, err=%v", err)
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a", Name: "storageclasses"}, ar); err != nil {
		t.Fatalf("the catalog of a live registration must stay: %v", err)
	}
	if ar.Status.GrantedResourceKind != "StorageClass" || ar.Status.AvailableCount != 1 ||
		len(ar.Status.Available) != 1 || ar.Status.Available[0].Name != "standard" {
		t.Fatalf("the live catalog lost its status: %+v", ar.Status)
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: "team-a", Name: "foreign"}, ar); err != nil {
		t.Fatalf("an AvailableClusterResource without the ownership labels must be left alone: %v", err)
	}
}

// catalogOf is an AvailableClusterResource in team-a with the given labels.
func catalogOf(name string, labels map[string]string) *v1alpha1.AvailableClusterResource {
	return &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "team-a", Labels: labels}}
}

// reconcileTeamA runs one pass over the team-a project namespace; the error is returned, not fatal.
func reconcileTeamA(t *testing.T, objs ...client.Object) (*ProjectReconciler, error) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	r := &ProjectReconciler{Client: buildClient(t, append([]client.Object{ns, sc}, objs...)...), Mapper: testMapper()}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	return r, err
}

func assertCatalog(t *testing.T, r *ProjectReconciler, name string, wantPresent bool) {
	t.Helper()
	err := r.Get(context.Background(), types.NamespacedName{Namespace: "team-a", Name: name}, &v1alpha1.AvailableClusterResource{})
	switch {
	case wantPresent && err != nil:
		t.Fatalf("AvailableClusterResource %s must be present: %v", name, err)
	case !wantPresent && !k8serrors.IsNotFound(err):
		t.Fatalf("AvailableClusterResource %s must be removed, err=%v", name, err)
	}
}

// TestReconcile_OrphanOfAnOlderHeritageIsRemoved: catalogs of earlier releases carry heritage=deckhouse;
// the sweep selects by the module label alone, so they are swept too, while an object of another
// module is left alone.
func TestReconcile_OrphanOfAnOlderHeritageIsRemoved(t *testing.T) {
	oldGen := catalogOf("ingressclasses", map[string]string{naming.HeritageLabel: "deckhouse", naming.ModuleLabel: naming.ModuleValue})
	other := catalogOf("other", map[string]string{naming.ModuleLabel: "other"})
	r, err := reconcileTeamA(t, storageClassDefinition(v1alpha1.AvailabilityAll), oldGen, other)
	if err != nil {
		t.Fatal(err)
	}
	assertCatalog(t, r, "ingressclasses", false)
	assertCatalog(t, r, "other", true)
	assertCatalog(t, r, "storageclasses", true)
}

// TestReconcile_NoRegistrationsRemovesAllModuleCatalogs: with no registration left, every catalog of
// the module goes, a foreign one stays.
func TestReconcile_NoRegistrationsRemovesAllModuleCatalogs(t *testing.T) {
	r, err := reconcileTeamA(t,
		catalogOf("storageclasses", naming.ManagedLabels("team-a")),
		catalogOf("ingressclasses", map[string]string{naming.HeritageLabel: "deckhouse", naming.ModuleLabel: naming.ModuleValue}),
		catalogOf("other", map[string]string{naming.ModuleLabel: "other"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	assertCatalog(t, r, "storageclasses", false)
	assertCatalog(t, r, "ingressclasses", false)
	assertCatalog(t, r, "other", true)
}

// TestReconcile_BrokenRegistrationDoesNotBlockTheOthers: a registration whose kind the mapper does not
// know fails, but the other registrations are still upserted, the orphan is still swept, the failed
// registration keeps its catalog, and the error is reported.
func TestReconcile_BrokenRegistrationDoesNotBlockTheOthers(t *testing.T) {
	// Named to sort before storageclasses, so an early return would skip its upsert.
	broken := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "aaa-broken"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "example.com", Kind: "Unknown"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
		},
	}
	r, err := reconcileTeamA(t, broken, storageClassDefinition(v1alpha1.AvailabilityAll),
		catalogOf("aaa-broken", naming.ManagedLabels("team-a")),
		catalogOf("ingressclasses", naming.ManagedLabels("team-a")),
	)
	if err == nil {
		t.Fatal("reconcile must report the broken registration")
	}
	assertCatalog(t, r, "ingressclasses", false)
	assertCatalog(t, r, "storageclasses", true)
	assertCatalog(t, r, "aaa-broken", true)
}

// TestReconcile_CatalogOfADeletingRegistrationIsRemoved: a registration held by a finalizer is gone as
// far as the catalog is concerned; its catalog is removed in the same pass.
func TestReconcile_CatalogOfADeletingRegistrationIsRemoved(t *testing.T) {
	deleting := storageClassDefinition(v1alpha1.AvailabilityAll)
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{"example.com/hold"}
	r, err := reconcileTeamA(t, deleting, catalogOf("storageclasses", naming.ManagedLabels("team-a")))
	if err != nil {
		t.Fatal(err)
	}
	assertCatalog(t, r, "storageclasses", false)
}
