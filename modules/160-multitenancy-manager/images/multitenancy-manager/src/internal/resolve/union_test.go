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

package resolve

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

func grantSelecting(name string, selector map[string]string) *v1alpha1.ClusterResourceGrantPolicy {
	return &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: selector},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
}

func grantNames(grants []*v1alpha1.ClusterResourceGrantPolicy) []string {
	names := make([]string, 0, len(grants))
	for _, g := range grants {
		names = append(names, g.Name)
	}
	return names
}

// TestGrantsForNamespace_ProjectLabelsSelectEveryNamespace: a label that lives only on the Project
// object selects the main and the additional namespace of that project. This is what USAGE promised
// and what "environment: production on the Project" in the audit expected.
func TestGrantsForNamespace_ProjectLabelsSelectEveryNamespace(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"env": "production"}}}
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	additional := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{naming.ProjectLabel: "team-a", "projects.deckhouse.io/project-namespace": "backend"}}}
	cl := newClient(t, project, main, additional, grantSelecting("by-env", map[string]string{"env": "production"}))

	for _, ns := range []*corev1.Namespace{main, additional} {
		grants, err := GrantsForNamespace(context.Background(), cl, ns)
		if err != nil {
			t.Fatalf("%s: %v", ns.Name, err)
		}
		if got := grantNames(grants); len(got) != 1 || got[0] != "by-env" {
			t.Fatalf("%s: a Project label must select the namespace, got %v", ns.Name, got)
		}
	}
}

// TestGrantsForNamespace_NamespaceValueWins: the same key on the Project and on the namespace with
// different values -- the namespace is closer to the object being checked, so its value decides.
func TestGrantsForNamespace_NamespaceValueWins(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"tier": "gold"}}}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: map[string]string{naming.ProjectLabel: "team-a", "tier": "silver"}}}
	cl := newClient(t, project, ns, grantSelecting("gold", map[string]string{"tier": "gold"}), grantSelecting("silver", map[string]string{"tier": "silver"}))

	grants, err := GrantsForNamespace(context.Background(), cl, ns)
	if err != nil {
		t.Fatal(err)
	}
	if got := grantNames(grants); len(got) != 1 || got[0] != "silver" {
		t.Fatalf("the namespace value must win the collision, got %v", got)
	}
}

// TestGrantsForNamespace_NamespaceOnlyKeysStillWork: the live policies select by
// projects.deckhouse.io/project-namespace, a key only the namespace carries; the union must not
// change their answer, and a namespace without a Project object is selected by its own labels.
func TestGrantsForNamespace_NamespaceOnlyKeysStillWork(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: map[string]string{naming.ProjectLabel: "team-a", "projects.deckhouse.io/project-namespace": "be"}}}
	cl := newClient(t, ns, grantSelecting("by-ns", map[string]string{"projects.deckhouse.io/project-namespace": "be"}))

	grants, err := GrantsForNamespace(context.Background(), cl, ns)
	if err != nil {
		t.Fatal(err)
	}
	if got := grantNames(grants); len(got) != 1 || got[0] != "by-ns" {
		t.Fatalf("namespace-only keys must keep selecting, got %v", got)
	}
}

func TestEffectiveLabels(t *testing.T) {
	got := EffectiveLabels(map[string]string{"a": "p", "b": "p"}, map[string]string{"b": "n", "c": "n"})
	want := map[string]string{"a": "p", "b": "n", "c": "n"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("key %s: got %q want %q", k, got[k], v)
		}
	}
	if EffectiveLabels(nil, nil) == nil {
		t.Fatal("must return an empty, non-nil map")
	}
}

func annotatedClass(name, value string) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: name}, Provisioner: "x"}
	if value != "" {
		sc.Annotations = map[string]string{"storageclass.kubernetes.io/is-default-class": value}
	}
	return sc
}

func defaultFromReg() *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			DefaultFrom:         &v1alpha1.DefaultFrom{AnnotationKey: "storageclass.kubernetes.io/is-default-class"},
		},
	}
}

func resolvedDefault(t *testing.T, objs ...client.Object) string {
	t.Helper()
	cl := newClient(t, objs...)
	resolved, err := Resolve(context.Background(), cl, testMapper(), defaultFromReg(), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resolved.Default()
}

// TestDefaultFrom_OnlyTrueCounts: the Kubernetes convention marks a non-default class with the same
// annotation set to "false"; the presence of the key must not make it the default.
func TestDefaultFrom_OnlyTrueCounts(t *testing.T) {
	if got := resolvedDefault(t, annotatedClass("a", "false"), annotatedClass("b", "")); got != "" {
		t.Fatalf(`"false" must not be a default, got %q`, got)
	}
	if got := resolvedDefault(t, annotatedClass("a", "true"), annotatedClass("b", "false")); got != "a" {
		t.Fatalf(`one "true" beside a "false" must pick the true one, got %q`, got)
	}
	if got := resolvedDefault(t, annotatedClass("a", "True"), annotatedClass("b", "")); got != "a" {
		t.Fatalf(`the value is compared case-insensitively, got %q`, got)
	}
	if got := resolvedDefault(t, annotatedClass("a", "true"), annotatedClass("b", "true")); got != "" {
		t.Fatalf(`two defaults mean no default, got %q`, got)
	}
}
