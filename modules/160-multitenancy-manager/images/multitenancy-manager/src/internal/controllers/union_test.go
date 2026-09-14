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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

// TestReconcile_ProjectLabelSelectsAdditionalNamespace: a policy selecting by a label that lives on
// the Project object materialises the catalog in the main and the additional namespace; taking the
// label off the Project removes the catalog from both on the next reconcile.
func TestReconcile_ProjectLabelSelectsAdditionalNamespace(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"env": "production"}}}
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	additional := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: map[string]string{naming.ProjectLabel: "team-a", "projects.deckhouse.io/project-namespace": "be"}}}
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
		},
	}
	grant := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "prod-classes"},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "production"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "standard"}, Provisioner: "x"}
	r := &ProjectReconciler{Client: buildClient(t, project, main, additional, def, grant, sc), Mapper: testMapper()}
	ctx := context.Background()

	for _, ns := range []string{"team-a", "team-a-be"} {
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ns}}); err != nil {
			t.Fatalf("reconcile %s: %v", ns, err)
		}
		ar := &v1alpha1.AvailableClusterResource{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "storageclasses"}, ar); err != nil {
			t.Fatalf("%s: a Project label must select the namespace, catalog missing: %v", ns, err)
		}
		if len(ar.Status.Available) != 1 || ar.Status.Available[0].Name != "standard" {
			t.Fatalf("%s: unexpected catalog %+v", ns, ar.Status.Available)
		}
	}

	// The requests the Project watch produces are exactly the two namespaces.
	reqs := r.namespacesOfProject(ctx, project)
	if len(reqs) != 2 {
		t.Fatalf("a Project change must enqueue its two namespaces, got %v", reqs)
	}

	project.Labels = nil
	if err := r.Update(ctx, project); err != nil {
		t.Fatal(err)
	}
	for _, ns := range []string{"team-a", "team-a-be"} {
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ns}}); err != nil {
			t.Fatalf("reconcile %s: %v", ns, err)
		}
		// The catalog object stays (an empty catalog is a fact worth publishing, see
		// TestReconcile_EmptyCatalogIsKept); what goes is its content.
		ar := &v1alpha1.AvailableClusterResource{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "storageclasses"}, ar); err != nil {
			t.Fatalf("%s: empty catalog must stay as an object: %v", ns, err)
		}
		if len(ar.Status.Available) != 0 || ar.Status.Default != "" {
			t.Fatalf("%s: catalog must empty once the Project label is removed, got %+v", ns, ar.Status)
		}
	}
}

// TestNamespacesOfProject_WithoutMainNamespaceYet: a Project whose main namespace does not carry the
// label yet is still enqueued by name, and system namespaces never are.
func TestNamespacesOfProject_WithoutMainNamespaceYet(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}}
	r := &ProjectReconciler{Client: buildClient(t, project), Mapper: testMapper()}
	reqs := r.namespacesOfProject(context.Background(), project)
	if len(reqs) != 1 || reqs[0].Name != "team-b" {
		t.Fatalf("expected the project's own namespace, got %v", reqs)
	}
	system := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}}
	if reqs := r.namespacesOfProject(context.Background(), system); len(reqs) != 0 {
		t.Fatalf("system namespaces must not be enqueued, got %v", reqs)
	}
}
