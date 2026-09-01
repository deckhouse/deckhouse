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
	"strings"
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
	"controller/internal/helm"
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

func project(name string, labels map[string]string, template string) *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: template},
	}
}

func TestAdopt_CreatesFullProject(t *testing.T) {
	ns := namespace("foo", map[string]string{"team": "blue"}, map[string]string{"note": "hi"})
	m, c := newManager(t, ns)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSimple, got.Spec.ProjectTemplateName)
	assert.NotContains(t, got.Labels, v1alpha3.ProjectLabelManagedByNamespace)

	nsParams, ok := got.Spec.Parameters["namespace"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"team": "blue"}, nsParams["labels"])
	assert.Equal(t, map[string]any{"note": "hi"}, nsParams["annotations"])

	// the namespace is handed over to helm so the project release can own an object it did not create.
	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.Equal(t, helm.ManagedByHelm, updated.Labels[helm.ResourceLabelManagedBy])
	assert.Equal(t, "foo", updated.Annotations[helm.ResourceAnnotationReleaseName])
	assert.Equal(t, "", updated.Annotations[helm.ResourceAnnotationReleaseNamespace])
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
}

func TestAdopt_PicksTemplateFromNamespaceState(t *testing.T) {
	ns := namespace("foo", map[string]string{
		labelSecurityScanning: "",
		labelPodPolicy:        "restricted",
	}, nil)
	m, c := newManager(t, ns)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSecure, got.Spec.ProjectTemplateName)
	assert.Equal(t, podSecurityProfileRestricted, got.Spec.Parameters["podSecurityProfile"])
	assert.Equal(t, networkPolicyNotRestricted, got.Spec.Parameters["networkPolicy"])
	assert.Equal(t, true, got.Spec.Parameters["securityScanningEnabled"])
}

func TestAdopt_StampsHelmWhenProjectExists(t *testing.T) {
	ns := namespace("bar", nil, nil)
	existing := project("bar", nil, TemplateDefault)
	m, c := newManager(t, ns, existing)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "bar"}, got))
	assert.Equal(t, TemplateDefault, got.Spec.ProjectTemplateName, "the existing project must be left alone")

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "bar"}, updated))
	assert.Equal(t, helm.ManagedByHelm, updated.Labels[helm.ResourceLabelManagedBy])
	assert.Equal(t, "bar", updated.Annotations[helm.ResourceAnnotationReleaseName])
	assert.Equal(t, "", updated.Annotations[helm.ResourceAnnotationReleaseNamespace])
}

func TestAdopt_StampsTruncatedHelmReleaseName(t *testing.T) {
	name := "t-mtm61-" + strings.Repeat("x", 53)
	require.Equal(t, 61, len(name))
	ns := namespace(name, nil, nil)
	m, c := newManager(t, ns)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, updated))
	rel := helm.ReleaseName(name)
	assert.NotEqual(t, name, rel)
	assert.Equal(t, rel, updated.Annotations[helm.ResourceAnnotationReleaseName])
}

func TestAdopt_Idempotent(t *testing.T) {
	ns := namespace("foo", nil, nil)
	m, c := newManager(t, ns)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, ns))
	_, err = m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	projects := new(v1alpha3.ProjectList)
	require.NoError(t, c.List(context.Background(), projects))
	assert.Len(t, projects.Items, 1)
}

func TestMigrate_ConvertsNamespaceManagedProject(t *testing.T) {
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		v1alpha3.ResourceLabelProject:           "foo",
		labelPodPolicy:                          "baseline",
		"team":                                  "blue",
	}, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
	managed := project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, "")
	m, c := newManager(t, ns, managed)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateDefault, got.Spec.ProjectTemplateName)
	assert.NotContains(t, got.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.Equal(t, podSecurityProfileBaseline, got.Spec.Parameters["podSecurityProfile"])

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.Equal(t, helm.ManagedByHelm, updated.Labels[helm.ResourceLabelManagedBy])
}

func TestMigrate_ConvertsTemplateLessProject(t *testing.T) {
	ns := namespace("foo", nil, nil)
	bare := project("foo", nil, "")
	m, c := newManager(t, ns, bare)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSimple, got.Spec.ProjectTemplateName)
}

func TestMigrate_KeepsVirtualProjects(t *testing.T) {
	virtual := project("deckhouse", map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}, "virtual")
	m, c := newManager(t, virtual)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "deckhouse"}, got))
	assert.Equal(t, "virtual", got.Spec.ProjectTemplateName)
}

func TestMigrate_LeavesMigratedProjectAlone(t *testing.T) {
	ns := namespace("foo", nil, nil)
	done := project("foo", nil, TemplateSecure)
	done.Spec.Parameters = map[string]any{"podSecurityProfile": podSecurityProfileRestricted}
	m, c := newManager(t, ns, done)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSecure, got.Spec.ProjectTemplateName)
	assert.Equal(t, podSecurityProfileRestricted, got.Spec.Parameters["podSecurityProfile"])
}

func TestMigrate_Idempotent(t *testing.T) {
	ns := namespace("foo", nil, nil)
	bare := project("foo", nil, "")
	m, c := newManager(t, ns, bare)

	require.NoError(t, m.Migrate(context.Background()))
	first := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, first))

	require.NoError(t, m.Migrate(context.Background()))
	second := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, second))

	assert.Equal(t, first.ResourceVersion, second.ResourceVersion, "a second pass must not write")
}

func TestNeedsTemplate_ExplicitEmptyString(t *testing.T) {
	// The CRD defaults an ABSENT projectTemplateName, but an explicit "" is stored verbatim — and
	// that spelling used to mean "a project without a template", so old manifests still carry it.
	// It has to be recognised as needing a template, not mistaken for an already-migrated project.
	assert.True(t, needsTemplate(project("foo", nil, "")))
}

func TestMigrate_ClearsLeftoverMarkersOnAlreadyMigrated(t *testing.T) {
	// A previous run wrote the template (needsTemplate is now false) and then failed
	// to peel the namespace finalizer. The sweep must still remove it.
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		v1alpha3.ResourceLabelProject:           "foo",
	}, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject, "example.com/keep"}
	done := project("foo", nil, TemplateSimple)
	m, c := newManager(t, ns, done)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSimple, got.Spec.ProjectTemplateName)

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.True(t, controllerutil.ContainsFinalizer(updated, "example.com/keep"))
}

func TestMigrate_PreservesHandmadeProjectParameters(t *testing.T) {
	ns := namespace("foo", map[string]string{labelPodPolicy: "restricted"}, nil)
	bare := project("foo", nil, "")
	bare.Spec.Parameters = map[string]any{"networkPolicy": "Isolated"}
	m, c := newManager(t, ns, bare)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateDefault, got.Spec.ProjectTemplateName)
	assert.Equal(t, "Isolated", got.Spec.Parameters["networkPolicy"])
	assert.NotContains(t, got.Spec.Parameters, "podSecurityProfile")
}

func TestMigrate_ProjectWithoutNamespace(t *testing.T) {
	// the namespace has not been created yet, so there is no state to preserve.
	bare := project("foo", nil, "")
	m, c := newManager(t, bare)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateSimple, got.Spec.ProjectTemplateName)
	assert.Nil(t, got.Spec.Parameters)
}

func TestMigrate_DeletesLeftoverWrapWhenNamespaceGone(t *testing.T) {
	managed := project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, "")
	m, c := newManager(t, managed)

	require.NoError(t, m.Migrate(context.Background()))

	got := new(v1alpha3.Project)
	err := c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got)
	require.True(t, apierrors.IsNotFound(err) || !got.DeletionTimestamp.IsZero(), "leftover wrap must not stay as a live project: %v", err)
	if err == nil {
		assert.Empty(t, got.Spec.ProjectTemplateName)
	}
}

func TestMigrate_DeletesLeftoverWrapWhenNamespaceTerminating(t *testing.T) {
	now := metav1.Now()
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
	}, nil)
	ns.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
	ns.DeletionTimestamp = &now
	managed := project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, "")
	m, c := newManager(t, ns, managed)

	require.NoError(t, m.Migrate(context.Background()))

	updated := new(corev1.Namespace)
	err := c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated)
	if err == nil {
		assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	}

	got := new(v1alpha3.Project)
	err = c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got)
	require.True(t, apierrors.IsNotFound(err) || !got.DeletionTimestamp.IsZero(), "leftover wrap must not be templated over a terminating namespace")
	if err == nil {
		assert.Empty(t, got.Spec.ProjectTemplateName)
	}
}
