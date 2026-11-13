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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/edition"
	"controller/internal/rolebinding"
)

// backendManifests makes the backend namespace of the project with a ProjectNamespace and puts a
// ConfigMap into it, next to a ConfigMap in the main namespace.
const backendManifests = `
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: backend
spec:
  name: backend
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: backend-settings
  namespace: {{ .projectName }}-backend
data:
  team: {{ .parameters.team | quote }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team | quote }}
`

// mainOnlyManifests is backendManifests without the backend namespace and its ConfigMap.
const mainOnlyManifests = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team | quote }}
`

var (
	backendNamespaceEntry = v1alpha3.ManifestObject{APIVersion: "deckhouse.io/v1alpha3", Kind: "ProjectNamespace", Namespace: "proj", Name: "backend"}
	backendConfigMapEntry = v1alpha3.ManifestObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj-backend", Name: "backend-settings"}
	settingsEntry         = v1alpha3.ManifestObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"}
)

// backendNamespace is the namespace that the ProjectNamespace named backend makes, as its controller
// labels it, which puts it into status.namespaces of the project.
func backendNamespace() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-backend", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj"}}}
}

// fannedOutManifestsBinding is the RoleBinding that the d8-manifests ProjectRoleBinding fans out
// into the namespace.
func fannedOutManifestsBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: rolebinding.PRBServiceName(v1alpha3.ProjectManifestsBinding)},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: v1alpha3.ProjectAdministratorsRoleName},
		Subjects:   []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: manifestsUser}},
	}
}

func TestManifestsNamespaceOf(t *testing.T) {
	projectNamespace := func(namespace string, spec map[string]any) *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetAPIVersion("deckhouse.io/v1alpha3")
		object.SetKind("ProjectNamespace")
		object.SetNamespace(namespace)
		object.SetName("backend")
		if spec != nil {
			object.Object["spec"] = spec
		}
		return object
	}
	other := projectNamespace("", map[string]any{"name": "backend"})
	other.SetKind("ConfigMap")

	tests := []struct {
		name      string
		object    *unstructured.Unstructured
		namespace string
		ok        bool
	}{
		{name: "without a namespace", object: projectNamespace("", map[string]any{"name": "backend"}), namespace: "proj-backend", ok: true},
		{name: "in the main namespace", object: projectNamespace("proj", map[string]any{"name": "backend"}), namespace: "proj-backend", ok: true},
		{name: "in another namespace, which its webhook refuses", object: projectNamespace("proj-extra", map[string]any{"name": "backend"})},
		{name: "without spec.name", object: projectNamespace("", nil)},
		{name: "another kind", object: other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, ok := manifestsNamespaceOf("proj", tt.object)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.namespace, namespace)
		})
	}
}

// A render names the namespaces its objects go to; the ones that are neither the project's nor made
// by a ProjectNamespace of the render are the ones the controller refuses.
func TestUnknownManifestNamespaces(t *testing.T) {
	rendered := `
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: backend
spec:
  name: backend
---
apiVersion: v1
kind: ConfigMap
metadata: {name: a, namespace: proj-backend}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: b, namespace: proj-extra}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: c, namespace: proj-frontend}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: d, namespace: kube-system}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: e}
`
	assert.Equal(t, []string{"kube-system", "proj-frontend"}, UnknownManifestNamespaces("proj", []string{"proj", "proj-extra"}, rendered))
	assert.Empty(t, UnknownManifestNamespaces("proj", nil, "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: e}\n"))
}

// The manifests make the backend namespace and put a ConfigMap into it. The first pass applies the
// ProjectNamespace and the ConfigMap of the main namespace, and the ConfigMap of the backend namespace
// waits for its namespace; then for the d8-manifests binding to reach it; and once it has, it is
// applied. The project is never in Error meanwhile, and nothing is deleted.
func TestHandle_ManifestsMakeTheirNamespaces(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, manifestsCluster(backendManifests)...)

	result, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Equal(t, []string{
		manifestsUser + " ProjectNamespace proj/backend",
		manifestsUser + " ConfigMap proj/settings",
	}, clients.verbs("apply"), "the objects of the existing namespaces are applied at once")
	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Equal(t, corev1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, "wait for the 'proj-backend' namespace to be created")
	assert.Positive(t, result.RequeueAfter)
	assert.LessOrEqual(t, result.RequeueAfter, manifestsNamespaceRecheck)
	stored := storedProject(t, c)
	assert.Equal(t, v1alpha3.ProjectStateDeployed, stored.Status.State, "a wait is no error")
	assert.Equal(t, []v1alpha3.ManifestObject{settingsEntry, backendNamespaceEntry}, stored.Status.ManifestObjects,
		"what waits is not on the list")

	// The namespace is still missing. The objects that were applied stay as they are.
	clients.calls = nil
	result, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Empty(t, clients.verbs("apply"), "ready objects are not applied again while only the wait is rechecked")
	assert.Positive(t, result.RequeueAfter)
	assert.LessOrEqual(t, result.RequeueAfter, manifestsNamespaceRecheck)
	assert.Contains(t, manifestsCondition(t, c).Message, "wait for the 'proj-backend' namespace to be created")

	// The ProjectNamespace controller makes the namespace, before the binding reaches it.
	require.NoError(t, c.Create(ctx, backendNamespace()))
	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.NotContains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	condition = manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Equal(t, corev1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, "wait for the d8-manifests ProjectRoleBinding to reach the 'proj-backend' namespace")

	// The binding is fanned out into it.
	require.NoError(t, c.Create(ctx, fannedOutManifestsBinding("proj-backend")))
	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Contains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	assert.Empty(t, clients.verbs("delete"))
	assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
	assert.Equal(t, []v1alpha3.ManifestObject{settingsEntry, backendNamespaceEntry, backendConfigMapEntry},
		storedProject(t, c).Status.ManifestObjects)
}

// A change of an object that is already applied invalidates the digest of the ready part, so that
// object is applied again while another namespace is still waited for.
func TestHandle_ReappliesAChangedReadyObjectWhileANamespaceIsWaitedFor(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, manifestsCluster(backendManifests)...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	require.Equal(t, "platform", settings.Data["team"])

	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	require.Empty(t, clients.verbs("apply"), "an unchanged ready object is not applied again")

	project := storedProject(t, c)
	project.Spec.Parameters = map[string]any{"team": "other"}
	require.NoError(t, c.Update(ctx, project))
	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.Contains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj/settings",
		"a changed ready object is applied again")
	assert.NotContains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	require.True(t, exists(t, c, settings))
	assert.Equal(t, "other", settings.Data["team"])
	assert.Equal(t, corev1.ConditionFalse, manifestsCondition(t, c).Status)
	assert.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State)
}

// A template that drops a ProjectNamespace of its manifests with the objects in its namespace deletes
// the ProjectNamespace, whose controller deletes the namespace with everything in it: the objects in
// the namespace leave the list without a delete of their own, which the API server could refuse while
// the namespace terminates.
func TestHandle_ManifestsDropTheirNamespace(t *testing.T) {
	ctx := context.Background()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status:     v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{settingsEntry, backendNamespaceEntry, backendConfigMapEntry}},
	}
	madeNamespace := &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: "backend", Labels: manifestLabels()},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "backend"},
	}
	backendSettings := configMap("proj-backend", "backend-settings")
	backendSettings.Labels = manifestLabels()
	settings := configMap("proj", "settings")
	settings.Labels = manifestLabels()
	objs := append(manifestsCluster(mainOnlyManifests)[1:], project, madeNamespace, backendNamespace(),
		fannedOutManifestsBinding("proj-backend"), backendSettings, settings)
	m, c, clients := newManifestsManager(t, objs...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.Equal(t, []string{manifestsUser + " ProjectNamespace proj/backend"}, clients.verbs("delete"),
		"only the ProjectNamespace is deleted: the namespace goes with it")
	assert.Equal(t, []v1alpha3.ManifestObject{settingsEntry}, storedProject(t, c).Status.ManifestObjects)
	assert.Equal(t, corev1.ConditionTrue, manifestsCondition(t, c).Status)
	assert.True(t, exists(t, c, backendSettings), "the controller of the ProjectNamespace deletes it with the namespace")
}

// A namespace that never appears does not keep the project from deleting what the render dropped,
// and the condition repeats why the ProjectNamespace is not Ready. The project stays Deployed.
func TestHandle_ManifestsPruneWhileANamespaceIsWaitedFor(t *testing.T) {
	ctx := context.Background()
	const notReady = `namespace "proj-backend" already exists and is not owned by project "proj"`
	stale := configMap("proj", "stale")
	stale.Labels = manifestLabels()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "tmpl"},
		Status: v1alpha3.ProjectStatus{ManifestObjects: []v1alpha3.ManifestObject{
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "stale"},
		}},
	}
	pns := &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: "backend", Labels: manifestLabels()},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "backend"},
		Status: v1alpha3.ProjectNamespaceStatus{Conditions: []v1alpha3.Condition{{
			Type: v1alpha3.ProjectNamespaceConditionReady, Status: corev1.ConditionFalse, Message: notReady,
		}}},
	}
	objs := append(manifestsCluster(backendManifests)[1:], project, stale, pns)
	m, c, clients := newManifestsManager(t, objs...)

	result, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.False(t, exists(t, c, stale), "an object the render no longer holds is deleted while another namespace is waited for")
	assert.Equal(t, []string{manifestsUser + " ConfigMap proj/stale"}, clients.verbs("delete"))
	assert.NotContains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Equal(t, corev1.ConditionFalse, condition.Status)
	assert.Contains(t, condition.Message, notReady)
	assert.NotContains(t, condition.Message, "to finish terminating")
	assert.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State)
	assert.Positive(t, result.RequeueAfter)
	assert.LessOrEqual(t, result.RequeueAfter, manifestsNamespaceRecheck)

	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Empty(t, clients.verbs("apply"), "the ready objects are not applied again on the recheck")
	assert.Contains(t, manifestsCondition(t, c).Message, notReady)
}

// backendNamespaceOnlyManifests is backendManifests without the ConfigMap of the backend namespace.
const backendNamespaceOnlyManifests = `
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: backend
spec:
  name: backend
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team | quote }}
`

// An object that the manifests applied and that waits again, as when the binding of their user left
// its namespace, stays on status.manifestObjects through a change of the render. A later render that
// drops it then deletes it once the binding is back: an object off the list is never deleted.
func TestHandle_ManifestsKeepAWaitingObjectOnTheList(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, append(manifestsCluster(backendManifests), backendNamespace(), fannedOutManifestsBinding("proj-backend"))...)
	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	require.Contains(t, storedProject(t, c).Status.ManifestObjects, backendConfigMapEntry, "applied in the first pass")

	// the binding leaves the namespace, and the parameters change, so the render changes
	require.NoError(t, c.Delete(ctx, fannedOutManifestsBinding("proj-backend")))
	project := storedProject(t, c)
	project.Spec.Parameters = map[string]any{"team": "other"}
	require.NoError(t, c.Update(ctx, project))
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Contains(t, storedProject(t, c).Status.ManifestObjects, backendConfigMapEntry, "a waiting object that was applied stays on the list")

	// the template drops the ConfigMap, and the binding comes back
	template := new(v1alpha2.ProjectTemplate)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "tmpl"}, template))
	template.Spec.Manifests = backendNamespaceOnlyManifests
	require.NoError(t, c.Update(ctx, template))
	require.NoError(t, c.Create(ctx, fannedOutManifestsBinding("proj-backend")))
	clients.calls = nil
	_, err = m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	assert.Contains(t, clients.verbs("delete"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	assert.NotContains(t, storedProject(t, c).Status.ManifestObjects, backendConfigMapEntry)
}

// A namespace that is being deleted is named as terminating, not as one that was never created.
func TestHandle_ManifestsWaitForATerminatingNamespace(t *testing.T) {
	ctx := context.Background()
	now := metav1.Now()
	going := backendNamespace()
	going.DeletionTimestamp = &now
	going.Finalizers = []string{"kubernetes"}
	m, c, clients := newManifestsManager(t, append(manifestsCluster(backendManifests), going)...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Contains(t, condition.Message, "the 'proj-backend' namespace to finish terminating")
	assert.NotContains(t, condition.Message, "to be created")
	assert.NotContains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj-backend/backend-settings")
	assert.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State)
}

// A ConfigMap in the namespace of a ProjectNamespace that the render does not hold refuses the render,
// as an object in any namespace outside the project does.
func TestHandle_ManifestsRefuseANamespaceNobodyMakes(t *testing.T) {
	ctx := context.Background()
	text := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: backend-settings
  namespace: {{ .projectName }}-backend
`
	m, c, clients := newManifestsManager(t, manifestsCluster(text)...)

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)
	assert.Empty(t, clients.verbs("apply"))
	condition := manifestsCondition(t, c)
	require.NotNil(t, condition)
	assert.Contains(t, condition.Message,
		"the ConfigMap 'backend-settings' is in the 'proj-backend' namespace, which is neither a namespace of the project nor one that a ProjectNamespace of the manifests makes")
	assert.Equal(t, v1alpha3.ProjectStateError, storedProject(t, c).Status.State)
}

// A namespace that a ProjectNamespace of the render makes and that is being deleted, as when the
// ProjectNamespace was dropped and comes back before its namespace is gone, is waited for as one
// that is terminating, not as one that was never created, even while status.namespaces still lists
// it and its binding is there.
func TestManager_HoldBackForATerminatingNamespace(t *testing.T) {
	ctx := context.Background()
	now := metav1.Now()
	going := backendNamespace()
	going.DeletionTimestamp = &now
	going.Finalizers = []string{"kubernetes"}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Status: v1alpha3.ProjectStatus{Namespaces: []v1alpha3.NamespaceStatus{
			{Name: "proj", Kind: v1alpha3.NamespaceKindMain}, {Name: "proj-backend", Kind: v1alpha3.NamespaceKindAdditional},
		}},
	}
	m, _ := newTransitionManager(t, projectNamespace(), going, fannedOutManifestsBinding("proj-backend"))
	inBackend := configMap("proj-backend", "backend-settings")
	object, err := runtimeToUnstructured(inBackend)
	require.NoError(t, err)

	ready, held, waiting, err := m.holdBackForNamespaces(ctx, project, []string{"proj-backend"}, []*unstructured.Unstructured{object})
	require.NoError(t, err)
	assert.Empty(t, ready)
	assert.Len(t, held, 1)
	assert.Equal(t, []string{"the 'proj-backend' namespace to finish terminating"}, waiting)
}

// A ProjectNamespace that is not Ready says why in the wait, in place of the generic "to be created".
func TestManager_HoldBackNamesProjectNamespaceNotReady(t *testing.T) {
	ctx := context.Background()
	const notReady = `namespace "proj-backend" already exists and is not owned by project "proj"`
	pns := &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: "backend"},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "backend"},
		Status: v1alpha3.ProjectNamespaceStatus{Conditions: []v1alpha3.Condition{{
			Type: v1alpha3.ProjectNamespaceConditionReady, Status: corev1.ConditionFalse, Message: notReady,
		}}},
	}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	m, _ := newTransitionManager(t, pns)
	object, err := runtimeToUnstructured(configMap("proj-backend", "backend-settings"))
	require.NoError(t, err)

	_, _, waiting, err := m.holdBackForNamespaces(ctx, project, []string{"proj-backend"}, []*unstructured.Unstructured{object})
	require.NoError(t, err)
	assert.Equal(t, []string{"the 'proj-backend' namespace to be created: " + notReady}, waiting)
}

// In CE no ProjectNamespace makes a namespace, so the objects of a namespace that a ProjectNamespace
// of the render names are held back for good, even when the namespace is there with the label of the
// project, and the message says so rather than that they wait for the namespace. The other objects
// are applied. The test switches the edition of the binary, so it does not run in parallel.
func TestManager_HoldBackForGoodInCE(t *testing.T) {
	defer edition.SetForTest(edition.CE)()
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	m, _ := newTransitionManager(t, projectNamespace(), backendNamespace(), fannedOutManifestsBinding("proj-backend"))
	inBackend, err := runtimeToUnstructured(configMap("proj-backend", "backend-settings"))
	require.NoError(t, err)
	inMain, err := runtimeToUnstructured(configMap("proj", "settings"))
	require.NoError(t, err)

	ready, held, waiting, err := m.holdBackForNamespaces(context.Background(), project, []string{"proj-backend"},
		[]*unstructured.Unstructured{inBackend, inMain})
	require.NoError(t, err)
	assert.Equal(t, []*unstructured.Unstructured{inMain}, ready)
	assert.Equal(t, []*unstructured.Unstructured{inBackend}, held)
	assert.Equal(t, []string{"'proj-backend'"}, waiting)
	assert.Equal(t, "The other objects of the manifests of the template are applied, and the objects in the namespaces "+
		"that ProjectNamespace objects of the manifests name ('proj-backend') are not applied. "+edition.OnlyMainNamespace, manifestsWaitingMessage(waiting))
}

// In a commercial edition the objects wait for the namespace the ProjectNamespace makes.
func TestManifestsWaitingMessage_CommercialEdition(t *testing.T) {
	defer edition.SetForTest("EE")()
	assert.Equal(t, "The other objects of the manifests of the template are applied, and the objects in the namespaces that "+
		"ProjectNamespace objects of the manifests make wait for the 'proj-backend' namespace to be created.",
		manifestsWaitingMessage([]string{"the 'proj-backend' namespace to be created"}))
}

func runtimeToUnstructured(object *corev1.ConfigMap) (*unstructured.Unstructured, error) {
	fields, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return nil, err
	}
	converted := &unstructured.Unstructured{Object: fields}
	converted.SetAPIVersion("v1")
	converted.SetKind("ConfigMap")
	return converted, nil
}
