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

package namespace

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"controller/apis/deckhouse.io/v1alpha3"
)

func newManager(t *testing.T, objs ...client.Object) (*Manager, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return New(c, logr.Discard()), c
}

func namespace(name string, labels, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func TestWrap_CreatesManagedProjectAndFinalizer(t *testing.T) {
	ns := namespace("foo", map[string]string{"team": "blue"}, map[string]string{"note": "hi"})
	m, c := newManager(t, ns)

	_, err := m.Wrap(context.Background(), ns)
	require.NoError(t, err)

	project := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, project))
	assert.Equal(t, v1alpha3.ManagedByNamespace, project.Labels[v1alpha3.ProjectLabelManagedByNamespace])
	assert.Empty(t, project.Spec.ProjectTemplateName, "managed projects use the template-less path")

	// labels/annotations are mirrored into spec.parameters.namespace
	nsParams, ok := project.Spec.Parameters["namespace"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"team": "blue"}, nsParams["labels"])
	assert.Equal(t, map[string]any{"note": "hi"}, nsParams["annotations"])

	// the namespace gets the managed-project finalizer and the project-managed-by-namespace marker
	// label (so the d8-multitenancy-manager admission policy exempts it from edit/delete protection).
	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.True(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.Equal(t, v1alpha3.ManagedByNamespace, updated.Labels[v1alpha3.ProjectLabelManagedByNamespace])

	// the marker is filtered out of the mirrored spec.parameters.namespace.labels.
	nsLabels, _ := nsParams["labels"].(map[string]any)
	assert.NotContains(t, nsLabels, v1alpha3.ProjectLabelManagedByNamespace)
}

func TestWrap_Idempotent(t *testing.T) {
	ns := namespace("foo", nil, nil)
	m, c := newManager(t, ns)

	_, err := m.Wrap(context.Background(), ns)
	require.NoError(t, err)
	// reload with the finalizer the first pass added
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, ns))

	_, err = m.Wrap(context.Background(), ns)
	require.NoError(t, err)

	projects := new(v1alpha3.ProjectList)
	require.NoError(t, c.List(context.Background(), projects))
	assert.Len(t, projects.Items, 1)
}

func TestWrap_SkipsRegularProject(t *testing.T) {
	// a regular project owns a namespace with the same name; auto-wrap must not touch it.
	ns := namespace("bar", map[string]string{v1alpha3.ResourceLabelProject: "bar"}, nil)
	regular := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	m, c := newManager(t, ns, regular)

	_, err := m.Wrap(context.Background(), ns)
	require.NoError(t, err)

	project := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "bar"}, project))
	assert.NotContains(t, project.Labels, v1alpha3.ProjectLabelManagedByNamespace)

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "bar"}, updated))
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
}

func TestWrap_DetachRemovesFinalizerAndMarker(t *testing.T) {
	// The project lost its managed-by-namespace label (detached) but the namespace still carries
	// the finalizer and marker from when it was wrapped. Wrap must strip both so the now-regular
	// project's namespace is re-protected by the admission policy.
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		v1alpha3.ResourceLabelProject:           "foo",
	}, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
	detached := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	m, c := newManager(t, ns, detached)

	_, err := m.Wrap(context.Background(), ns)
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
}

func TestSyncParameters_MirrorsLabelChanges(t *testing.T) {
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}},
	}
	ns := namespace("foo", map[string]string{
		"env":                          "prod",
		v1alpha3.ResourceLabelProject:  "foo",
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
	}, nil)
	m, c := newManager(t, ns, project)

	require.NoError(t, m.syncParameters(context.Background(), ns, project))

	updated := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	nsParams := updated.Spec.Parameters["namespace"].(map[string]any)
	// controller-managed labels are excluded; only the user label is mirrored
	assert.Equal(t, map[string]any{"env": "prod"}, nsParams["labels"])
}

func TestHandleDeletion_CascadesManagedProject(t *testing.T) {
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}},
	}
	ns := namespace("foo", nil, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
	m, c := newManager(t, ns, project)

	// deleting a finalizer-protected namespace sets DeletionTimestamp but keeps the object.
	require.NoError(t, c.Delete(context.Background(), ns))
	deleting := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, deleting))
	require.False(t, deleting.DeletionTimestamp.IsZero())

	_, err := m.HandleDeletion(context.Background(), deleting)
	require.NoError(t, err)

	// the managed project is deleted and the namespace finalizer is released (namespace removed).
	err = c.Get(context.Background(), client.ObjectKey{Name: "foo"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err))
	err = c.Get(context.Background(), client.ObjectKey{Name: "foo"}, new(corev1.Namespace))
	assert.True(t, apierrors.IsNotFound(err))
}

func TestHandleDeletion_KeepsDetachedProject(t *testing.T) {
	// after detach the project no longer carries the managed-by-namespace label; deleting the
	// namespace must release the finalizer but keep the now-independent project.
	detached := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	ns := namespace("foo", nil, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
	m, c := newManager(t, ns, detached)

	require.NoError(t, c.Delete(context.Background(), ns))
	deleting := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, deleting))

	_, err := m.HandleDeletion(context.Background(), deleting)
	require.NoError(t, err)

	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, new(v1alpha3.Project)))
	err = c.Get(context.Background(), client.ObjectKey{Name: "foo"}, new(corev1.Namespace))
	assert.True(t, apierrors.IsNotFound(err))
}

func TestMirrorParameters_FiltersManagedKeys(t *testing.T) {
	ns := namespace("foo",
		map[string]string{
			"team":                         "blue",
			v1alpha3.ResourceLabelProject:  "foo",
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
			"kubernetes.io/metadata.name":  "foo",
		},
		map[string]string{
			"owner":                           "alice",
			"meta.helm.sh/release-name":       "foo",
			v1alpha3.NamespaceAnnotationAdopt: "",
		},
	)
	got := mirrorParameters(ns)
	nsParams := got["namespace"].(map[string]any)
	assert.Equal(t, map[string]any{"team": "blue"}, nsParams["labels"])
	assert.Equal(t, map[string]any{"owner": "alice"}, nsParams["annotations"])
}

func TestMirrorParameters_NilWhenEmpty(t *testing.T) {
	ns := namespace("foo", map[string]string{v1alpha3.ResourceLabelProject: "foo"}, nil)
	assert.Nil(t, mirrorParameters(ns))
}
