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

package namespaces

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/edition"
)

func labelled(name, project string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   name,
		Labels: map[string]string{v1alpha3.ResourceLabelProject: project},
	}}
}

// fixture is team-a with its main namespace, a namespace of a ProjectNamespace, a namespace labelled
// by hand and a namespace of team-b.
func fixture(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		labelled("team-a", "team-a"),
		labelled("team-a-cache", "team-a"),
		labelled("billing", "team-a"),
		labelled("team-b-cache", "team-b"),
		&v1alpha3.ProjectNamespace{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "team-a"},
			Spec:       v1alpha3.ProjectNamespaceSpec{Name: "cache"},
		},
	).Build()
}

func names(items []corev1.Namespace) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		out = append(out, items[i].Name)
	}
	return out
}

// The tests switch the edition of the binary, so none of them runs in parallel.
func TestOfProject(t *testing.T) {
	c := fixture(t)

	t.Run("every labelled namespace in an edition with additional namespaces", func(t *testing.T) {
		defer edition.SetForTest("EE")()
		items, err := OfProject(context.Background(), c, "team-a")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"team-a", "team-a-cache", "billing"}, names(items))
	})

	t.Run("in CE, only the main namespace, even one a ProjectNamespace claims", func(t *testing.T) {
		defer edition.SetForTest(edition.CE)()
		items, err := OfProject(context.Background(), c, "team-a")
		require.NoError(t, err)
		assert.Equal(t, []string{"team-a"}, names(items))
	})

	t.Run("in CE, nothing for a project whose main namespace is gone", func(t *testing.T) {
		defer edition.SetForTest(edition.CE)()
		items, err := OfProject(context.Background(), c, "team-b")
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("in a build without an edition, as in the editions with additional namespaces", func(t *testing.T) {
		defer edition.SetForTest("")()
		items, err := OfProject(context.Background(), c, "team-a")
		require.NoError(t, err)
		assert.Len(t, items, 3)
	})
}

func TestInProject(t *testing.T) {
	for _, tc := range []struct {
		edition  string
		ns       *corev1.Namespace
		project  string
		expected bool
	}{
		{edition: edition.CE, ns: labelled("team-a", "team-a"), project: "team-a", expected: true},
		{edition: edition.CE, ns: labelled("team-a-cache", "team-a"), project: "team-a", expected: false},
		{edition: edition.CE, ns: labelled("billing", "team-a"), project: "team-a", expected: false},
		{edition: edition.CE, ns: labelled("team-b", "team-b"), project: "team-a", expected: false},
		{edition: "EE", ns: labelled("billing", "team-a"), project: "team-a", expected: true},
		{edition: "EE", ns: labelled("team-b-cache", "team-b"), project: "team-a", expected: false},
		{edition: "EE", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}, project: "team-a", expected: false},
	} {
		t.Run(tc.edition+"/"+tc.ns.Name, func(t *testing.T) {
			defer edition.SetForTest(tc.edition)()
			assert.Equal(t, tc.expected, InProject(tc.ns, tc.project))
		})
	}
}

// Dropped lists the namespaces with the label of the project that are not namespaces of it, which only
// CE has, and leaves out the ones that are being deleted; DroppedOfAnyProject does it for every project.
func TestDropped(t *testing.T) {
	ctx := context.Background()
	going := labelled("team-a-old", "team-a")
	going.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	going.Finalizers = []string{"kubernetes"}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		labelled("team-a", "team-a"), labelled("team-a-cache", "team-a"), going,
		labelled("team-b", "team-b"), labelled("team-b-db", "team-b"),
	).Build()

	t.Run("CE", func(t *testing.T) {
		defer edition.SetForTest(edition.CE)()
		dropped, err := Dropped(ctx, c, "team-a")
		require.NoError(t, err)
		assert.Equal(t, map[string]struct{}{"team-a-cache": {}}, dropped)
		dropped, err = DroppedOfAnyProject(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, map[string]struct{}{"team-a-cache": {}, "team-b-db": {}}, dropped)
	})

	t.Run("a commercial edition", func(t *testing.T) {
		defer edition.SetForTest("EE")()
		dropped, err := DroppedOfAnyProject(ctx, c)
		require.NoError(t, err)
		assert.Empty(t, dropped)
	})
}
