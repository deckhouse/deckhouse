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
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	rolebinding "controller/internal/rolebinding"
)

// fakeHelmClient is a stand-in for *helm.Client. It records the project state passed to the render
// path and lets tests script the apply/analyze outcomes so the up-to-date fallback can be asserted.
type fakeHelmClient struct {
	seenNamespaces []v1alpha3.NamespaceStatus
	applyResult    helm.ReleaseOutcome // returned by Upgrade/UpgradeManifests
	analyzeResult  helm.ReleaseOutcome // returned by Analyze{Manifests,Rendered}
	analyzeCalls   int
}

func (f *fakeHelmClient) UpgradeManifests(_ context.Context, project *v1alpha3.Project, _ string) (helm.ReleaseOutcome, error) {
	f.seenNamespaces = append([]v1alpha3.NamespaceStatus(nil), project.Status.Namespaces...)
	return f.applyResult, nil
}

func (f *fakeHelmClient) Upgrade(context.Context, *v1alpha3.Project, *v1alpha1.ProjectTemplate) (helm.ReleaseOutcome, error) {
	return f.applyResult, nil
}

func (f *fakeHelmClient) AnalyzeRendered(*v1alpha3.Project, *v1alpha1.ProjectTemplate) (helm.ReleaseOutcome, error) {
	f.analyzeCalls++
	return f.analyzeResult, nil
}

func (f *fakeHelmClient) AnalyzeManifests(*v1alpha3.Project, string) (helm.ReleaseOutcome, error) {
	f.analyzeCalls++
	return f.analyzeResult, nil
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

// TestUpgradeResourcesRecomputesOnUpToDate pins the fix for the filtered/roleRefs staleness: when the
// release is already up to date (apply short-circuits without post-rendering, Applied=false),
// upgradeResources must analyze the manifests to recover the filtered flag and role refs.
func TestUpgradeResourcesRecomputesOnUpToDate(t *testing.T) {
	m, _ := newManager(t)
	fh := &fakeHelmClient{
		applyResult: helm.ReleaseOutcome{Applied: false},
		analyzeResult: helm.ReleaseOutcome{
			Filtered: true,
			RoleRefs: []helm.BindingRoleRef{{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "b", RoleKind: "ClusterRole", RoleName: "d8:project:admin"}},
		},
	}
	m.helmClient = fh

	filtered, refs, err := m.upgradeResources(context.Background(), &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}, structuredTemplate())
	require.NoError(t, err)
	assert.True(t, filtered, "filtered must be recomputed from the manifests on an up-to-date release")
	assert.Len(t, refs, 1)
	assert.Equal(t, 1, fh.analyzeCalls, "an up-to-date release must trigger exactly one manifest analysis")
}

// TestUpgradeResourcesReusesApplyOutcome pins the perf side: when the release IS (re)applied the apply
// post-render already yields filtered/roleRefs, so no second analysis pass runs.
func TestUpgradeResourcesReusesApplyOutcome(t *testing.T) {
	m, _ := newManager(t)
	fh := &fakeHelmClient{
		applyResult: helm.ReleaseOutcome{
			Applied:  true,
			Filtered: true,
			RoleRefs: []helm.BindingRoleRef{{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "b", RoleKind: "ClusterRole", RoleName: "d8:project:admin"}},
		},
	}
	m.helmClient = fh

	filtered, refs, err := m.upgradeResources(context.Background(), &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}, structuredTemplate())
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Len(t, refs, 1)
	assert.Equal(t, 0, fh.analyzeCalls, "an applied release must not trigger a second post-render pass")
}

var errBoom = errors.New("boom")

func disabledRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{rolebinding.AnnotationDisabledForProjects: "true"},
		},
	}
}

func conditionByType(project *v1alpha3.Project, condName string) *v1alpha3.Condition {
	for i := range project.Status.Conditions {
		if project.Status.Conditions[i].Type == condName {
			return &project.Status.Conditions[i]
		}
	}
	return nil
}

func TestRoleViolation(t *testing.T) {
	m, _ := newManager(t,
		disabledRole("d8:project:secret-reader"),
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:project:admin"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "view"}},
	)
	ctx := context.Background()

	reason := func(name string, enforce bool) string {
		r, err := m.roleViolation(ctx, name, enforce)
		require.NoError(t, err)
		return r
	}

	// the disabled annotation is a violation regardless of the binding kind
	assert.Contains(t, reason("d8:project:secret-reader", true), "disabled")
	assert.Contains(t, reason("d8:project:secret-reader", false), "disabled")

	// an allowed role without the annotation is fine
	assert.Empty(t, reason("d8:project:admin", true))

	// a role outside the allow-list is a violation only for project bindings
	assert.Contains(t, reason("view", true), "allowed project role list")
	assert.Empty(t, reason("view", false))

	// a missing role is not flagged here; existence is enforced by the binding webhook
	assert.Empty(t, reason("d8:project:ghost", false))
}

// TestHandlePreCollectsNamespacesBeforeRender pins the reconcile ordering fix: Handle must refresh
// project.Status.Namespaces from the live cluster BEFORE rendering, so a schema-based template fans its
// namespaced objects into every project namespace on the same reconcile. The fake helm client records
// the namespace set it observed; it must include the additional namespace, not just the main one.
func TestHandlePreCollectsNamespacesBeforeRender(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))

	tmpl := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl"},
		Spec: v1alpha2.ProjectTemplateSpec{
			NetworkPolicy: &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)},
		},
	}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
	}
	label := map[string]string{v1alpha3.ResourceLabelProject: "proj"}
	mainNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: label}}
	extraNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra", Labels: label}}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(tmpl, project, mainNS, extraNS).
		WithStatusSubresource(&v1alpha3.Project{}).
		Build()
	fh := &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}}
	m := New(c, fh, logr.Discard())

	_, err := m.Handle(context.Background(), project)
	require.NoError(t, err)

	names := make([]string, 0, len(fh.seenNamespaces))
	for _, ns := range fh.seenNamespaces {
		names = append(names, ns.Name)
	}
	assert.ElementsMatch(t, []string{"proj", "proj-extra"}, names,
		"render must observe the additional namespace (status.namespaces pre-collected before render)")
}

// TestRoleViolationFailsClosed verifies that a transient API error while resolving a template role is
// propagated (fail closed) instead of being masked as "no violation".
func TestRoleViolationFailsClosed(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return apierrors.NewInternalError(errBoom)
		},
	}).Build()
	m := New(c, nil, logr.Discard())
	ctx := context.Background()

	_, err := m.roleViolation(ctx, "d8:project:admin", true)
	require.Error(t, err)

	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	condErr := m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
		{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "b", RoleKind: "ClusterRole", RoleName: "d8:project:admin"},
	})
	require.Error(t, condErr)
	// the condition must NOT have been fabricated as allowed
	assert.Nil(t, conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed))
}

func TestApplyTemplateRolesCondition(t *testing.T) {
	m, _ := newManager(t,
		disabledRole("d8:project:secret-reader"),
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:project:admin"}},
	)
	ctx := context.Background()

	t.Run("disabled role flips the condition to false", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		require.NoError(t, m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "secret", RoleKind: "ClusterRole", RoleName: "d8:project:secret-reader"},
		}))
		assert.True(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		assert.Contains(t, cond.Message, "d8:project:secret-reader")
		assert.Contains(t, cond.Message, "disabled")
	})

	t.Run("allowed role keeps the condition true", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		require.NoError(t, m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "ok", RoleKind: "ClusterRole", RoleName: "d8:project:admin"},
		}))
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		assert.Empty(t, cond.Message)
	})

	t.Run("non-clusterrole ref is ignored", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		require.NoError(t, m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: "RoleBinding", BindingName: "rb", RoleKind: "Role", RoleName: "anything"},
		}))
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
	})

	t.Run("native binding to a non-allow-list ClusterRole is allowed", func(t *testing.T) {
		// the allow-list is a PRB/CPRB policy; native RoleBindings may bind any existing role.
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		require.NoError(t, m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: "ClusterRoleBinding", BindingName: "crb", RoleKind: "ClusterRole", RoleName: "cluster-admin"},
		}))
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
	})

	t.Run("offending bindings are listed deterministically", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		require.NoError(t, m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "b", RoleKind: "ClusterRole", RoleName: "d8:project:secret-reader"},
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "a", RoleKind: "ClusterRole", RoleName: "kube-system-admin"},
		}))
		assert.True(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		// "a" (allow-list violation) sorts before "b" (disabled annotation)
		assert.Less(t, strings.Index(cond.Message, `"a"`), strings.Index(cond.Message, `"b"`))
	})
}
