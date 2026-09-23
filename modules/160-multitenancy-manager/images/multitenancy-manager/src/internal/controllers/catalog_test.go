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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/api/v1alpha1"
	"controller/internal/naming"
)

func storageClassDefinition(defaultAvailability v1alpha1.AvailabilityDefault) *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: defaultAvailability,
		},
	}
}

// TestReconcile_CatalogCarriesTheModuleHeritage: the catalog is labelled as the module's own object,
// which is what the protective admission policy binds on. With heritage=deckhouse it was protected
// by the /protect webhook alone, and that webhook is fail-open.
func TestReconcile_CatalogCarriesTheModuleHeritage(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	// A catalog left over from a release that stamped heritage=deckhouse is relabelled in place.
	stale := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: "storageclasses", Namespace: "team-a", Labels: map[string]string{"heritage": "deckhouse"}}}
	r := &ProjectReconciler{Client: buildClient(t, ns, storageClassDefinition(v1alpha1.AvailabilityAll), sc, stale), Mapper: testMapper()}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
		t.Fatal(err)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "team-a", Name: "storageclasses"}, ar); err != nil {
		t.Fatal(err)
	}
	if ar.Labels[naming.HeritageLabel] != "multitenancy-manager" {
		t.Fatalf("catalog heritage = %q, want multitenancy-manager", ar.Labels[naming.HeritageLabel])
	}
}

// TestReconcile_EmptyCatalogIsKept: nothing available is a fact worth publishing. The object stays,
// with an empty list, an empty default and a zero count, instead of being deleted -- a reader could
// not tell a deleted catalog from one the controller has not produced yet.
func TestReconcile_EmptyCatalogIsKept(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	r := &ProjectReconciler{Client: buildClient(t, ns, storageClassDefinition(v1alpha1.AvailabilityNone), sc), Mapper: testMapper()}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}}); err != nil {
		t.Fatal(err)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "team-a", Name: "storageclasses"}, ar); err != nil {
		t.Fatalf("an empty catalog must exist as an object: %v", err)
	}
	if ar.Status.Available == nil || len(ar.Status.Available) != 0 || ar.Status.Default != "" || ar.Status.AvailableCount != 0 {
		t.Fatalf("empty catalog must read as [] / \"\" / 0, got %+v", ar.Status)
	}
}

// TestReconcile_TerminatingNamespaceIsLeftAlone: a namespace with a deletionTimestamp accepts no new
// objects; reconciling it produced an error and a retry per registration for the whole time the
// namespace was draining.
func TestReconcile_TerminatingNamespaceIsLeftAlone(t *testing.T) {
	now := metav1.Now()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}, DeletionTimestamp: &now, Finalizers: []string{"kubernetes"}}}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	r := &ProjectReconciler{Client: buildClient(t, ns, storageClassDefinition(v1alpha1.AvailabilityAll), sc), Mapper: testMapper()}

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	if err != nil {
		t.Fatalf("a terminating namespace must not error: %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Fatalf("a terminating namespace must not be requeued, got %v", res.RequeueAfter)
	}
	ar := &v1alpha1.AvailableClusterResource{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "team-a", Name: "storageclasses"}, ar); !k8serrors.IsNotFound(err) {
		t.Fatalf("nothing must be written into a terminating namespace, err=%v", err)
	}
}
