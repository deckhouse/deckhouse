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
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

func grantNames(grants []*grantsv1alpha2.ClusterResourceGrantPolicy) []string {
	names := make([]string, 0, len(grants))
	for _, g := range grants {
		names = append(names, g.Name)
	}
	return names
}

func policyWith(projectSelector, namespaceSelector *metav1.LabelSelector) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			ProjectSelector:   projectSelector,
			NamespaceSelector: namespaceSelector,
			Resources:         []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
}

func labelsSelector(key, value string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: map[string]string{key: value}}
}

// TestPolicyMatches: projectSelector reads only the labels of the Project, namespaceSelector only the
// labels of the namespace, and a policy applies when every selector it sets matches.
func TestPolicyMatches(t *testing.T) {
	t.Parallel()

	project := map[string]string{"team": "a"}
	namespace := map[string]string{"env": "prod", "projects.deckhouse.io/project-namespace": "cache"}

	tests := []struct {
		name              string
		projectSelector   *metav1.LabelSelector
		namespaceSelector *metav1.LabelSelector
		expected          bool
	}{
		{name: "project selector on a project label", projectSelector: labelsSelector("team", "a"), expected: true},
		{name: "project selector does not see namespace labels", projectSelector: labelsSelector("env", "prod"), expected: false},
		{name: "namespace selector on a namespace label", namespaceSelector: labelsSelector("env", "prod"), expected: true},
		{name: "namespace selector does not see project labels", namespaceSelector: labelsSelector("team", "a"), expected: false},
		{name: "both selectors match", projectSelector: labelsSelector("team", "a"), namespaceSelector: labelsSelector("projects.deckhouse.io/project-namespace", "cache"), expected: true},
		{name: "both set, the namespace one does not match", projectSelector: labelsSelector("team", "a"), namespaceSelector: labelsSelector("env", "dev"), expected: false},
		{name: "both set, the project one does not match", projectSelector: labelsSelector("team", "b"), namespaceSelector: labelsSelector("env", "prod"), expected: false},
		{name: "a library policy matches nothing", expected: false},
		{name: "an empty project selector matches every project", projectSelector: &metav1.LabelSelector{}, expected: true},
		{name: "an empty namespace selector matches every namespace", namespaceSelector: &metav1.LabelSelector{}, expected: true},
		{
			name: "match expressions on the namespace",
			namespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "projects.deckhouse.io/project-namespace", Operator: metav1.LabelSelectorOpDoesNotExist},
			}},
			expected: false,
		},
		{
			name: "an invalid selector matches nothing",
			projectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "team", Operator: "Bogus"},
			}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PolicyMatches(policyWith(tt.projectSelector, tt.namespaceSelector), project, namespace); got != tt.expected {
				t.Fatalf("got %v, expected %v", got, tt.expected)
			}
		})
	}
}

// TestGrantsForNamespace_ProjectLabelsSelectEveryNamespace: a label on the Project selects the main
// namespace and an additional one, which carries no such label itself.
func TestGrantsForNamespace_ProjectLabelsSelectEveryNamespace(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"env": "production"}}}
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	additional := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{naming.ProjectLabel: "team-a", "projects.deckhouse.io/project-namespace": "backend"}}}
	policy := policyWith(labelsSelector("env", "production"), nil)
	policy.Name = "by-env"
	cl := newClient(t, project, main, additional, policy)

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

// TestGrantsForNamespace_NamespaceSelectorPicksOneNamespace: a namespace selector on a key only the
// additional namespace carries selects that namespace and not the main one.
func TestGrantsForNamespace_NamespaceSelectorPicksOneNamespace(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{naming.ProjectLabel: "team-a"}}}
	additional := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-be", Labels: map[string]string{naming.ProjectLabel: "team-a", "projects.deckhouse.io/project-namespace": "be"}}}
	policy := policyWith(nil, labelsSelector("projects.deckhouse.io/project-namespace", "be"))
	policy.Name = "by-ns"
	cl := newClient(t, project, main, additional, policy)

	for ns, expected := range map[*corev1.Namespace]int{main: 0, additional: 1} {
		grants, err := GrantsForNamespace(context.Background(), cl, ns)
		if err != nil {
			t.Fatal(err)
		}
		if got := grantNames(grants); len(got) != expected {
			t.Fatalf("%s: got %v, expected %d policies", ns.Name, got, expected)
		}
	}
}

// TestGrantsForNamespace_NamespaceWithoutProject: a namespace no Project owns has no project labels, so
// only a policy whose project selector accepts an empty label set can apply to it.
func TestGrantsForNamespace_NamespaceWithoutProject(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "orphan", Labels: map[string]string{"env": "prod"}}}
	byProject := policyWith(labelsSelector("team", "a"), nil)
	byProject.Name = "by-project"
	byNamespace := policyWith(nil, labelsSelector("env", "prod"))
	byNamespace.Name = "by-namespace"
	cl := newClient(t, ns, byProject, byNamespace)

	grants, err := GrantsForNamespace(context.Background(), cl, ns)
	if err != nil {
		t.Fatal(err)
	}
	if got := grantNames(grants); len(got) != 1 || got[0] != "by-namespace" {
		t.Fatalf("got %v", got)
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
