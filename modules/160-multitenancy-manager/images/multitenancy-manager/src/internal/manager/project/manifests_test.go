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
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

const manifestsUser = "system:multitenancy-manager:project:proj"

// manifestsCall is what a client of the user of a project asked the API server for.
type manifestsCall struct {
	user, verb, kind, namespace, name string
	fieldManager                      string
	force                             bool
}

// fakeManifestsClients hands out clients over the fake cluster that record what each user asks for,
// answer with refuse when it says so, and do a server-side apply, which the fake client does not
// support, the way it works for the fields an apply with force sets.
type fakeManifestsClients struct {
	cluster client.WithWatch
	// refuse, when set, is the answer of the API server to the verb on the object, nil for none.
	refuse func(verb string, object client.Object) error
	calls  []manifestsCall
	// made counts the clients handed out.
	made int
}

func (f *fakeManifestsClients) For(user string) (client.Client, error) {
	f.made++
	record := func(verb string, object client.Object, options *client.PatchOptions) error {
		call := manifestsCall{
			user: user, verb: verb, kind: object.GetObjectKind().GroupVersionKind().Kind,
			namespace: object.GetNamespace(), name: object.GetName(),
		}
		if options != nil {
			call.fieldManager = options.FieldManager
			call.force = options.Force != nil && *options.Force
		}
		f.calls = append(f.calls, call)
		if f.refuse == nil {
			return nil
		}
		return f.refuse(verb, object)
	}
	return interceptor.NewClient(f.cluster, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
			options := new(client.PatchOptions)
			options.ApplyOptions(opts)
			verb := "patch"
			if patch.Type() == types.ApplyPatchType {
				verb = "apply"
			}
			if err := record(verb, object, options); err != nil {
				return err
			}
			applied, ok := object.(*unstructured.Unstructured)
			if verb != "apply" || !ok {
				return c.Patch(ctx, object, patch, opts...)
			}
			return emulateApply(ctx, c, applied)
		},
		Delete: func(ctx context.Context, c client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
			if err := record("delete", object, nil); err != nil {
				return err
			}
			return c.Delete(ctx, object, opts...)
		},
	}), nil
}

// verbs lists the calls of the verb, as "user kind namespace/name".
func (f *fakeManifestsClients) verbs(verb string) []string {
	var calls []string
	for _, call := range f.calls {
		if call.verb == verb {
			calls = append(calls, call.user+" "+call.kind+" "+call.namespace+"/"+call.name)
		}
	}
	return calls
}

// emulateApply creates the applied object, or applies it to the stored one the way a server-side apply
// with force by the field manager of the manifests does for these tests: every field the applied
// object holds is set, a map key by key, and a field that this manager owns by an apply and that the
// applied object leaves out is removed. A field that only another manager owns stays, as on an API
// server. The owners are read from the managed fields of the stored object, which the emulation does
// not update.
func emulateApply(ctx context.Context, c client.Client, applied *unstructured.Unstructured) error {
	stored := new(unstructured.Unstructured)
	stored.SetGroupVersionKind(applied.GroupVersionKind())
	switch err := c.Get(ctx, client.ObjectKeyFromObject(applied), stored); {
	case apierrors.IsNotFound(err):
		return c.Create(ctx, applied)
	case err != nil:
		return err
	}
	for _, entry := range stored.GetManagedFields() {
		if entry.Manager != manifestsFieldManager || entry.Operation != metav1.ManagedFieldsOperationApply || entry.FieldsV1 == nil {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(entry.FieldsV1.Raw, &fields); err != nil {
			return err
		}
		for _, path := range ownedFieldPaths(fields, nil) {
			if _, found, _ := unstructured.NestedFieldNoCopy(applied.Object, path...); !found {
				unstructured.RemoveNestedField(stored.Object, path...)
			}
		}
	}
	mergeFields(stored.Object, applied.Object)
	return c.Update(ctx, stored)
}

// ownedFieldPaths lists the fields of a FieldsV1 set that are named by field names alone, the leaves
// and the ones owned as a whole ("."), which covers the maps of the objects of these tests.
func ownedFieldPaths(set map[string]any, prefix []string) [][]string {
	var paths [][]string
	for key, value := range set {
		name, ok := strings.CutPrefix(key, "f:")
		if !ok {
			continue
		}
		path := append(slices.Clone(prefix), name)
		children, _ := value.(map[string]any)
		nested := ownedFieldPaths(children, path)
		if _, whole := children["."]; whole || len(nested) == 0 {
			paths = append(paths, path)
		}
		paths = append(paths, nested...)
	}
	return paths
}

// mergeFields sets every field of src on dst, merging maps key by key.
func mergeFields(dst, src map[string]any) {
	for key, value := range src {
		nested, isMap := value.(map[string]any)
		existing, wasMap := dst[key].(map[string]any)
		if isMap && wasMap {
			mergeFields(existing, nested)
			continue
		}
		dst[key] = value
	}
}

// manifestsText renders a ConfigMap into the main namespace and a RoleBinding into the additional one.
const manifestsText = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team | quote }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: viewers
  namespace: proj-extra
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:project:viewer
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: viewers
`

// manifestsObjects are the entries of status.manifestObjects of manifestsText.
var manifestsObjects = []v1alpha3.ManifestObject{
	{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"},
	{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding", Namespace: "proj-extra", Name: "viewers"},
}

// manifestsCluster is the fake cluster of a project on a structured template with the manifests: its
// main namespace, an additional one, the binding of the user of the project as its controller leaves
// it once fanned out, and the role the binding grants with its aggregated rules.
func manifestsCluster(manifests string) []client.Object {
	template := structuredTemplate()
	template.Spec.Manifests = manifests
	template.Spec.ParametersSchema.OpenAPIV3Schema = map[string]any{
		"type":       "object",
		"properties": map[string]any{"team": map[string]any{"type": "string", "default": "platform"}},
	}
	return []client.Object{
		&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}, Spec: v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"}},
		template,
		projectNamespace(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj"}}},
		readyManifestsBinding(),
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
		},
	}
}

func readyManifestsBinding() *v1alpha3.ProjectRoleBinding {
	return &v1alpha3.ProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "proj", Name: v1alpha3.ProjectManifestsBinding, Generation: 1, Labels: standardFieldLabels("proj"),
		},
		Spec: v1alpha3.ProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: manifestsUser}},
			RoleRef:  v1alpha3.RoleRef{Kind: "ClusterRole", Name: v1alpha3.ProjectAdministratorsRoleName},
		},
		Status: v1alpha3.ProjectRoleBindingStatus{
			ObservedGeneration: 1,
			Conditions:         []v1alpha3.Condition{{Type: v1alpha3.ProjectRoleBindingConditionReady, Status: corev1.ConditionTrue}},
		},
	}
}

// newManifestsManager is a manager over the fake cluster with the objects, rendering manifests as the
// controller does and applying them through fake clients of the users.
func newManifestsManager(t *testing.T, objs ...client.Object) (*Manager, client.Client, *fakeManifestsClients) {
	t.Helper()
	m, c := newTransitionManager(t, objs...)
	cluster, ok := c.(client.WithWatch)
	require.True(t, ok, "the fake cluster watches")
	clients := &fakeManifestsClients{cluster: cluster}
	m.manifestsClients = clients
	renderer, err := helm.NewRenderOnly("../../../helmlib", logr.Discard())
	require.NoError(t, err)
	m.helmClient = &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, renderer: renderer}
	t.Cleanup(func() { m.Forget("proj") })
	return m, c, clients
}

func manifestsCondition(t *testing.T, c client.Client) *v1alpha3.Condition {
	t.Helper()
	for _, cond := range storedProject(t, c).Status.Conditions {
		if cond.Type == v1alpha3.ProjectConditionManifestsApplied {
			return &cond
		}
	}
	return nil
}

func configMap(namespace, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

// manifestLabels are the labels of an object applied from the manifests of the project.
func manifestLabels() map[string]string {
	return map[string]string{v1alpha3.ResourceLabelProject: "proj", v1alpha3.ResourceLabelSource: v1alpha3.ResourceSourceManifests}
}

// The objects of the manifests are applied as the user of the project, with server-side apply and its
// own field manager, into the namespaces of the project. They get the labels of the project and of
// the manifests and no heritage, the project lists them, and an unchanged render is not applied again.
func TestHandle_AppliesManifestsAsTheUserOfTheProject(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, manifestsCluster(manifestsText)...)

	result, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	assert.Equal(t, manifestLabels(), settings.Labels, "the labels of the manifests and no heritage")
	assert.Equal(t, map[string]string{"team": "platform"}, settings.Data, "rendered with the defaults of the schema")
	viewers := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj-extra", Name: "viewers"}}
	require.True(t, exists(t, c, viewers), "an object goes to the additional namespace it names")
	assert.Equal(t, manifestLabels(), viewers.Labels)

	assert.Equal(t, []string{
		manifestsUser + " ConfigMap proj/settings",
		manifestsUser + " RoleBinding proj-extra/viewers",
	}, clients.verbs("apply"), "applied as the user of the project, in the order of the render")
	for _, call := range clients.calls {
		assert.Equal(t, manifestsFieldManager, call.fieldManager)
		assert.True(t, call.force, "the apply takes over the fields of other managers")
	}

	stored := storedProject(t, c)
	assert.Equal(t, manifestsObjects, stored.Status.ManifestObjects)
	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Equal(t, corev1.ConditionTrue, condition.Status)
	assert.Equal(t, v1alpha3.ProjectStateDeployed, stored.Status.State)
	assert.Equal(t, manifestsReapply, result.RequeueAfter, "the objects are put back after a while")

	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Len(t, clients.verbs("apply"), 2, "an unchanged render is not applied again at once")
	assert.Equal(t, 1, clients.made, "nor is a client of the user made for it")
	assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
}

// An apply that a refusal stops halfway leaves some objects as the new render has them. A template
// that goes back to the render applied before is applied again at once, and not taken for applied.
func TestHandle_ReappliesARenderAFailedApplyWentBackFrom(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, manifestsCluster(manifestsText)...)
	setTeam := func(team any) {
		t.Helper()
		project := storedProject(t, c)
		project.Spec.Parameters = map[string]any{"team": team}
		if team == nil {
			project.Spec.Parameters = nil
		}
		require.NoError(t, c.Update(ctx, project))
	}
	team := func() string {
		t.Helper()
		settings := configMap("proj", "settings")
		require.True(t, exists(t, c, settings))
		return settings.Data["team"]
	}

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	require.Equal(t, "platform", team())

	setTeam("other")
	clients.refuse = func(verb string, object client.Object) error {
		if verb == "apply" && object.GetName() == "viewers" {
			return apierrors.NewForbidden(schema.GroupResource{Group: rbacv1.GroupName, Resource: "rolebindings"}, "viewers", errors.New("denied"))
		}
		return nil
	}
	_, err = m.Handle(ctx, storedProject(t, c))
	require.Error(t, err)
	require.Equal(t, "other", team(), "the first object got the new render before the refusal")

	setTeam(nil)
	clients.refuse = nil
	applies := len(clients.verbs("apply"))
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Len(t, clients.verbs("apply"), applies+2, "the render applied before the failed apply is applied again")
	assert.Equal(t, "platform", team())
	assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
}

// Nothing is applied before the binding of the user grants it the role: the controller makes the
// binding, and the condition says what the objects wait for.
func TestHandle_ManifestsWaitForTheirBinding(t *testing.T) {
	ctx := context.Background()

	t.Run("a binding that is not there yet", func(t *testing.T) {
		objs := manifestsCluster(manifestsText)
		objs = objs[:len(objs)-2]
		objs = append(objs, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
		})
		m, c, clients := newManifestsManager(t, objs...)

		result, err := m.Handle(ctx, storedProject(t, c))
		require.NoError(t, err)

		binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectManifestsBinding}}
		require.True(t, exists(t, c, binding), "the controller binds the user")
		assert.Equal(t, []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: manifestsUser}}, binding.Spec.Subjects)
		assert.Equal(t, v1alpha3.RoleRef{Kind: "ClusterRole", Name: v1alpha3.ProjectAdministratorsRoleName}, binding.Spec.RoleRef)
		assert.Equal(t, v1alpha3.ManagedByController, binding.Labels[v1alpha3.ResourceLabelManagedBy], "nobody else may change it")

		assert.Empty(t, clients.calls, "nothing is applied before the binding works")
		assert.False(t, exists(t, c, configMap("proj", "settings")))
		condition := manifestsCondition(t, c)
		require.NotNil(t, condition)
		assert.Equal(t, corev1.ConditionFalse, condition.Status)
		assert.Contains(t, condition.Message, "the d8-manifests ProjectRoleBinding that grants it the d8:project:admin role is Ready for its current generation")
		assert.Contains(t, condition.Message, manifestsUser)
		assert.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State, "waiting is no error")
		assert.Equal(t, manifestsBindingRecheck, result.RequeueAfter)
	})

	t.Run("a role without its aggregated rules", func(t *testing.T) {
		objs := manifestsCluster(manifestsText)
		objs[len(objs)-1] = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsRoleName}}
		m, c, clients := newManifestsManager(t, objs...)

		_, err := m.Handle(ctx, storedProject(t, c))
		require.NoError(t, err)
		assert.Empty(t, clients.calls)
		assert.Contains(t, manifestsCondition(t, c).Message, "the d8:project:admin ClusterRole has its aggregated rules")
	})
}

// A refusal of the API server, such as a 403 for what the role of the user does not allow, is named in
// the condition with its code, puts the project in Error and is retried. The other objects are
// applied, and nothing is deleted while one is refused.
func TestHandle_ManifestsNameA403(t *testing.T) {
	ctx := context.Background()
	stale := configMap("proj", "stale")
	stale.Labels = manifestLabels()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status:     v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "stale"}}},
	}
	objs := append(manifestsCluster(manifestsText)[1:], project, stale)
	m, c, clients := newManifestsManager(t, objs...)
	clients.refuse = func(verb string, object client.Object) error {
		if verb == "apply" && object.GetName() == "viewers" {
			return apierrors.NewForbidden(schema.GroupResource{Group: rbacv1.GroupName, Resource: "rolebindings"}, "viewers",
				errors.New(`User "`+manifestsUser+`" cannot patch resource "rolebindings" in API group "rbac.authorization.k8s.io" in the namespace "proj-extra"`))
		}
		return nil
	}

	_, err := m.Handle(ctx, storedProject(t, c))
	require.Error(t, err, "a refusal is retried")

	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Equal(t, corev1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, "as the "+manifestsUser+" user")
	assert.Contains(t, condition.Message, "RoleBinding proj-extra/viewers: 403 Forbidden: ")
	assert.Contains(t, condition.Message, `cannot patch resource "rolebindings"`)
	stored := storedProject(t, c)
	assert.Equal(t, v1alpha3.ProjectStateError, stored.Status.State)

	assert.True(t, exists(t, c, configMap("proj", "settings")), "the other objects are applied")
	assert.True(t, exists(t, c, stale), "nothing is deleted while an object is refused")
	assert.Equal(t, sortManifestObjects(append([]v1alpha3.ManifestObject{project.Status.ManifestObjects[0]}, manifestsObjects...)), stored.Status.ManifestObjects,
		"what was to be applied is listed before the apply, next to what was applied before")
	assert.Empty(t, clients.verbs("delete"))
}

// An object the manifests no longer render is deleted as the user of the project. One that lost the
// labels of the manifests is somebody else's and stays, as does one in a namespace that left the
// project, and both leave the list with the ones already gone.
func TestHandle_PrunesWhatTheManifestsNoLongerRender(t *testing.T) {
	ctx := context.Background()
	stale := configMap("proj", "stale")
	stale.Labels = manifestLabels()
	taken := configMap("proj", "taken")
	taken.Labels = map[string]string{v1alpha3.ResourceLabelProject: "proj"}
	elsewhere := configMap("former", "elsewhere")
	elsewhere.Labels = manifestLabels()
	listed := func(namespace, name string) v1alpha3.ManifestObject {
		return v1alpha3.ManifestObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: namespace, Name: name}
	}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status: v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{
			listed("proj", "settings"), listed("proj", "stale"), listed("proj", "taken"), listed("proj", "vanished"), listed("former", "elsewhere"),
		}},
	}
	objs := append(manifestsCluster(manifestsText)[1:], project, stale, taken, elsewhere)
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.False(t, exists(t, c, stale), "an object the manifests no longer render is deleted")
	assert.Equal(t, []string{manifestsUser + " ConfigMap proj/stale"}, clients.verbs("delete"), "by the user, and nothing else")
	assert.True(t, exists(t, c, taken), "an object without the labels of the manifests is not the project's any more")
	assert.True(t, exists(t, c, elsewhere), "an object in a namespace that left the project stays")
	assert.Equal(t, manifestsObjects, storedProject(t, c).Status.ManifestObjects)
	assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
}

// servedVersionsClient answers a read of a kind at a version its mapper does not serve with the NoMatch
// error of the API server, as the fake cluster, which knows no served versions, does not.
type servedVersionsClient struct {
	client.Client
	mapper meta.RESTMapper
}

func (c servedVersionsClient) RESTMapper() meta.RESTMapper { return c.mapper }

func (c servedVersionsClient) Get(ctx context.Context, key client.ObjectKey, object client.Object, opts ...client.GetOption) error {
	gvk := object.GetObjectKind().GroupVersionKind()
	if _, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
		return err
	}
	return c.Client.Get(ctx, key, object, opts...)
}

// An object applied with an API version that the API server does not serve any more, while it serves
// the kind at another one, is still there: it is read and deleted through the served version, not
// dropped from the list as gone. A kind that is not served at all leaves the list.
func TestPruneManifestObjectsThroughTheServedVersionOfAKind(t *testing.T) {
	ctx := context.Background()
	stale := configMap("proj", "stale")
	stale.Labels = manifestLabels()
	m, c, _ := newManifestsManager(t, append(manifestsCluster(""), stale)...)

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	actor := servedVersionsClient{Client: c, mapper: mapper}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Status: v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{
			{APIVersion: "v1beta9", Kind: "ConfigMap", Namespace: "proj", Name: "stale"},
			{APIVersion: "example.com/v1", Kind: "Widget", Namespace: "proj", Name: "gone"},
		}},
	}

	left, refusals := m.pruneManifestObjects(ctx, project, actor, nil)
	assert.Empty(t, refusals)
	assert.Empty(t, left)
	assert.False(t, exists(t, c, stale), "deleted through the version the kind is served at")
}

// A template that drops its manifests has the objects applied from them deleted, and then the binding
// of the user, which is to have no rights any more. Such a project has no condition of the manifests.
func TestHandle_DeletesTheManifestsATemplateDrops(t *testing.T) {
	ctx := context.Background()
	settings := configMap("proj", "settings")
	settings.Labels = manifestLabels()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status:     v1alpha3.ProjectStatus{ManifestObjects: manifestsObjects[:1]},
	}
	objs := append(manifestsCluster("")[1:], project, settings)
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.False(t, exists(t, c, settings))
	assert.Equal(t, []string{manifestsUser + " ConfigMap proj/settings"}, clients.verbs("delete"))
	assert.Empty(t, storedProject(t, c).Status.ManifestObjects)
	assert.False(t, exists(t, c, readyManifestsBinding()), "the user loses its binding once nothing of the manifests is left")
	assert.Nil(t, manifestsCondition(t, c))

	// a project of a template without manifests gets no binding at all
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.False(t, exists(t, c, readyManifestsBinding()))
	assert.Nil(t, manifestsCondition(t, c))
}

// An object the manifests cannot carry is refused before anything is applied or bound: the condition
// says why, the project is in Error, and nothing is retried until the template, the project or its
// namespaces change. A heritage label of another value than the platform's is the author's.
func TestHandle_RefusesWhatTheManifestsCannotCarry(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		manifests string
		reason    string
	}{
		{
			name:      "a cluster-scoped object",
			manifests: "apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: reader\n",
			reason:    "the ClusterRole 'reader' is cluster-scoped, and the manifests of a template create objects in the namespaces of the project only",
		},
		{
			name:      "a namespace",
			manifests: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: proj-more\n",
			reason:    "the Namespace 'proj-more' is cluster-scoped",
		},
		{
			name:      "an object in a namespace outside the project",
			manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: d8-system\n",
			reason:    "the ConfigMap 'settings' is in the 'd8-system' namespace, which is not a namespace of the project",
		},
		{
			name:      "an object that tries the heritage of the module",
			manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  labels:\n    heritage: multitenancy-manager\n",
			reason:    "has the heritage: multitenancy-manager label, which marks the objects that only the platform writes",
		},
		{
			name:      "an object that tries the heritage of Deckhouse",
			manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  labels:\n    heritage: deckhouse\n",
			reason:    "has the heritage: deckhouse label",
		},
		{
			name:      "an object without a name",
			manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  generateName: settings-\n",
			reason:    "a v1 ConfigMap has no metadata.name",
		},
		{
			name:      "an object twice",
			manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: proj\n",
			reason:    "they hold the ConfigMap 'proj/settings' twice",
		},
		{
			name:      "an object without a kind",
			manifests: "metadata:\n  name: settings\n",
			reason:    "a rendered object has no apiVersion or kind",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			objs := manifestsCluster(tc.manifests)
			objs = append(objs[:4], objs[5]) // no binding yet
			m, c, clients := newManifestsManager(t, objs...)

			result, err := m.Handle(ctx, storedProject(t, c))
			require.NoError(t, err, "a refusal is not retried")
			condition := manifestsCondition(t, c)
			require.NotNil(t, condition)
			assert.Equal(t, corev1.ConditionFalse, condition.Status)
			assert.Contains(t, condition.Message, "The manifests of the template are not applied, because ")
			assert.Contains(t, condition.Message, tc.reason)
			assert.Equal(t, v1alpha3.ProjectStateError, storedProject(t, c).Status.State)
			assert.Empty(t, clients.calls, "nothing is applied")
			assert.False(t, exists(t, c, readyManifestsBinding()), "nothing is bound")
			assert.Zero(t, result.RequeueAfter)
		})
	}

	t.Run("a heritage of the author", func(t *testing.T) {
		m, c, clients := newManifestsManager(t, manifestsCluster("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  labels:\n    heritage: Helm\n")...)
		_, err := m.Handle(ctx, storedProject(t, c))
		require.NoError(t, err)
		assert.Len(t, clients.verbs("apply"), 1)
		settings := configMap("proj", "settings")
		require.True(t, exists(t, c, settings))
		assert.Equal(t, "Helm", settings.Labels[v1alpha3.ResourceLabelHeritage])
	})
}

// A template that got past the webhook with both a Helm text and manifests renders from the text, and
// its manifests are not applied.
func TestHandle_NoManifestsNextToAHelmText(t *testing.T) {
	ctx := context.Background()
	objs := manifestsCluster(manifestsText)
	template := objs[1].(*v1alpha2.ProjectTemplate)
	template.Spec.NetworkPolicy = nil
	template.Spec.ResourcesTemplate = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: from-the-text\n"
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Contains(t, manifestsCondition(t, c).Message, "because the template renders from its resourcesTemplate")
	assert.Empty(t, clients.calls)
}

// A kind the API server does not serve may come with a module, so the project looks again.
func TestHandle_ManifestsWithAKindNotServed(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, manifestsCluster("apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n")...)

	result, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Contains(t, manifestsCondition(t, c).Message, "the API server does not serve a kind they hold")
	assert.Equal(t, v1alpha3.ProjectStateError, storedProject(t, c).Status.State)
	assert.Empty(t, clients.calls)
	assert.Equal(t, manifestsKindRecheck, result.RequeueAfter)
}

// The manifests are checked for a parameter that becomes structure, as a Helm text is (#21756).
func TestHandle_RefusesManifestsAParameterInjects(t *testing.T) {
	ctx := context.Background()
	objs := manifestsCluster("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\ndata:\n  team: {{ .parameters.team }}\n")
	objs[0].(*v1alpha3.Project).Spec.Parameters = map[string]any{
		"team": "evil\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata:\n  name: escalation\n",
	}
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Contains(t, condition.Message, "The manifests of the template do not render: ")
	assert.Contains(t, condition.Message, helm.ErrParameterInjection.Error())
	assert.Equal(t, v1alpha3.ProjectStateError, storedProject(t, c).Status.State)
	assert.Empty(t, clients.calls)
	assert.False(t, exists(t, c, configMap("proj", "settings")))
}

// An object that a switch from a Helm text left in place is taken over when the manifests render it:
// the fields that the release set go to the field manager of the manifests before the apply, so the
// apply leaves the object as the manifests render it, the marks of Helm and of a left object
// included, while a field that somebody else set stays. The object leaves status.keptObjects.
func TestHandle_ManifestsTakeOverWhatASwitchLeft(t *testing.T) {
	ctx := context.Background()
	left := configMap("proj", "settings")
	left.Labels = map[string]string{
		v1alpha3.ResourceLabelProject:              "proj",
		v1alpha3.ResourceLabelTemplate:             "custom",
		v1alpha3.ResourceLabelKeptFromHelmTemplate: "true",
	}
	left.Annotations = releaseAnnotations(map[string]string{helmResourcePolicy: helmResourcePolicyKeep})
	left.Data = map[string]string{"team": "from the Helm text", "extra": "only the Helm text has it", "edited": "by hand"}
	left.ManagedFields = []metav1.ManagedFieldsEntry{
		{
			Manager: releaseFieldManager, Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "v1", FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:data":{"f:team":{},"f:extra":{}},"f:metadata":{` +
				`"f:annotations":{"f:` + helm.ResourceAnnotationReleaseName + `":{},"f:` + helmResourcePolicy + `":{}},` +
				`"f:labels":{"f:` + v1alpha3.ResourceLabelProject + `":{},"f:` + v1alpha3.ResourceLabelTemplate + `":{},"f:` +
				v1alpha3.ResourceLabelKeptFromHelmTemplate + `":{}}}}`)},
		},
		{
			Manager: "kubectl-edit", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "v1", FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:data":{"f:edited":{}}}`)},
		},
	}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status:     v1alpha3.ProjectStatus{KeptObjects: []v1alpha3.KeptObject{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"}}},
	}
	objs := append(manifestsCluster(manifestsText)[1:], project, left)
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	takeover := manifestsUser + " ConfigMap proj/settings"
	assert.Equal(t, []string{takeover}, clients.verbs("patch"), "the fields of the release are handed over as the user")
	assert.Contains(t, clients.verbs("apply"), takeover, "taken over by the apply")
	callOf := func(verb string) int {
		return slices.IndexFunc(clients.calls, func(call manifestsCall) bool {
			return call.verb == verb && call.kind == "ConfigMap" && call.name == "settings"
		})
	}
	assert.Less(t, callOf("patch"), callOf("apply"), "the fields are handed over before the apply")

	assert.Equal(t, map[string]string{"team": "platform", "edited": "by hand"}, settings.Data,
		"what the release set and the manifests leave out goes, what somebody else set stays")
	assert.Equal(t, manifestLabels(), settings.Labels, "the labels of the release and the mark of a left object go")
	assert.Empty(t, settings.Annotations, "the annotations of Helm go")
	for _, entry := range settings.ManagedFields {
		assert.NotEqual(t, releaseFieldManager, entry.Manager, "the release owns no field any more")
	}

	stored := storedProject(t, c)
	assert.Empty(t, stored.Status.KeptObjects)
	assert.Equal(t, manifestsObjects, stored.Status.ManifestObjects)
	assert.Zero(t, keptObjectsSeries("proj"))
}

// An object that exists and that the manifests neither applied nor got from a switch is somebody
// else's: it is not overwritten, listed or later pruned, and the condition says how to hand it over.
// The other objects are applied, and nothing is deleted meanwhile. One that has the labels of the
// manifests of the project is the project's, as after the list was lost.
func TestHandle_ManifestsLeaveAloneWhatIsNotTheirs(t *testing.T) {
	ctx := context.Background()

	t.Run("an object of somebody else", func(t *testing.T) {
		theirs := configMap("proj", "settings")
		theirs.Data = map[string]string{"team": "the tenant's"}
		stale := configMap("proj", "stale")
		stale.Labels = manifestLabels()
		project := &v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "proj"},
			Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
			Status:     v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "stale"}}},
		}
		objs := append(manifestsCluster(manifestsText)[1:], project, theirs, stale)
		m, c, clients := newManifestsManager(t, objs...)

		result, err := m.Handle(ctx, storedProject(t, c))
		require.NoError(t, err, "nothing to retry until somebody acts")
		assert.Equal(t, manifestsForeignRecheck, result.RequeueAfter)

		assert.Equal(t, []string{manifestsUser + " RoleBinding proj-extra/viewers"}, clients.verbs("apply"), "the other objects are applied")
		require.True(t, exists(t, c, theirs))
		assert.Equal(t, map[string]string{"team": "the tenant's"}, theirs.Data, "not overwritten")
		assert.Empty(t, theirs.Labels, "not labelled")
		assert.True(t, exists(t, c, stale), "nothing is deleted while an object is left out")
		assert.Empty(t, clients.verbs("delete"))

		condition := manifestsCondition(t, c)
		require.NotNil(t, condition)
		assert.Equal(t, corev1.ConditionFalse, condition.Status)
		assert.Contains(t, condition.Message, "do not take over objects that exist and are not theirs: ConfigMap proj/settings exists")
		assert.Contains(t, condition.Message, "give it the projects.deckhouse.io/project: proj and projects.deckhouse.io/source: manifests labels")
		stored := storedProject(t, c)
		assert.Equal(t, v1alpha3.ProjectStateError, stored.Status.State)
		assert.Equal(t, []v1alpha3.ManifestObject{
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "stale"},
			manifestsObjects[1],
		}, stored.Status.ManifestObjects, "the object is not listed")
	})

	t.Run("an object with the labels of the manifests of the project", func(t *testing.T) {
		ours := configMap("proj", "settings")
		ours.Labels = manifestLabels()
		ours.Data = map[string]string{"team": "before the list was lost"}
		m, c, clients := newManifestsManager(t, append(manifestsCluster(manifestsText), ours)...)

		_, err := m.Handle(ctx, storedProject(t, c))
		require.NoError(t, err)
		assert.Len(t, clients.verbs("apply"), 2)
		require.True(t, exists(t, c, ours))
		assert.Equal(t, map[string]string{"team": "platform"}, ours.Data)
		assert.Equal(t, manifestsObjects, storedProject(t, c).Status.ManifestObjects)
		assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
	})
}

// A deleted project takes the binding of its user with it, before its namespaces go.
func TestManager_DeleteTakesTheManifestsBinding(t *testing.T) {
	ctx := context.Background()
	m, c, _ := newManifestsManager(t, manifestsCluster(manifestsText)...)

	_, err := m.Delete(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.False(t, exists(t, c, readyManifestsBinding()))
}

// The objects of a render come out in its order, a List unwrapped, each pinned to the main namespace
// when it names none, labelled, and without the fields a server sets; a document of comments is none.
func TestManifestObjectsPinsEveryObject(t *testing.T) {
	m, _, _ := newManifestsManager(t, manifestsCluster("")...)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	const rendered = `
---
# only a comment
---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: first
    resourceVersion: "42"
    uid: 0b4e6b4f-1d2c-4bb5-9c1e-0a0a0a0a0a0a
    managedFields: [{manager: kubectl}]
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: second
    labels:
      app: web
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: third
  creationTimestamp: "2026-10-01T00:00:00Z"
status: {}
`
	objects, err := m.manifestObjects(project, rendered)
	require.NoError(t, err)
	require.Len(t, objects, 3)
	assert.Equal(t, []string{"first", "second", "third"}, []string{objects[0].GetName(), objects[1].GetName(), objects[2].GetName()})
	for _, object := range objects {
		assert.Equal(t, "proj", object.GetNamespace())
		assert.Empty(t, object.GetResourceVersion())
		assert.Empty(t, object.GetUID())
		assert.Empty(t, object.GetManagedFields())
		assert.NotContains(t, object.Object["metadata"], "creationTimestamp")
		assert.NotContains(t, object.Object, "status")
		assert.Equal(t, v1alpha3.ResourceSourceManifests, object.GetLabels()[v1alpha3.ResourceLabelSource])
	}
	assert.Equal(t, "web", objects[1].GetLabels()["app"], "the labels of the author stay")
}
