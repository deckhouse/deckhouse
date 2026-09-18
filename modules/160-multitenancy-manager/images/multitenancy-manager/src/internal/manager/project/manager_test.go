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

package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// fakeHelmClient is a stand-in for *helm.Client. It records the project state passed to the render
// path and counts the releases applied.
type fakeHelmClient struct {
	seenNamespaces []v1alpha3.NamespaceStatus
	upgradeCalls   int
}

func (f *fakeHelmClient) UpgradeManifests(_ context.Context, project *v1alpha3.Project, _ string) error {
	f.upgradeCalls++
	f.seenNamespaces = append([]v1alpha3.NamespaceStatus(nil), project.Status.Namespaces...)
	return nil
}

func (f *fakeHelmClient) Delete(context.Context, string) error { return nil }

func structuredTemplate() *v1alpha2.ProjectTemplate {
	return &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl"},
		Spec: v1alpha2.ProjectTemplateSpec{
			NetworkPolicy: &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)},
		},
	}
}

// A template that came up from v1alpha1 with a Helm resourcesTemplate is marked, and its projects
// are parked in Error without touching the release: the empty structured shape it now has would
// otherwise be rendered over the objects the Helm string produced and delete them. Removing the mark
// -- what an administrator does after rewriting the template -- lets the render proceed.
func TestHandleTemplateRefusesALegacyHelmTemplate(t *testing.T) {
	tmpl := &v1alpha2.ProjectTemplate{ObjectMeta: metav1.ObjectMeta{
		Name:        "legacy",
		Annotations: map[string]string{v1alpha2.TemplateAnnotationLegacyHelm: "true"},
	}}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "legacy"},
	}
	m, c := newManager(t, tmpl, project)
	fh := &fakeHelmClient{}
	m.helmClient = fh
	ctx := context.Background()

	done, err := m.handleTemplate(ctx, project)
	require.NoError(t, err)
	assert.True(t, done, "the reconcile stops at the template")
	assert.Equal(t, 0, fh.upgradeCalls, "the release must not be touched")
	assert.Equal(t, v1alpha3.ProjectStateError, project.Status.State)
	assert.True(t, project.IsConditionFalse(v1alpha3.ProjectConditionProjectTemplateUsable))
	cond := conditionByType(project, v1alpha3.ProjectConditionProjectTemplateUsable)
	require.NotNil(t, cond)
	assert.Contains(t, cond.Message, v1alpha2.TemplateAnnotationLegacyHelm)

	stored := new(v1alpha3.Project)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, stored))
	assert.Equal(t, v1alpha3.ProjectStateError, stored.Status.State, "the refusal is persisted in the status")

	// the administrator rewrote the template and removed the mark
	tmpl.Annotations = nil
	require.NoError(t, c.Update(ctx, tmpl))
	project.ClearConditions()

	done, err = m.handleTemplate(ctx, project)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, 1, fh.upgradeCalls, "an unmarked template renders")
	assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionProjectTemplateUsable))
}

func conditionByType(project *v1alpha3.Project, condName string) *v1alpha3.Condition {
	for i := range project.Status.Conditions {
		if project.Status.Conditions[i].Type == condName {
			return &project.Status.Conditions[i]
		}
	}
	return nil
}

func TestEnsureTemplateName(t *testing.T) {
	t.Run("empty string becomes simple", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m, c := newManager(t, project)
		require.NoError(t, m.ensureTemplateName(context.Background(), project))
		assert.Equal(t, MinimalTemplate, project.Spec.ProjectTemplateName)

		got := new(v1alpha3.Project)
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "proj"}, got))
		assert.Equal(t, MinimalTemplate, got.Spec.ProjectTemplateName)
	})

	t.Run("existing template is left alone", func(t *testing.T) {
		project := &v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "proj"},
			Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "secure"},
		}
		m, c := newManager(t, project)
		before := new(v1alpha3.Project)
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "proj"}, before))

		require.NoError(t, m.ensureTemplateName(context.Background(), project))
		assert.Equal(t, "secure", project.Spec.ProjectTemplateName)

		got := new(v1alpha3.Project)
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "proj"}, got))
		assert.Equal(t, before.ResourceVersion, got.ResourceVersion)
	})

	t.Run("virtual project is skipped", func(t *testing.T) {
		project := &v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "deckhouse",
				Labels: map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"},
			},
		}
		m, c := newManager(t, project)
		require.NoError(t, m.ensureTemplateName(context.Background(), project))
		assert.Empty(t, project.Spec.ProjectTemplateName)

		got := new(v1alpha3.Project)
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "deckhouse"}, got))
		assert.Empty(t, got.Spec.ProjectTemplateName)
	})

	t.Run("leftover wrap is not pinned to simple", func(t *testing.T) {
		project := &v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "foo",
				Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
			},
		}
		m, c := newManager(t, project)
		require.NoError(t, m.ensureTemplateName(context.Background(), project))
		assert.Empty(t, project.Spec.ProjectTemplateName)

		got := new(v1alpha3.Project)
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
		assert.Empty(t, got.Spec.ProjectTemplateName)
		assert.Equal(t, v1alpha3.ManagedByNamespace, got.Labels[v1alpha3.ProjectLabelManagedByNamespace])
	})
}

func TestHandle_HoldsOffYoungUnstampedNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:              "foo",
		CreationTimestamp: metav1.Now(),
	}}
	proj := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "simple"},
	}
	m, c := newManager(t, ns, proj)
	fh := &fakeHelmClient{}
	m.helmClient = fh

	res, err := m.Handle(context.Background(), proj.DeepCopy())
	require.NoError(t, err)
	assert.Greater(t, res.RequeueAfter, time.Duration(0))
	assert.LessOrEqual(t, res.RequeueAfter, helm.AdoptGracePeriod)
	assert.Zero(t, fh.upgradeCalls)

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.Empty(t, updated.Annotations[helm.ResourceAnnotationReleaseName])
	assert.NotEqual(t, helm.ManagedByHelm, updated.Labels[helm.ResourceLabelManagedBy])

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.NotEqual(t, v1alpha3.ProjectStateError, got.Status.State)
}

func TestHandle_DeletesLeftoverWrapWhenNamespaceGone(t *testing.T) {
	leftover := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "foo",
			Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
		},
	}
	m, c := newManager(t, leftover)
	fh := &fakeHelmClient{}
	m.helmClient = fh

	_, err := m.Handle(context.Background(), leftover)
	require.NoError(t, err)
	assert.Zero(t, fh.upgradeCalls)

	got := new(v1alpha3.Project)
	err = c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got)
	require.True(t, apierrors.IsNotFound(err) || !got.DeletionTimestamp.IsZero())
}

func TestHandle_LeftoverWrapKeepsIdentityWhenHelmForeign(t *testing.T) {
	leftover := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "foo",
			Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
		},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			helm.ResourceAnnotationReleaseName:      "foo",
			helm.ResourceAnnotationReleaseNamespace: "foo",
		},
	}}
	m, c := newManager(t, leftover, ns)
	fh := &fakeHelmClient{}
	m.helmClient = fh

	_, err := m.Handle(context.Background(), leftover)
	require.Error(t, err)
	assert.ErrorIs(t, err, helm.ErrForeignRelease)
	assert.Zero(t, fh.upgradeCalls)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Empty(t, got.Spec.ProjectTemplateName)
	assert.Equal(t, v1alpha3.ManagedByNamespace, got.Labels[v1alpha3.ProjectLabelManagedByNamespace])
	assert.Equal(t, v1alpha3.ProjectStateError, got.Status.State)
	assert.True(t, got.IsConditionFalse(v1alpha3.ProjectConditionHelmOwnership))

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.Equal(t, "foo", updated.Annotations[helm.ResourceAnnotationReleaseName])
	assert.Equal(t, "foo", updated.Annotations[helm.ResourceAnnotationReleaseNamespace])
}

func TestHandle_RequeuesLeftoverWrapWhenNamespaceGetFails(t *testing.T) {
	leftover := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "foo",
			Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(leftover).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				return errors.New("namespace get failed")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}).Build()
	m := New(c, &fakeHelmClient{}, logr.Discard())

	_, err := m.Handle(context.Background(), leftover.DeepCopy())
	require.Error(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Empty(t, got.Spec.ProjectTemplateName)
	assert.Equal(t, v1alpha3.ManagedByNamespace, got.Labels[v1alpha3.ProjectLabelManagedByNamespace])
}
