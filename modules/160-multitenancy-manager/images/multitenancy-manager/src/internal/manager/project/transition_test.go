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
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// newTransitionManager is newManager with the AuthorizationRule kind known to the fake cluster, the
// way user-authz serves it, the kinds of a workload, and the status of the standard-field objects
// writable.
func newTransitionManager(t *testing.T, objs ...client.Object) (*Manager, client.Client) {
	t.Helper()
	return newTransitionManagerWith(t, interceptor.Funcs{}, objs...)
}

// newTransitionManagerWith is newTransitionManager over a fake cluster whose calls go through funcs.
func newTransitionManagerWith(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) (*Manager, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, rbacv1.AddToScheme, appsv1.AddToScheme, networkingv1.AddToScheme,
		v1alpha2.AddToScheme, v1alpha3.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	scheme.AddKnownTypeWithName(authorizationRuleGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(authorizationRuleGVK.GroupVersion().WithKind("AuthorizationRuleList"), &unstructured.UnstructuredList{})
	c := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(testRESTMapper(scheme)).WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.Project{}, &v1alpha3.ProjectRoleBinding{}, &corev1.ResourceQuota{}).
		WithInterceptorFuncs(funcs).Build()
	return New(c, nil, logr.Discard()), c
}

// testRESTMapper maps every kind of the scheme with the scope the API server gives it, so the scope a
// read depends on is the real one.
func testRESTMapper(scheme *runtime.Scheme) meta.RESTMapper {
	clusterScoped := map[string]bool{
		"Namespace": true, "ClusterRole": true, "ClusterRoleBinding": true,
		"Project": true, "ProjectTemplate": true, "ClusterProjectRoleBinding": true,
	}
	mapper := meta.NewDefaultRESTMapper(nil)
	for gvk := range scheme.AllKnownTypes() {
		scope := meta.RESTScopeNamespace
		if clusterScoped[strings.TrimSuffix(gvk.Kind, "List")] {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(gvk, scope)
	}
	return mapper
}

// releaseObjectLabels are the labels the post-renderer gives an object of the release of the project.
func releaseObjectLabels() map[string]string {
	return map[string]string{
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:  "proj",
		v1alpha3.ResourceLabelTemplate: "custom",
	}
}

// releaseAnnotations are the annotations of an object of the release of the project: the release
// name Helm records, and the given ones.
func releaseAnnotations(annotations map[string]string) map[string]string {
	merged := map[string]string{helm.ResourceAnnotationReleaseName: "proj"}
	maps.Copy(merged, annotations)
	return merged
}

// releaseMeta is the metadata of an object the release of the project created in its main namespace.
func releaseMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: "proj", Name: name, Labels: releaseObjectLabels(), Annotations: releaseAnnotations(nil)}
}

// liveRule is the AuthorizationRule the former default Helm text rendered for a user administrator.
func liveRule(name string) *unstructured.Unstructured {
	rule := new(unstructured.Unstructured)
	rule.SetGroupVersionKind(authorizationRuleGVK)
	rule.SetNamespace("proj")
	rule.SetName(name)
	rule.SetLabels(releaseObjectLabels())
	rule.SetAnnotations(releaseAnnotations(nil))
	rule.Object["spec"] = map[string]any{
		"accessLevel":    "Admin",
		"portForwarding": false,
		"allowScale":     false,
		"subjects":       []any{map[string]any{"kind": "User", "name": name}},
	}
	return rule
}

// withSpec is the rule with another spec.
func withSpec(rule *unstructured.Unstructured, spec map[string]any) *unstructured.Unstructured {
	rule.Object["spec"] = spec
	return rule
}

// liveQuota is the all-pods quota the former default Helm text rendered.
func liveQuota(hard corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{ObjectMeta: releaseMeta("all-pods"), Spec: corev1.ResourceQuotaSpec{Hard: hard}}
}

// helmProject is a project with an administrator and a quota.
func helmProject() *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec: v1alpha3.ProjectSpec{
			ProjectTemplateName: "custom",
			Administrators:      []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			Quota:               corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
		},
	}
}

// projectNamespace is the main namespace of the project in the fake cluster.
func projectNamespace() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: releaseObjectLabels()}}
}

// ref is the release object of the kind and name in the main namespace.
func ref(apiVersion, kind, name string) helm.ReleaseObject {
	return helm.ReleaseObject{APIVersion: apiVersion, Kind: kind, Namespace: "proj", Name: name}
}

// mainNamespace is the main namespace as the manifest of a release holds it.
var mainNamespace = helm.ReleaseObject{APIVersion: "v1", Kind: "Namespace", Name: "proj"}

// helmTextRevision is a deployed revision a Helm text rendered, holding the main namespace and the
// objects.
func helmTextRevision(objects ...helm.ReleaseObject) *helm.ProjectRelease {
	return &helm.ProjectRelease{Revision: 7, HelmText: true, Objects: append([]helm.ReleaseObject{mainNamespace}, objects...)}
}

// switchTo is the change of an upgrade from the deployed revision to a native render of the targets.
func switchTo(deployed *helm.ProjectRelease, targets ...helm.ReleaseObject) helm.ReleaseChange {
	return helm.ReleaseChange{Deployed: deployed, Targets: append([]helm.ReleaseObject{mainNamespace}, targets...), Native: true}
}

func exists(t *testing.T, c client.Client, object client.Object) bool {
	t.Helper()
	err := c.Get(context.Background(), client.ObjectKeyFromObject(object), object)
	require.True(t, err == nil || apierrors.IsNotFound(err), "get %s: %v", object.GetName(), err)
	return err == nil
}

// assertLeft checks that the object is left in place for good: the upgrade keeps it, it is not the
// module's any more, and it is listed by the label.
func assertLeft(t *testing.T, c client.Client, object client.Object) {
	t.Helper()
	require.True(t, exists(t, c, object), "%s is there", object.GetName())
	assert.Equal(t, helmResourcePolicyKeep, object.GetAnnotations()[helmResourcePolicy], object.GetName())
	assert.NotContains(t, object.GetLabels(), v1alpha3.ResourceLabelHeritage, object.GetName())
	assert.Equal(t, "true", object.GetLabels()[v1alpha3.ResourceLabelKeptFromHelmTemplate], object.GetName())
	assert.Equal(t, "proj", object.GetLabels()[v1alpha3.ResourceLabelProject], "the project label stays")
	assert.Equal(t, "proj", object.GetAnnotations()[helm.ResourceAnnotationReleaseName], "the release can take it over again")
}

// assertUntouched checks that the object is as the release left it.
func assertUntouched(t *testing.T, c client.Client, object client.Object) {
	t.Helper()
	require.True(t, exists(t, c, object), "%s is there", object.GetName())
	assert.Equal(t, releaseObjectLabels(), object.GetLabels(), object.GetName())
	assert.Equal(t, releaseAnnotations(nil), object.GetAnnotations(), object.GetName())
}

func keptObjectsSeries(project string) float64 {
	if promtestutil.CollectAndCount(keptObjectsProjects) == 0 {
		return 0
	}
	return promtestutil.ToFloat64(keptObjectsProjects.WithLabelValues(project))
}

// storedProject is the project as the fake cluster stores it.
func storedProject(t *testing.T, c client.Client) *v1alpha3.Project {
	t.Helper()
	stored := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "proj"}, stored))
	return stored
}

// A switch from a Helm text to structured fields leaves in place every object of the release the new
// render does not hold and the standard fields do not replace: a workload with its volume, a rule of
// a subject that is no administrator, a quota spec.quota does not cover, a binding, a cluster-scoped
// object. What the new render holds stays the release's, a namespace is never left, an unmanaged
// object is not the release's to leave, and an object already gone is nothing to list.
func TestManager_PrepareUpgradeLeavesWhatASwitchDrops(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	database := &appsv1.StatefulSet{ObjectMeta: releaseMeta("database")}
	volume := &corev1.PersistentVolumeClaim{ObjectMeta: releaseMeta("data")}
	binding := &rbacv1.RoleBinding{
		ObjectMeta: releaseMeta("ci"),
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:namespace:manager"},
	}
	reader := &rbacv1.ClusterRole{ObjectMeta: releaseMeta("proj-reader")}
	reader.Namespace = ""
	isolated := &networkingv1.NetworkPolicy{ObjectMeta: releaseMeta("isolated")}
	once := &corev1.ConfigMap{ObjectMeta: releaseMeta("once")}
	once.Labels = map[string]string{v1alpha3.ResourceLabelUnmanaged: "true", v1alpha3.ResourceLabelProject: "proj"}
	onceEmpty := &corev1.ConfigMap{ObjectMeta: releaseMeta("once-empty")}
	onceEmpty.Labels = map[string]string{v1alpha3.ResourceLabelUnmanaged: "", v1alpha3.ResourceLabelProject: "proj"}
	rule := liveRule("developers")
	quota := liveQuota(corev1.ResourceList{"pods": resource.MustParse("10")})
	m, c := newTransitionManager(t, project, projectNamespace(), database, volume, binding, reader, isolated, once, onceEmpty, rule, quota)
	t.Cleanup(func() { m.Forget("proj") })

	deployed := helmTextRevision(
		ref("apps/v1", "StatefulSet", "database"),
		ref("v1", "PersistentVolumeClaim", "data"),
		ref("rbac.authorization.k8s.io/v1", "RoleBinding", "ci"),
		// the manifest pins a cluster-scoped object to the namespace of the project as well
		ref("rbac.authorization.k8s.io/v1", "ClusterRole", "proj-reader"),
		ref("networking.k8s.io/v1", "NetworkPolicy", "isolated"),
		ref("v1", "ConfigMap", "once"),
		ref("v1", "ConfigMap", "once-empty"),
		ref("v1", "ConfigMap", "gone"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "developers"),
		ref("v1", "ResourceQuota", "all-pods"),
	)
	require.NoError(t, m.prepareUpgrade(ctx, project, switchTo(deployed, ref("networking.k8s.io/v1", "NetworkPolicy", "isolated"))))

	for _, object := range []client.Object{database, volume, binding, reader, rule, quota} {
		assertLeft(t, c, object)
	}
	assertUntouched(t, c, isolated)
	require.True(t, exists(t, c, once))
	assert.NotContains(t, once.Labels, v1alpha3.ResourceLabelKeptFromHelmTemplate, "an unmanaged object is not the release's")
	require.True(t, exists(t, c, onceEmpty))
	assert.NotContains(t, onceEmpty.Labels, v1alpha3.ResourceLabelKeptFromHelmTemplate, "the label makes it unmanaged whatever its value")
	namespace := projectNamespace()
	require.True(t, exists(t, c, namespace))
	assert.Equal(t, releaseObjectLabels(), namespace.Labels, "a namespace is never left")

	expected := []v1alpha3.KeptObject{
		{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole", Name: "proj-reader"},
		{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "developers"},
		{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: "proj", Name: "data"},
		{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods"},
		{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding", Namespace: "proj", Name: "ci"},
		{APIVersion: "apps/v1", Kind: "StatefulSet", Namespace: "proj", Name: "database"},
	}
	assert.Equal(t, expected, storedProject(t, c).Status.KeptObjects, "the list is in the stored status before the upgrade")
	assert.Equal(t, expected, project.Status.KeptObjects, "and on the project the reconcile goes on with")
	assert.InDelta(t, 6, keptObjectsSeries("proj"), 0)

	// A second pass, after a failed upgrade, changes nothing and lists nothing twice.
	again := storedProject(t, c)
	require.NoError(t, m.prepareUpgrade(ctx, again, switchTo(deployed, ref("networking.k8s.io/v1", "NetworkPolicy", "isolated"))))
	assert.Equal(t, expected, storedProject(t, c).Status.KeptObjects)
}

// Only a switch from a Helm text to structured fields leaves objects: an edit of a Helm text, a move
// to another one and an upgrade of a native release delete what they drop, as before.
func TestManager_PrepareUpgradeLeavesNothingWithoutASwitch(t *testing.T) {
	deployed := helmTextRevision(ref("v1", "ConfigMap", "settings"))
	native := &helm.ProjectRelease{Revision: 3, Objects: deployed.Objects}
	tests := []struct {
		name      string
		change    helm.ReleaseChange
		namespace bool
	}{
		{name: "an edit of a Helm text", change: helm.ReleaseChange{Deployed: deployed, Targets: []helm.ReleaseObject{mainNamespace}}, namespace: true},
		{name: "an upgrade of a native release", change: switchTo(native), namespace: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &corev1.ConfigMap{ObjectMeta: releaseMeta("settings")}
			objects := []client.Object{helmProject(), settings}
			if tt.namespace {
				objects = append(objects, projectNamespace())
			}
			m, c := newTransitionManager(t, objects...)
			project := helmProject()

			require.NoError(t, m.prepareUpgrade(context.Background(), project, tt.change))
			assertUntouched(t, c, settings)
			assert.Empty(t, storedProject(t, c).Status.KeptObjects)
		})
	}
}

// Without the main namespace a switch still leaves what survived elsewhere, a cluster-scoped object
// and the objects of an additional namespace, but hands nothing over: the standard fields cannot be
// created, and the upgrade that brings the namespace back must not wait for them.
func TestManager_PrepareUpgradeWithoutTheMainNamespace(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	project.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "proj-extra", Kind: v1alpha3.NamespaceKindAdditional}}
	reader := &rbacv1.ClusterRole{ObjectMeta: releaseMeta("proj-reader")}
	reader.Namespace = ""
	rule := liveRule("alice")
	rule.SetNamespace("proj-extra")
	m, c := newTransitionManager(t, project, reader, rule)
	t.Cleanup(func() { m.Forget("proj") })

	deployed := helmTextRevision(
		ref("rbac.authorization.k8s.io/v1", "ClusterRole", "proj-reader"),
		helm.ReleaseObject{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj-extra", Name: "alice"},
	)
	require.NoError(t, m.prepareUpgrade(ctx, project, switchTo(deployed)))

	assertLeft(t, c, reader)
	assertLeft(t, c, rule)
	assert.Nil(t, handoverCondition(t, c), "nothing is handed over")
	assert.False(t, exists(t, c, &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}}))
}

// A left object leaves the list once it is deleted or loses its label; the project looks again while
// something is left, and the series follows the list. An object the release holds again is the
// release's once more, a custom resource too, which Helm does not label back: it loses the label, gets
// the heritage label, and keeps the keep annotation only when the manifest has it, so the release
// deletes it when the template stops rendering it. A left object that a rollback labelled back is not
// the release's and loses the heritage label again.
func TestManager_RefreshKeptObjects(t *testing.T) {
	ctx := context.Background()
	left := map[string]string{v1alpha3.ResourceLabelKeptFromHelmTemplate: "true", v1alpha3.ResourceLabelProject: "proj"}
	keep := map[string]string{helmResourcePolicy: helmResourcePolicyKeep}
	labelled := func(name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: name,
			Labels: maps.Clone(left), Annotations: releaseAnnotations(keep)}}
	}
	unlabelled := labelled("taken-over")
	unlabelled.Labels = nil
	rendered := labelled("rendered-again")
	rendered.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	ownKeep := labelled("own-keep")
	relabelled := labelled("relabelled")
	relabelled.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	rule := adminRule("team-admins", subject("Group", "team-admins"))
	rule.SetLabels(maps.Clone(left))
	rule.SetAnnotations(releaseAnnotations(keep))
	kept := func(name string) v1alpha3.KeptObject {
		return v1alpha3.KeptObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: name}
	}
	project := helmProject()
	project.Status.KeptObjects = []v1alpha3.KeptObject{
		{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "team-admins"},
		kept("gone"), kept("own-keep"), kept("relabelled"), kept("rendered-again"), kept("still"), kept("taken-over"),
	}
	m, c := newTransitionManager(t, project, labelled("still"), unlabelled, rendered, ownKeep, relabelled, rule)
	held := func(object helm.ReleaseObject, keep bool) helm.ReleaseObject {
		object.Heritage, object.Keep = true, keep
		return object
	}
	m.helmClient = &fakeHelmClient{current: helmTextRevision(
		held(ref("v1", "ConfigMap", "rendered-again"), false),
		held(ref("v1", "ConfigMap", "own-keep"), true),
		held(ref("deckhouse.io/v1alpha1", "AuthorizationRule", "team-admins"), false),
	)}
	t.Cleanup(func() { m.Forget("proj") })

	recheck, err := m.refreshKeptObjects(ctx, project)
	require.NoError(t, err)
	assert.Equal(t, leftObjectsRecheck, recheck)
	assert.Equal(t, []v1alpha3.KeptObject{kept("relabelled"), kept("still")}, project.Status.KeptObjects)
	assert.InDelta(t, 2, keptObjectsSeries("proj"), 0)

	for _, object := range []client.Object{rendered, ownKeep, liveRule("team-admins")} {
		require.True(t, exists(t, c, object))
		assert.NotContains(t, object.GetLabels(), v1alpha3.ResourceLabelKeptFromHelmTemplate, "the release has %s back", object.GetName())
		assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, object.GetLabels()[v1alpha3.ResourceLabelHeritage], object.GetName())
	}
	assert.NotContains(t, rendered.Annotations, helmResourcePolicy, "and deletes it when the template stops rendering it")
	assert.Equal(t, helmResourcePolicyKeep, ownKeep.Annotations[helmResourcePolicy], "a keep of the Helm text stays")
	require.True(t, exists(t, c, relabelled))
	assert.NotContains(t, relabelled.Labels, v1alpha3.ResourceLabelHeritage, "it is not the release's")
	assert.Equal(t, "true", relabelled.Labels[v1alpha3.ResourceLabelKeptFromHelmTemplate])

	// within a minute the list is not read again, whatever wakes the project
	require.NoError(t, c.Delete(ctx, labelled("still")))
	recheck, err = m.refreshKeptObjects(ctx, project)
	require.NoError(t, err)
	assert.Equal(t, leftObjectsRecheck, recheck)
	assert.Equal(t, []v1alpha3.KeptObject{kept("relabelled"), kept("still")}, project.Status.KeptObjects)

	m.keptChecks.Delete("proj") // a minute later
	project.Status.KeptObjects = []v1alpha3.KeptObject{kept("gone")}
	recheck, err = m.refreshKeptObjects(ctx, project)
	require.NoError(t, err)
	assert.Zero(t, recheck)
	assert.Empty(t, project.Status.KeptObjects)
	assert.Zero(t, keptObjectsSeries("proj"))
}

// A reconcile of a project switched from a Helm text runs the upgrade with the preparation: the
// objects the switch drops are left and listed in the stored status, and the project looks again at
// them later. A deleted project drops its series.
func TestHandle_LeavesWhatASwitchDrops(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	project.Spec.ProjectTemplateName = "tmpl"
	settings := &corev1.ConfigMap{ObjectMeta: releaseMeta("settings")}
	m, c := newTransitionManager(t, project, structuredTemplate(), projectNamespace(), settings)
	change := switchTo(helmTextRevision(ref("v1", "ConfigMap", "settings")))
	m.helmClient = &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, change: &change}
	t.Cleanup(func() { m.Forget("proj") })

	result, err := m.Handle(ctx, project)
	require.NoError(t, err)
	assertLeft(t, c, settings)
	stored := storedProject(t, c)
	assert.Equal(t, []v1alpha3.KeptObject{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"}}, stored.Status.KeptObjects)
	assert.Equal(t, v1alpha3.ProjectStateDeployed, stored.Status.State)
	assert.Equal(t, leftObjectsRecheck, result.RequeueAfter)
	assert.InDelta(t, 1, keptObjectsSeries("proj"), 0)

	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, project))
	require.NoError(t, c.Delete(ctx, project))
	_, err = m.Delete(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Zero(t, keptObjectsSeries("proj"), "a deleted project drops its series")
}

// A deleted project takes with it the cluster-scoped objects a switch left that still carry the
// label: the uninstall of the release would have deleted them, and they are in no namespace that goes.
// One whose label was removed is its owners', an object of another project is not this one's, and a
// namespaced one goes with its namespace.
func TestManager_DeleteTakesTheLeftClusterObjects(t *testing.T) {
	ctx := context.Background()
	left := func(name string) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name,
			Labels:      map[string]string{v1alpha3.ResourceLabelKeptFromHelmTemplate: "true", v1alpha3.ResourceLabelProject: "proj"},
			Annotations: releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep})}}
	}
	reader := left("proj-reader")
	own := left("proj-own")
	delete(own.Labels, v1alpha3.ResourceLabelKeptFromHelmTemplate)
	foreign := left("other-reader")
	foreign.Labels[v1alpha3.ResourceLabelProject] = "other"
	foreign.Annotations[helm.ResourceAnnotationReleaseName] = "other"
	settings := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: "settings",
		Labels: map[string]string{v1alpha3.ResourceLabelKeptFromHelmTemplate: "true", v1alpha3.ResourceLabelProject: "proj"}}}
	role := func(name string) v1alpha3.KeptObject {
		return v1alpha3.KeptObject{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole", Name: name}
	}
	project := helmProject()
	project.Status.KeptObjects = []v1alpha3.KeptObject{
		role("other-reader"), role("proj-own"), role("proj-reader"), role("gone"),
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"},
	}
	m, c := newTransitionManager(t, project, projectNamespace(), reader, own, foreign, settings)
	m.helmClient = &fakeHelmClient{}
	t.Cleanup(func() { m.Forget("proj") })

	_, err := m.Delete(ctx, project)
	require.NoError(t, err)
	assert.False(t, exists(t, c, reader), "it goes with the project")
	assert.True(t, exists(t, c, own), "its owners keep it")
	assert.True(t, exists(t, c, foreign), "it is not this project's")
	assert.True(t, exists(t, c, settings), "it goes with the namespace")
}

// subject is an entry of spec.subjects of an AuthorizationRule.
func subject(kind, name string) map[string]any {
	return map[string]any{"kind": kind, "name": name}
}

// adminRule is a rule with the Admin access level for the subjects, as a Helm text spells it.
func adminRule(name string, subjects ...any) *unstructured.Unstructured {
	return withSpec(liveRule(name), map[string]any{"accessLevel": "Admin", "subjects": subjects})
}

// The standard fields replace an AuthorizationRule only when it grants nothing but the Admin access
// level to User and Group subjects that are all administrators, current or of the revision, and a
// ResourceQuota of the main namespace only when it has no scopes and spec.quota covers every limit.
// The rules a Commander-like text renders per role from its own parameter grant to other subjects
// and are not replaced, and neither is an all-pods quota spec.quota does not cover.
func TestReplacedByStandardFields(t *testing.T) {
	project := helmProject()
	project.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("2"), "limits.memory": resource.MustParse("4Gi")}
	// bob was an administrator when the revision was rendered, admins is still named by the parameters
	project.Spec.Parameters = map[string]any{"administrators": []any{map[string]any{"subject": "Group", "name": "admins", "accessLevel": "Editor"}}}
	deployed := helmTextRevision()
	deployed.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "User", Name: "bob"}, {Kind: "Group", Name: "admins"}}
	administrators := handoverAdministrators(project, deployed)

	foreign := liveRule("alice")
	foreign.SetAnnotations(map[string]string{helm.ResourceAnnotationReleaseName: "other"})
	handMade := liveRule("alice")
	handMade.SetLabels(map[string]string{v1alpha3.ResourceLabelProject: "proj"})
	quota := func(hard corev1.ResourceList, change func(*corev1.ResourceQuota)) *unstructured.Unstructured {
		typed := liveQuota(hard)
		if change != nil {
			change(typed)
		}
		object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(typed)
		require.NoError(t, err)
		live := &unstructured.Unstructured{Object: object}
		live.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ResourceQuota"))
		return live
	}
	inAdditional := quota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")}, nil)
	inAdditional.SetNamespace("proj-extra")

	rules := []struct {
		name     string
		live     *unstructured.Unstructured
		expected bool
	}{
		{name: "the rule of an administrator, as the former default Helm text rendered it", live: liveRule("alice"), expected: true},
		{name: "a rule of the administrators", live: adminRule("both", subject("User", "alice"), subject("User", "bob")), expected: true},
		{name: "the rule of an administrator removed since the revision", live: adminRule("bob", subject("User", "bob")), expected: true},
		{name: "the rule of an administrator the parameters still name", live: adminRule("admins", subject("Group", "admins"))},
		{name: "a Commander rule of a role", live: adminRule("team-admins", subject("Group", "team-admins"))},
		{name: "a rule of an administrator and of a role", live: adminRule("mixed", subject("User", "alice"), subject("Group", "team-admins"))},
		{name: "another access level", live: withSpec(liveRule("alice"), map[string]any{"accessLevel": "Editor", "subjects": []any{subject("User", "alice")}})},
		{name: "port forwarding", live: withSpec(liveRule("alice"), map[string]any{"accessLevel": "Admin", "portForwarding": true, "subjects": []any{subject("User", "alice")}})},
		{name: "scaling", live: withSpec(liveRule("alice"), map[string]any{"accessLevel": "Admin", "allowScale": true, "subjects": []any{subject("User", "alice")}})},
		{name: "another field", live: withSpec(liveRule("alice"), map[string]any{"accessLevel": "Admin", "additionalRoles": []any{}, "subjects": []any{subject("User", "alice")}})},
		{name: "a ServiceAccount", live: adminRule("ci", map[string]any{"kind": "ServiceAccount", "name": "alice", "namespace": "proj"})},
		{name: "a subject with a namespace", live: adminRule("alice", map[string]any{"kind": "User", "name": "alice", "namespace": "proj"})},
		{name: "no subjects", live: adminRule("nobody")},
		{name: "a rule of another release", live: foreign},
		{name: "a rule made by hand", live: handMade},
	}
	for _, tt := range rules {
		t.Run(tt.name, func(t *testing.T) {
			key := releaseKey{group: authorizationRuleGVK.Group, kind: authorizationRuleGVK.Kind, namespace: "proj", name: tt.live.GetName()}
			assert.Equal(t, tt.expected, replacedByStandardFields(project, key, tt.live, administrators))
		})
	}

	quotas := []struct {
		name     string
		live     *unstructured.Unstructured
		expected bool
	}{
		{name: "the same limits", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("2000m"), "limits.memory": resource.MustParse("4096Mi")}, nil), expected: true},
		{name: "looser limits", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("3")}, nil), expected: true},
		{name: "no limits", live: quota(nil, nil), expected: true},
		{name: "a tighter limit", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("1")}, nil)},
		{name: "a limit spec.quota does not set", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("2"), "pods": resource.MustParse("10")}, nil)},
		{name: "scopes", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")}, func(q *corev1.ResourceQuota) {
			q.Spec.Scopes = []corev1.ResourceQuotaScope{corev1.ResourceQuotaScopeBestEffort}
		})},
		{name: "a scope selector", live: quota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")}, func(q *corev1.ResourceQuota) {
			q.Spec.ScopeSelector = &corev1.ScopeSelector{}
		})},
		{name: "an additional namespace", live: inAdditional},
	}
	for _, tt := range quotas {
		t.Run(tt.name, func(t *testing.T) {
			key := releaseKey{kind: "ResourceQuota", namespace: tt.live.GetNamespace(), name: tt.live.GetName()}
			assert.Equal(t, tt.expected, replacedByStandardFields(project, key, tt.live, administrators))
		})
	}

	config := quota(nil, nil)
	assert.False(t, replacedByStandardFields(project, releaseKey{kind: "ConfigMap", namespace: "proj", name: "all-pods"}, config, administrators),
		"nothing else is replaced")
}

// assertKeptForHandover checks that the object is kept for the handover: the upgrade keeps it, and
// the cleanup deletes it once its replacement works.
func assertKeptForHandover(t *testing.T, c client.Client, object client.Object) {
	t.Helper()
	require.True(t, exists(t, c, object), "%s is there", object.GetName())
	assert.Equal(t, helmResourcePolicyKeep, object.GetAnnotations()[helmResourcePolicy], object.GetName())
	assert.Equal(t, "true", object.GetAnnotations()[keptByHandoverAnnotation], object.GetName())
	assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, object.GetLabels()[v1alpha3.ResourceLabelHeritage], object.GetName())
}

// handoverCondition is the HandoverPending condition of the stored project, nil without one.
func handoverCondition(t *testing.T, c client.Client) *v1alpha3.Condition {
	t.Helper()
	return conditionByType(storedProject(t, c), v1alpha3.ProjectConditionHandoverPending)
}

// handoverSeries is the value of the d8_multitenancy_project_handover_pending series of the project,
// zero without one.
func handoverSeries(project string) float64 {
	if promtestutil.CollectAndCount(handoverPendingProjects) == 0 {
		return 0
	}
	return promtestutil.ToFloat64(handoverPendingProjects.WithLabelValues(project))
}

// commanderRevision is a deployed revision of a Commander-like text: the namespace, the rule of each
// administrator and the all-pods quota of a copy of the default text, the per-role rules it renders
// from its projectRoleBindings parameter, and other objects.
func commanderRevision() *helm.ProjectRelease {
	deployed := helmTextRevision(
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "bob"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "team-admins"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "team-viewers"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "own-keep"),
		ref("v1", "ResourceQuota", "all-pods"),
		ref("v1", "ResourceQuota", "services"),
		ref("v1", "ConfigMap", "settings"),
	)
	deployed.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "User", Name: "bob"}}
	return deployed
}

// commanderObjects are the live objects of commanderRevision: bob has been removed from the
// administrators since, and the text itself asked Helm to keep one rule of alice.
func commanderObjects() []client.Object {
	ownKeep := adminRule("own-keep", subject("User", "alice"))
	ownKeep.SetAnnotations(releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep}))
	return []client.Object{
		liveRule("alice"),
		liveRule("bob"),
		adminRule("team-admins", subject("Group", "team-admins")),
		withSpec(liveRule("team-viewers"), map[string]any{"accessLevel": "User", "subjects": []any{subject("Group", "team-viewers")}}),
		ownKeep,
		liveQuota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")}),
		&corev1.ResourceQuota{ObjectMeta: releaseMeta("services"), Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{"services.loadbalancers": resource.MustParse("1")}}},
		&corev1.ConfigMap{ObjectMeta: releaseMeta("settings")},
	}
}

// A switch of a Commander-like release to structured fields keeps for the handover the rule of the
// administrator and the quota the standard fields replace, and applies the standard fields before the
// upgrade. The rule of a removed administrator is left to the upgrade to delete. The per-role rules,
// a quota spec.quota does not cover, a rule the text keeps itself and the other objects are left in
// place for good: none of them is ever deleted.
func TestManager_PrepareUpgradeHandsOverASwitch(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	m, c := newTransitionManager(t, append(commanderObjects(), project, projectNamespace())...)
	t.Cleanup(func() { m.Forget("proj") })

	require.NoError(t, m.prepareUpgrade(ctx, project, switchTo(commanderRevision())))

	assertKeptForHandover(t, c, liveRule("alice"))
	assertKeptForHandover(t, c, liveQuota(nil))
	assertUntouched(t, c, liveRule("bob"))
	for _, name := range []string{"team-admins", "team-viewers", "own-keep"} {
		assertLeft(t, c, liveRule(name))
	}
	assertLeft(t, c, &corev1.ResourceQuota{ObjectMeta: releaseMeta("services")})
	assertLeft(t, c, &corev1.ConfigMap{ObjectMeta: releaseMeta("settings")})

	record := handoverCondition(t, c)
	require.NotNil(t, record, "the handover is recorded in the status before the upgrade")
	assert.Equal(t, corev1.ConditionTrue, record.Status)
	assert.Equal(t, handoverStartedMessage(), record.Message)
	assert.True(t, project.IsConditionTrue(v1alpha3.ProjectConditionHandoverPending), "and on the project the reconcile goes on with")
	assert.Zero(t, handoverSeries("proj"), "a handover that has just begun raises no series")
	assert.Len(t, storedProject(t, c).Status.KeptObjects, 5)

	binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}}
	assert.True(t, exists(t, c, binding), "the standard fields exist before the upgrade")
	assert.True(t, exists(t, c, &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectQuotaName}}))
}

// An edit of a Helm text that removes the rules and the quota of the administrators hands them over
// the same way. Everything else it drops is deleted by the upgrade as before, the per-role rules
// included: the text says what the project holds. An edit that drops no rule and no quota prepares
// nothing.
func TestManager_PrepareUpgradeHandsOverAHelmEdit(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	m, c := newTransitionManager(t, append(commanderObjects(), project, projectNamespace())...)
	t.Cleanup(func() { m.Forget("proj") })

	edit := helm.ReleaseChange{Deployed: commanderRevision(), Targets: []helm.ReleaseObject{mainNamespace}}
	require.NoError(t, m.prepareUpgrade(ctx, project, edit))

	assertKeptForHandover(t, c, liveRule("alice"))
	assertKeptForHandover(t, c, liveQuota(nil))
	for _, name := range []string{"bob", "team-admins", "team-viewers"} {
		assertUntouched(t, c, liveRule(name))
	}
	assertUntouched(t, c, &corev1.ConfigMap{ObjectMeta: releaseMeta("settings")})
	assert.Empty(t, storedProject(t, c).Status.KeptObjects)
	assert.NotNil(t, handoverCondition(t, c))

	t.Run("an edit that drops no rule and no quota", func(t *testing.T) {
		project := helmProject()
		m, c := newTransitionManager(t, append(commanderObjects(), project, projectNamespace())...)
		deployed := commanderRevision()
		targets := slices.DeleteFunc(slices.Clone(deployed.Objects), func(object helm.ReleaseObject) bool { return object.Kind == "ConfigMap" })

		require.NoError(t, m.prepareUpgrade(ctx, project, helm.ReleaseChange{Deployed: deployed, Targets: targets}))
		assertUntouched(t, c, liveRule("alice"))
		assert.Nil(t, handoverCondition(t, c))
		assert.False(t, exists(t, c, &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}}))
	})
}

// A pass that stops halfway, at a patch that fails, has already put the handover and the left objects
// on record: the cleanup and the next pass find what it marked. The next pass marks what is left.
func TestManager_PrepareUpgradeRecordsBeforeItMarks(t *testing.T) {
	ctx := context.Background()
	failing := true
	funcs := interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
		if failing && object.GetName() == "alice" {
			return apierrors.NewServiceUnavailable("the API server is restarting")
		}
		return c.Patch(ctx, object, patch, opts...)
	}}
	project := helmProject()
	m, c := newTransitionManagerWith(t, funcs, append(commanderObjects(), project, projectNamespace())...)
	t.Cleanup(func() { m.Forget("proj") })

	require.Error(t, m.prepareUpgrade(ctx, project, switchTo(commanderRevision())))
	record := handoverCondition(t, c)
	require.NotNil(t, record, "the handover is on record before the first mark")
	assert.Equal(t, corev1.ConditionTrue, record.Status)
	assert.Len(t, storedProject(t, c).Status.KeptObjects, 5, "and so are the objects the pass leaves")
	assertUntouched(t, c, liveRule("alice"))

	failing = false
	require.NoError(t, m.prepareUpgrade(ctx, storedProject(t, c), switchTo(commanderRevision())))
	assertKeptForHandover(t, c, liveRule("alice"))
	assertKeptForHandover(t, c, liveQuota(nil))
	assert.Len(t, storedProject(t, c).Status.KeptObjects, 5)
}

// A rule an earlier pass marked, whose administrators have all been removed since, loses the mark and
// the keep annotation, so the upgrade deletes it: a revocation is never delayed, whether or not the
// handover is on record. A marked object that the standard fields no longer replace goes with an
// edit of a Helm text, which deletes what it drops.
func TestManager_PrepareUpgradeUnmarksWhatTheUpgradeDeletes(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	project.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "bob"}}
	project.Spec.Quota = nil
	deployed := helmTextRevision(ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice"))
	deployed.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice"}}

	for _, tt := range []struct {
		name   string
		change helm.ReleaseChange
	}{
		{name: "a switch", change: switchTo(deployed)},
		{name: "an edit of a Helm text", change: helm.ReleaseChange{Deployed: deployed, Targets: []helm.ReleaseObject{mainNamespace}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, c := newTransitionManager(t, project.DeepCopy(), projectNamespace(), keptRule("alice", subject("User", "alice")))
			t.Cleanup(func() { m.Forget("proj") })

			require.NoError(t, m.prepareUpgrade(ctx, project.DeepCopy(), tt.change))
			rule := liveRule("alice")
			require.True(t, exists(t, c, rule))
			assert.Equal(t, releaseAnnotations(nil), rule.GetAnnotations(), "the upgrade deletes it")
			assert.Nil(t, handoverCondition(t, c), "nothing is handed over")
			assert.Empty(t, storedProject(t, c).Status.KeptObjects, "nor left")
		})
	}

	t.Run("a quota the standard fields no longer replace", func(t *testing.T) {
		project := helmProject()
		project.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("4")}
		m, c := newTransitionManager(t, project, projectNamespace(), keptQuota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")}))
		t.Cleanup(func() { m.Forget("proj") })

		edit := helm.ReleaseChange{Deployed: helmTextRevision(ref("v1", "ResourceQuota", "all-pods")), Targets: []helm.ReleaseObject{mainNamespace}}
		require.NoError(t, m.prepareUpgrade(ctx, project, edit))
		quota := liveQuota(nil)
		require.True(t, exists(t, c, quota))
		assert.Equal(t, releaseAnnotations(nil), quota.GetAnnotations())
	})
}

// handingOver is helmProject with a handover on record since the given time.
func handingOver(since time.Time) *v1alpha3.Project {
	project := helmProject()
	project.Status.Conditions = []v1alpha3.Condition{{
		Type:               v1alpha3.ProjectConditionHandoverPending,
		Status:             corev1.ConditionTrue,
		Message:            handoverStartedMessage(),
		LastTransitionTime: metav1.NewTime(since),
	}}
	return project
}

// keptRule and keptQuota are the rule of alice and the all-pods quota as a handover kept them.
func keptRule(name string, subjects ...any) *unstructured.Unstructured {
	rule := adminRule(name, subjects...)
	rule.SetAnnotations(releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep, keptByHandoverAnnotation: "true"}))
	return rule
}

func keptQuota(hard corev1.ResourceList) *corev1.ResourceQuota {
	quota := liveQuota(hard)
	quota.Annotations = releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep, keptByHandoverAnnotation: "true"})
	return quota
}

// standardFields are the replacements of the kept objects: d8-administrators fanned out for its
// generation since the given time, d8:project:admin with its aggregated rules, and d8-project-quota
// calculated.
type standardFields struct {
	readySince  time.Time
	notReady    bool
	staleStatus bool
	emptyRole   bool
	noBinding   bool
	uncounted   bool
	// staleSpec leaves out of the binding and the quota what the project has added since
	staleSpec bool
}

func (f standardFields) objects(project *v1alpha3.Project) []client.Object {
	var objects []client.Object
	if !f.noBinding {
		binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding, Generation: 2}}
		for _, administrator := range project.Spec.Administrators {
			binding.Spec.Subjects = append(binding.Spec.Subjects, rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: administrator.Kind, Name: administrator.Name})
		}
		if f.staleSpec {
			binding.Spec.Subjects = binding.Spec.Subjects[:len(binding.Spec.Subjects)-1]
		}
		binding.Status.ObservedGeneration = 2
		if f.staleStatus {
			binding.Status.ObservedGeneration = 1
		}
		status := corev1.ConditionTrue
		if f.notReady {
			status = corev1.ConditionFalse
		}
		binding.Status.Conditions = []v1alpha3.Condition{{Type: v1alpha3.ProjectRoleBindingConditionReady, Status: status, LastTransitionTime: metav1.NewTime(f.readySince)}}
		objects = append(objects, binding)
	}
	role := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName}}
	if !f.emptyRole {
		role.Rules = []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}}}
	}
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectQuotaName},
		Spec:       corev1.ResourceQuotaSpec{Hard: project.Spec.Quota.DeepCopy()},
	}
	if f.staleSpec {
		delete(quota.Spec.Hard, "limits.memory")
	}
	if !f.uncounted {
		quota.Status.Hard = project.Spec.Quota.DeepCopy()
	}
	return append(objects, role, quota)
}

// A kept rule or quota goes only once the standard field that replaces it works and the grace period
// has passed after that; until then the project looks again soon, and the condition says why. The
// rule of an administrator removed since goes at once. A kept object the release renders again is
// the release's once more, and a kept quota spec.quota no longer covers is left for good.
func TestManager_DeleteKeptObjects(t *testing.T) {
	ctx := context.Background()
	long := time.Now().Add(-time.Hour)
	recent := time.Now().Add(-time.Minute)
	covered := corev1.ResourceList{"requests.cpu": resource.MustParse("2")}

	waiting := []struct {
		name    string
		fields  standardFields
		message string
	}{
		{name: "no administrators binding", fields: standardFields{noBinding: true, readySince: long}, message: "ProjectRoleBinding does not exist"},
		{name: "a binding that is not Ready", fields: standardFields{notReady: true, readySince: long}, message: "is not Ready for its current generation"},
		{name: "a binding Ready for an older generation", fields: standardFields{staleStatus: true, readySince: long}, message: "is not Ready for its current generation"},
		{name: "a role without its aggregated rules", fields: standardFields{emptyRole: true, readySince: long}, message: "has no aggregated rules yet"},
		{name: "a quota that is not calculated", fields: standardFields{uncounted: true, readySince: long}, message: "ResourceQuota is not calculated"},
		// the reconcile has just written them, and the cache has not seen the write yet
		{name: "a binding and a quota without what the project added", fields: standardFields{staleSpec: true, readySince: long},
			message: "does not name the administrators of the project yet and the d8-project-quota ResourceQuota does not have the limits"},
	}
	for _, tt := range waiting {
		t.Run(tt.name, func(t *testing.T) {
			project := handingOver(long)
			project.Spec.Administrators = append(project.Spec.Administrators, v1alpha3.Administrator{Kind: "Group", Name: "admins"})
			project.Spec.Quota["limits.memory"] = resource.MustParse("4Gi")
			objects := append(tt.fields.objects(project), project, keptRule("alice", subject("User", "alice")), keptQuota(covered))
			m, c := newTransitionManager(t, objects...)
			fh := &fakeHelmClient{}
			m.helmClient = fh
			t.Cleanup(func() { m.Forget("proj") })

			recheck, err := m.deleteKeptObjects(ctx, project)
			require.NoError(t, err)
			assert.Equal(t, keptObjectsRecheck, recheck)
			assertKeptForHandover(t, c, liveRule("alice"))
			assertKeptForHandover(t, c, liveQuota(nil))
			record := handoverCondition(t, c)
			require.NotNil(t, record)
			assert.Contains(t, record.Message, tt.message)
			assert.InDelta(t, 1, handoverSeries("proj"), 0)
			assert.Zero(t, fh.currentCalls, "a handover that waits does not read the release")
		})
	}

	t.Run("within the grace period", func(t *testing.T) {
		project := handingOver(long)
		objects := append(standardFields{readySince: recent}.objects(project), project, keptRule("alice", subject("User", "alice")), keptQuota(covered))
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.InDelta(t, time.Minute, recheck, float64(5*time.Second), "the rest of the grace period after the binding became Ready")
		assertKeptForHandover(t, c, liveRule("alice"))
		assert.Contains(t, handoverCondition(t, c).Message, "deleted after "+recent.Add(handoverGrace).UTC().Format(time.RFC3339))
	})

	t.Run("a handover that has just begun", func(t *testing.T) {
		project := handingOver(recent)
		objects := append(standardFields{readySince: long}.objects(project), project, keptQuota(covered))
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.Positive(t, recheck, "the grace period counts from the start of the handover too")
		assertKeptForHandover(t, c, liveQuota(nil))
	})

	t.Run("the replacements work", func(t *testing.T) {
		project := handingOver(long)
		objects := append(standardFields{readySince: long}.objects(project), project,
			keptRule("alice", subject("User", "alice")), keptRule("both", subject("User", "alice"), subject("User", "bob")), keptQuota(covered))
		m, c := newTransitionManager(t, objects...)
		fh := &fakeHelmClient{}
		m.helmClient = fh
		recordHandoverSeries(t, m)

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.Zero(t, recheck)
		assert.Equal(t, 1, fh.currentCalls, "the release is read once, before the deletes")
		assert.False(t, exists(t, c, liveRule("alice")))
		assert.False(t, exists(t, c, liveRule("both")))
		assert.False(t, exists(t, c, liveQuota(nil)))
		assert.Nil(t, handoverCondition(t, c), "the record goes with the kept objects")
		assert.False(t, project.IsConditionTrue(v1alpha3.ProjectConditionHandoverPending))
		assert.Zero(t, handoverSeries("proj"))
	})

	t.Run("the rule of a removed administrator", func(t *testing.T) {
		project := handingOver(recent)
		objects := append(standardFields{noBinding: true}.objects(project), project,
			keptRule("bob", subject("User", "bob")), keptRule("alice", subject("User", "alice")))
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.Equal(t, keptObjectsRecheck, recheck)
		assert.False(t, exists(t, c, liveRule("bob")), "a revocation is never delayed")
		assertKeptForHandover(t, c, liveRule("alice"))
	})

	t.Run("a project without administrators and quota any more", func(t *testing.T) {
		project := handingOver(recent)
		project.Spec.Administrators = nil
		project.Spec.Quota = nil
		objects := []client.Object{project, keptRule("alice", subject("User", "alice")), keptQuota(nil)}
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		_, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.False(t, exists(t, c, liveRule("alice")))
		assert.True(t, exists(t, c, liveQuota(nil)), "a quota without limits waits for the grace period alone")
	})

	t.Run("what the release renders again and what is no longer replaced", func(t *testing.T) {
		project := handingOver(long)
		project.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("4")}
		objects := append(standardFields{readySince: long}.objects(project), project, keptRule("alice", subject("User", "alice")), keptQuota(covered))
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{current: helmTextRevision(ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice"))}
		t.Cleanup(func() { m.Forget("proj") })

		_, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		rule := liveRule("alice")
		require.True(t, exists(t, c, rule))
		assert.Equal(t, releaseAnnotations(nil), rule.GetAnnotations(), "the release manages it as before")
		assertLeft(t, c, liveQuota(nil))
		assert.NotContains(t, liveQuota(nil).Annotations, keptByHandoverAnnotation)
		assert.Equal(t, []v1alpha3.KeptObject{{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods"}}, storedProject(t, c).Status.KeptObjects)
		assert.Nil(t, handoverCondition(t, c))
	})

	t.Run("a kept rule the release renders again with keep", func(t *testing.T) {
		project := handingOver(long)
		objects := append(standardFields{readySince: long}.objects(project), project, keptRule("alice", subject("User", "alice")))
		m, c := newTransitionManager(t, objects...)
		rendered := ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice")
		rendered.Heritage, rendered.Keep = true, true
		m.helmClient = &fakeHelmClient{current: helmTextRevision(rendered)}
		t.Cleanup(func() { m.Forget("proj") })

		_, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		rule := liveRule("alice")
		require.True(t, exists(t, c, rule))
		assert.Equal(t, releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep}), rule.GetAnnotations(),
			"the keep the text sets stays, the mark goes")
		assert.Nil(t, handoverCondition(t, c))
	})

	t.Run("a rule kept in an additional namespace", func(t *testing.T) {
		project := handingOver(long)
		project.Status.Namespaces = []v1alpha3.NamespaceStatus{
			{Name: "proj", Kind: v1alpha3.NamespaceKindMain}, {Name: "proj-extra", Kind: v1alpha3.NamespaceKindAdditional},
		}
		extra := keptRule("alice", subject("User", "alice"))
		extra.SetNamespace("proj-extra")
		// a rule of another project is not this handover's, though it carries the label
		elsewhere := keptRule("alice", subject("User", "alice"))
		elsewhere.SetNamespace("other")
		objects := append(standardFields{readySince: long}.objects(project), project, extra, elsewhere)
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		_, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.False(t, exists(t, c, extra), "d8-administrators fans out into the additional namespace too")
		assert.True(t, exists(t, c, elsewhere))
		assert.Nil(t, handoverCondition(t, c))
	})

	t.Run("a quota left halfway is on record", func(t *testing.T) {
		project := handingOver(long)
		project.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("4")}
		funcs := interceptor.Funcs{Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return apierrors.NewServiceUnavailable("the API server is restarting")
		}}
		objects := append(standardFields{readySince: long}.objects(project), project, keptQuota(covered))
		m, c := newTransitionManagerWith(t, funcs, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		_, err := m.deleteKeptObjects(ctx, project)
		require.Error(t, err)
		assert.Equal(t, []v1alpha3.KeptObject{{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods"}},
			storedProject(t, c).Status.KeptObjects, "the list has it before it loses the mark")
		assert.NotNil(t, handoverCondition(t, c), "and the handover stays on record")
	})

	t.Run("no handover on record", func(t *testing.T) {
		project := helmProject()
		m, c := newTransitionManager(t, project, keptRule("alice", subject("User", "alice")))

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.Zero(t, recheck)
		assertKeptForHandover(t, c, liveRule("alice"))
	})
}

// A kept rule that names a current administrator and a removed one loses the removed one at once,
// when the upgrade drops it and while the handover waits: a revocation is never delayed.
func TestManager_HandoverNarrowsAKeptRule(t *testing.T) {
	ctx := context.Background()
	subjectsOf := func(t *testing.T, c client.Client, name string) []any {
		t.Helper()
		rule := liveRule(name)
		require.True(t, exists(t, c, rule))
		subjects, _, err := unstructured.NestedSlice(rule.Object, "spec", "subjects")
		require.NoError(t, err)
		return subjects
	}

	t.Run("when the upgrade drops it", func(t *testing.T) {
		project := helmProject()
		deployed := helmTextRevision(ref("deckhouse.io/v1alpha1", "AuthorizationRule", "both"))
		deployed.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "User", Name: "bob"}}
		m, c := newTransitionManager(t, project, projectNamespace(), adminRule("both", subject("User", "alice"), subject("User", "bob")))
		t.Cleanup(func() { m.Forget("proj") })

		require.NoError(t, m.prepareUpgrade(ctx, project, switchTo(deployed)))
		assertKeptForHandover(t, c, liveRule("both"))
		assert.Equal(t, []any{subject("User", "alice")}, subjectsOf(t, c, "both"))
	})

	t.Run("while the handover waits", func(t *testing.T) {
		project := handingOver(time.Now().Add(-time.Hour))
		objects := append(standardFields{notReady: true}.objects(project), project,
			keptRule("both", subject("User", "alice"), subject("User", "bob")))
		m, c := newTransitionManager(t, objects...)
		m.helmClient = &fakeHelmClient{}
		t.Cleanup(func() { m.Forget("proj") })

		recheck, err := m.deleteKeptObjects(ctx, project)
		require.NoError(t, err)
		assert.Equal(t, keptObjectsRecheck, recheck)
		assertKeptForHandover(t, c, liveRule("both"))
		assert.Equal(t, []any{subject("User", "alice")}, subjectsOf(t, c, "both"))
	})
}

// recordHandoverSeries publishes the series of the project, as a reconcile with a handover on record
// does, and drops it after the test.
func recordHandoverSeries(t *testing.T, m *Manager) {
	t.Helper()
	handoverPendingProjects.WithLabelValues("proj").Set(1)
	t.Cleanup(func() { m.Forget("proj") })
}

// A reconcile of a project switched from a Helm text runs the handover across reconciles: the first
// keeps the rule and the quota of the release and applies the standard fields, the condition outlives
// the reset of the conditions and a restart, and a later reconcile deletes the kept objects once the
// replacements work and the grace period has passed.
func TestHandle_HandsOverAcrossReconciles(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	project.Spec.ProjectTemplateName = "tmpl"
	quota := liveQuota(corev1.ResourceList{"requests.cpu": resource.MustParse("2")})
	m, c := newTransitionManager(t, project, structuredTemplate(), projectNamespace(), liveRule("alice"), quota,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName},
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}}}})
	change := switchTo(helmTextRevision(ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice"), ref("v1", "ResourceQuota", "all-pods")))
	fh := &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, change: &change}
	m.helmClient = fh
	t.Cleanup(func() { m.Forget("proj") })

	result, err := m.Handle(ctx, project)
	require.NoError(t, err)
	assert.Equal(t, keptObjectsRecheck, result.RequeueAfter, "the binding is not Ready yet")
	assertKeptForHandover(t, c, liveRule("alice"))
	assertKeptForHandover(t, c, liveQuota(nil))
	assert.Empty(t, storedProject(t, c).Status.KeptObjects, "the standard fields replace both")
	require.NotNil(t, handoverCondition(t, c))

	assert.Zero(t, handoverSeries("proj"), "a handover that has just begun raises no series")

	// a restart, once the handover has had the time to finish: the series is published again from the
	// record in the status
	m.Forget("proj")
	fh.change = nil
	stored := storedProject(t, c)
	for i := range stored.Status.Conditions {
		stored.Status.Conditions[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-handoverExpected))
	}
	require.NoError(t, c.Status().Update(ctx, stored))
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.InDelta(t, 1, handoverSeries("proj"), 0)
	require.NotNil(t, handoverCondition(t, c), "a reconcile keeps the record")

	// the ProjectRoleBinding controller fans the binding out, the quota controller counts the quota,
	// and the grace period passes
	binding := new(v1alpha3.ProjectRoleBinding)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, binding))
	binding.Status.ObservedGeneration = binding.Generation
	binding.Status.Conditions = []v1alpha3.Condition{{Type: v1alpha3.ProjectRoleBindingConditionReady, Status: corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
	require.NoError(t, c.Status().Update(ctx, binding))
	projectQuota := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectQuotaName}, projectQuota))
	projectQuota.Status.Hard = projectQuota.Spec.Hard.DeepCopy()
	require.NoError(t, c.Status().Update(ctx, projectQuota))
	stored = storedProject(t, c)
	for i := range stored.Status.Conditions {
		stored.Status.Conditions[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour))
	}
	require.NoError(t, c.Status().Update(ctx, stored))

	result, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	assert.False(t, exists(t, c, liveRule("alice")))
	assert.False(t, exists(t, c, liveQuota(nil)))
	assert.Nil(t, handoverCondition(t, c))
	assert.Zero(t, handoverSeries("proj"))
}

// The series follow the status as read, before any step that can stop the reconcile: a project whose
// template is gone still reports its left objects and a handover that has not finished, and a
// handover that has just begun reports nothing yet.
func TestHandle_PublishesTheSeriesBeforeItStops(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name     string
		since    time.Duration
		expected float64
	}{
		{name: "a handover that began long ago", since: time.Hour, expected: 1},
		{name: "a handover that has just begun", since: time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			project := handingOver(time.Now().Add(-tt.since))
			project.Spec.ProjectTemplateName = "gone"
			project.Status.KeptObjects = []v1alpha3.KeptObject{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"},
				{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: "proj", Name: "data"},
			}
			m, _ := newTransitionManager(t, project, projectNamespace())
			m.helmClient = &fakeHelmClient{}
			t.Cleanup(func() { m.Forget("proj") })

			_, err := m.Handle(ctx, project.DeepCopy())
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, handoverSeries("proj"), 0)
			assert.InDelta(t, 2, keptObjectsSeries("proj"), 0)
		})
	}
}

// An all-pods quota with a limit spec.quota does not set is never deleted: the switch leaves it in
// place, and the handover of the rule of the administrator finishes without it. The per-role rules of
// a Commander-like text stay as well.
func TestHandle_NeverDeletesWhatTheStandardFieldsDoNotReplace(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	project.Spec.ProjectTemplateName = "tmpl"
	quota := liveQuota(corev1.ResourceList{"requests.cpu": resource.MustParse("2"), "limits.memory": resource.MustParse("4Gi")})
	roleRule := adminRule("team-admins", subject("Group", "team-admins"))
	m, c := newTransitionManager(t, project, structuredTemplate(), projectNamespace(), liveRule("alice"), roleRule, quota,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName},
			Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}}}})
	change := switchTo(helmTextRevision(
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "alice"),
		ref("deckhouse.io/v1alpha1", "AuthorizationRule", "team-admins"),
		ref("v1", "ResourceQuota", "all-pods"),
	))
	m.helmClient = &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, change: &change}
	t.Cleanup(func() { m.Forget("proj") })

	_, err := m.Handle(ctx, project)
	require.NoError(t, err)
	assertKeptForHandover(t, c, liveRule("alice"))
	assertLeft(t, c, liveQuota(nil))
	assertLeft(t, c, liveRule("team-admins"))

	binding := new(v1alpha3.ProjectRoleBinding)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, binding))
	binding.Status.ObservedGeneration = binding.Generation
	binding.Status.Conditions = []v1alpha3.Condition{{Type: v1alpha3.ProjectRoleBindingConditionReady, Status: corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
	require.NoError(t, c.Status().Update(ctx, binding))
	stored := storedProject(t, c)
	for i := range stored.Status.Conditions {
		stored.Status.Conditions[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour))
	}
	require.NoError(t, c.Status().Update(ctx, stored))

	result, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.False(t, exists(t, c, liveRule("alice")), "the handover finishes")
	assert.Nil(t, handoverCondition(t, c))
	assertLeft(t, c, liveQuota(nil))
	assertLeft(t, c, liveRule("team-admins"))
	assert.Equal(t, []v1alpha3.KeptObject{
		{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "team-admins"},
		{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods"},
	}, storedProject(t, c).Status.KeptObjects)
	assert.Equal(t, leftObjectsRecheck, result.RequeueAfter, "the left objects are looked at later")
}
