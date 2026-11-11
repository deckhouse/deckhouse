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
	"slices"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// isolatedRendered is the NetworkPolicy named isolated as the release writes it once
// networkPolicy.mode is Isolated: in the main namespace, with the labels of the release, allowing
// ingress from the project only.
const isolatedRendered = `---
apiVersion: v1
kind: Namespace
metadata:
  name: proj
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: isolated
  namespace: proj
  labels:
    heritage: multitenancy-manager
    projects.deckhouse.io/project: proj
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          projects.deckhouse.io/project: proj
`

// isolatedRef is the NetworkPolicy named isolated as a release manifest holds it.
var isolatedRef = ref("networking.k8s.io/v1", "NetworkPolicy", "isolated")

// manifestsNetworkPolicy is the NetworkPolicy named isolated as spec.manifests applied it: with the
// labels of the manifests and an egress block the release does not render, every field owned by the
// field manager of the manifests, as an apply leaves it.
func manifestsNetworkPolicy(labels map[string]string) *unstructured.Unstructured {
	policy := new(unstructured.Unstructured)
	policy.SetAPIVersion("networking.k8s.io/v1")
	policy.SetKind("NetworkPolicy")
	policy.SetNamespace("proj")
	policy.SetName("isolated")
	policy.SetLabels(labels)
	policy.Object["spec"] = map[string]any{
		"podSelector": map[string]any{},
		"policyTypes": []any{"Egress"},
		"egress":      []any{map[string]any{}},
	}
	policy.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    manifestsFieldManager,
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "networking.k8s.io/v1",
		FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{"f:projects.deckhouse.io/project":{},` +
			`"f:projects.deckhouse.io/source":{}}},"f:spec":{"f:egress":{},"f:podSelector":{},"f:policyTypes":{}}}`)},
	}})
	return policy
}

// manifestsLabels are the labels the manifests of the project put on an object.
func manifestsLabels() map[string]string {
	return map[string]string{
		v1alpha3.ResourceLabelProject: "proj",
		v1alpha3.ResourceLabelSource:  v1alpha3.ResourceSourceManifests,
	}
}

// listedIsolated is a project whose status.manifestObjects lists the NetworkPolicy named isolated and
// a ConfigMap the manifests still render.
func listedIsolated() *v1alpha3.Project {
	project := helmProject()
	project.Status.ManifestObjects = []v1alpha3.ManifestObject{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"},
		{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated"},
	}
	return project
}

// addIsolated is the upgrade of a native release that adds the NetworkPolicy named isolated.
func addIsolated() helm.ReleaseChange {
	deployed := &helm.ProjectRelease{Revision: 3, Objects: []helm.ReleaseObject{mainNamespace}}
	return helm.ReleaseChange{
		Deployed:     deployed,
		Targets:      []helm.ReleaseObject{mainNamespace, isolatedRef},
		Manifest:     isolatedRendered,
		Native:       true,
		NextRevision: 4,
	}
}

// applyCall is one server-side apply through the client of the controller.
type applyCall struct {
	name, fieldManager string
	force              bool
}

// newHandoverManager is newTransitionManager whose client emulates a server-side apply and records
// it. refuse, when set, answers an apply first.
func newHandoverManager(t *testing.T, refuse func() error, objs ...client.Object) (*Manager, client.Client, *[]applyCall) {
	t.Helper()
	calls := new([]applyCall)
	m, c := newTransitionManagerWith(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, object client.Object, patch client.Patch, opts ...client.PatchOption) error {
			applied, ok := object.(*unstructured.Unstructured)
			if patch.Type() != types.ApplyPatchType || !ok {
				return c.Patch(ctx, object, patch, opts...)
			}
			options := new(client.PatchOptions)
			options.ApplyOptions(opts)
			*calls = append(*calls, applyCall{
				name: applied.GetName(), fieldManager: options.FieldManager, force: options.Force != nil && *options.Force,
			})
			if refuse != nil {
				if err := refuse(); err != nil {
					return err
				}
			}
			return emulateApply(ctx, c, applied)
		},
	}, objs...)
	return m, c, calls
}

// assertHandedOver checks that the NetworkPolicy named isolated is the release's now: it has the
// ownership marks Helm takes it over by, the labels and the spec the release writes, and nothing of
// the manifests, and status.manifestObjects no longer lists it.
func assertHandedOver(t *testing.T, c client.Client) {
	t.Helper()
	policy := manifestsNetworkPolicy(nil)
	require.True(t, exists(t, c, policy))
	assert.Equal(t, map[string]string{
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:  "proj",
		helm.ResourceLabelManagedBy:    helm.ManagedByHelm,
	}, policy.GetLabels(), "the labels of the release, not of the manifests")
	assert.Equal(t, map[string]string{
		helm.ResourceAnnotationReleaseName:      "proj",
		helm.ResourceAnnotationReleaseNamespace: "",
	}, policy.GetAnnotations())
	_, hasEgress, _ := unstructured.NestedFieldNoCopy(policy.Object, "spec", "egress")
	assert.False(t, hasEgress, "a field only the manifests set is gone")
	policyTypes, _, _ := unstructured.NestedStringSlice(policy.Object, "spec", "policyTypes")
	assert.Equal(t, []string{"Ingress"}, policyTypes)
	_, hasIngress, _ := unstructured.NestedFieldNoCopy(policy.Object, "spec", "ingress")
	assert.True(t, hasIngress, "the release writes its own spec")

	stored := storedProject(t, c)
	assert.Equal(t, []v1alpha3.ManifestObject{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"}},
		stored.Status.ManifestObjects, "the manifests keep the rest of their list")
	assert.Equal(t, []v1alpha3.HandingOverObject{{
		APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated", Revision: 4,
	}}, stored.Status.HandingOver, "the journal names the object and the revision the upgrade is expected to deploy")
}

// A move of the NetworkPolicy named isolated from spec.manifests to networkPolicy.mode in one change
// of the template hands the object over to the release before the upgrade, whether
// status.manifestObjects lists it or it has only the labels of the manifests, as when the list was
// lost.
func TestManager_PrepareUpgradeHandsManifestObjectsToTheRelease(t *testing.T) {
	tests := []struct {
		name    string
		project *v1alpha3.Project
	}{
		{name: "listed in status.manifestObjects", project: listedIsolated()},
		{name: "found by the labels of the manifests", project: func() *v1alpha3.Project {
			project := listedIsolated()
			project.Status.ManifestObjects = project.Status.ManifestObjects[:1]
			return project
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m, c, calls := newHandoverManager(t, nil, tt.project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()))

			require.NoError(t, m.prepareUpgrade(ctx, tt.project, addIsolated()))

			assert.Equal(t, []applyCall{{name: "isolated", fieldManager: manifestsFieldManager, force: true}}, *calls,
				"one apply by the field manager of the manifests")
			assertHandedOver(t, c)
		})
	}
}

// An object that the upgrade adds and that the manifests did not apply is somebody else's: it is not
// touched, and the upgrade refuses it as before.
func TestManager_PrepareUpgradeLeavesAForeignObject(t *testing.T) {
	ctx := context.Background()
	project := helmProject()
	foreign := manifestsNetworkPolicy(map[string]string{"app": "mine"})
	m, c, calls := newHandoverManager(t, nil, project, projectNamespace(), foreign)

	require.NoError(t, m.prepareUpgrade(ctx, project, addIsolated()))

	assert.Empty(t, *calls)
	live := manifestsNetworkPolicy(nil)
	require.True(t, exists(t, c, live))
	assert.Equal(t, map[string]string{"app": "mine"}, live.GetLabels())
}

// An object the deployed revision already holds is the release's: there is nothing to hand over, and
// neither is there for an object that does not exist yet, which the upgrade creates.
func TestManager_PrepareUpgradeHandsOverOnlyWhatTheUpgradeAddsAndExists(t *testing.T) {
	ctx := context.Background()

	t.Run("held by the deployed revision", func(t *testing.T) {
		project := listedIsolated()
		m, _, calls := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()))
		change := addIsolated()
		change.Deployed.Objects = append(change.Deployed.Objects, isolatedRef)

		require.NoError(t, m.prepareUpgrade(ctx, project, change))
		assert.Empty(t, *calls)
	})

	t.Run("not there", func(t *testing.T) {
		project := listedIsolated()
		m, c, calls := newHandoverManager(t, nil, project, projectNamespace())

		require.NoError(t, m.prepareUpgrade(ctx, project, addIsolated()))
		assert.Empty(t, *calls)
		assert.Len(t, storedProject(t, c).Status.ManifestObjects, 2, "the list is the manifests' to keep")
	})
}

// The handover is on record before the object is applied. A pass that the API server stops at the
// apply leaves the object off status.manifestObjects, in status.handingOver, with the labels of the
// manifests still on it. The next pass finds it from the journal.
func TestManager_PrepareUpgradeRecordsTheHandoverBeforeTheApply(t *testing.T) {
	ctx := context.Background()
	project := listedIsolated()
	refused := errors.New("the API server is away")
	refuse := refused
	m, c, calls := newHandoverManager(t, func() error { return refuse }, project, projectNamespace(),
		manifestsNetworkPolicy(manifestsLabels()))

	require.ErrorIs(t, m.prepareUpgrade(ctx, project, addIsolated()), refused)
	stored := storedProject(t, c)
	assert.Len(t, stored.Status.ManifestObjects, 1, "off the list before the apply")
	assert.Equal(t, []v1alpha3.HandingOverObject{{
		APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated", Revision: 4,
	}}, stored.Status.HandingOver, "the journal is written in the same status update")
	live := manifestsNetworkPolicy(nil)
	require.True(t, exists(t, c, live))
	assert.Equal(t, manifestsLabels(), live.GetLabels(), "the object is as the manifests left it")

	refuse = nil
	require.NoError(t, m.prepareUpgrade(ctx, storedProject(t, c), addIsolated()))
	assert.Len(t, *calls, 2)
	assertHandedOver(t, c)
}

// An upgrade that did not deploy the expected revision puts the object back under manifests. The
// heritage and template labels and the Helm ownership marks the handover added go, because the deployed revision
// does not hold the object. The controller may remove the heritage label.
func TestFinishManifestHandoverRollsBackWhenTheUpgradeDidNotDeploy(t *testing.T) {
	ctx := context.Background()
	project := listedIsolated()
	m, c, _ := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()))
	require.NoError(t, m.prepareUpgrade(ctx, project, addIsolated()))

	// The apply emulation drops a field the manifests manager owns only when the next apply leaves it
	// out. Claim the marks the handover wrote, the way that apply owns them.
	live := manifestsNetworkPolicy(nil)
	require.True(t, exists(t, c, live))
	live.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager: manifestsFieldManager, Operation: metav1.ManagedFieldsOperationApply, APIVersion: "networking.k8s.io/v1", FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{"f:heritage":{},"f:projects.deckhouse.io/project":{},"f:app.kubernetes.io/managed-by":{},` +
			`"f:projects.deckhouse.io/project-template":{}},"f:annotations":{"f:meta.helm.sh/release-name":{},"f:meta.helm.sh/release-namespace":{}}}}`)},
	}})
	// The release labels its objects with the template too, and the handover applied that label.
	labels := live.GetLabels()
	labels[v1alpha3.ResourceLabelTemplate] = "custom"
	live.SetLabels(labels)
	require.NoError(t, c.Update(ctx, live))

	m.helmClient = &fakeHelmClient{current: &helm.ProjectRelease{Revision: 3, Objects: []helm.ReleaseObject{mainNamespace}}}
	require.NoError(t, m.finishManifestHandover(ctx, storedProject(t, c)))

	require.True(t, exists(t, c, live))
	assert.Equal(t, manifestsLabels(), live.GetLabels(), "the labels of the release, the template label among them, go")
	assert.Empty(t, live.GetAnnotations()[helm.ResourceAnnotationReleaseName])
	assert.Empty(t, live.GetAnnotations()[helm.ResourceAnnotationReleaseNamespace])
	stored := storedProject(t, c)
	assert.Empty(t, stored.Status.HandingOver)
	assert.Contains(t, stored.Status.ManifestObjects, v1alpha3.ManifestObject{
		APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated",
	})
}

// An upgrade that deployed the expected revision drops the journal entry and leaves the object with
// the release. The entry is still there until this pass, which is the step after a successful upgrade.
func TestFinishManifestHandoverDropsAnEntryTheReleaseDeployed(t *testing.T) {
	ctx := context.Background()
	project := listedIsolated()
	m, c, _ := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()))
	require.NoError(t, m.prepareUpgrade(ctx, project, addIsolated()))
	m.helmClient = &fakeHelmClient{current: &helm.ProjectRelease{Revision: 4, Objects: []helm.ReleaseObject{mainNamespace, isolatedRef}}}

	require.NoError(t, m.finishManifestHandover(ctx, storedProject(t, c)))

	assert.Empty(t, storedProject(t, c).Status.HandingOver)
	live := manifestsNetworkPolicy(nil)
	require.True(t, exists(t, c, live))
	assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, live.GetLabels()[v1alpha3.ResourceLabelHeritage], "the release keeps the object")
	assert.NotContains(t, storedProject(t, c).Status.ManifestObjects, v1alpha3.ManifestObject{
		APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated",
	})
}

// A failed upgrade, then a template that no longer hands the object to the release, leaves the
// project Deployed with the object back under manifests.
func TestHandle_FailedUpgradeRollsTheManifestsHandoverBack(t *testing.T) {
	ctx := context.Background()
	// The handover and its rollback apply through the controller client, which has to emulate a
	// server-side apply the way the manifests client does.
	m, c, _ := newHandoverManager(t, nil, manifestsCluster(manifestsText)...)
	cluster, ok := c.(client.WithWatch)
	require.True(t, ok)
	m.manifestsClients = &fakeManifestsClients{cluster: cluster}
	renderer, err := helm.NewRenderOnly("../../../helmlib", logr.Discard())
	require.NoError(t, err)
	m.helmClient = &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, renderer: renderer}
	t.Cleanup(func() { m.Forget("proj") })
	require.NoError(t, mustHandle(t, m, c))
	require.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State)

	fake, ok := m.helmClient.(*fakeHelmClient)
	require.True(t, ok)
	fake.current = &helm.ProjectRelease{Revision: 1, Objects: []helm.ReleaseObject{mainNamespace}}
	fake.change = &helm.ReleaseChange{
		Deployed: fake.current,
		Targets:  []helm.ReleaseObject{mainNamespace, ref("v1", "ConfigMap", "settings")},
		Manifest: `---
apiVersion: v1
kind: Namespace
metadata:
  name: proj
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  namespace: proj
  labels:
    heritage: multitenancy-manager
data:
  team: from the release
`,
		Native:       true,
		NextRevision: 2,
	}
	fake.upgradeErr = errors.New("the upgrade failed")

	_, err = m.Handle(ctx, storedProject(t, c))
	require.Error(t, err)
	failed := storedProject(t, c)
	assert.Equal(t, v1alpha3.ProjectStateError, failed.Status.State)
	assert.Empty(t, failed.Status.HandingOver)
	assert.Contains(t, failed.Status.ManifestObjects, v1alpha3.ManifestObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"})
	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	assert.Equal(t, v1alpha3.ResourceSourceManifests, settings.Labels[v1alpha3.ResourceLabelSource])
	assert.Equal(t, "proj", settings.Labels[v1alpha3.ResourceLabelProject])

	// The template no longer renders the object into the release.
	fake.change = nil
	fake.upgradeErr = nil
	require.NoError(t, mustHandle(t, m, c))
	deployed := storedProject(t, c)
	assert.Equal(t, v1alpha3.ProjectStateDeployed, deployed.Status.State)
	assert.Empty(t, deployed.Status.HandingOver)
	assert.Contains(t, deployed.Status.ManifestObjects, v1alpha3.ManifestObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "settings"})
	require.True(t, exists(t, c, settings))
	assert.Equal(t, v1alpha3.ResourceSourceManifests, settings.Labels[v1alpha3.ResourceLabelSource])
	assert.Equal(t, "proj", settings.Labels[v1alpha3.ResourceLabelProject])
}

// An object of somebody else that the upgrade would add keeps the handover from running. The upgrade
// does not run either, and ResourcesUpgraded names that object on every reconcile, never the object
// of the manifests, which Helm refuses too and may report first.
func TestHandle_BlockedHandoverNamesTheForeignObject(t *testing.T) {
	m, c, calls := newHandoverManager(t, nil, append(manifestsCluster(manifestsText),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "proj-extra", Name: "settings"}})...)
	cluster, ok := c.(client.WithWatch)
	require.True(t, ok)
	m.manifestsClients = &fakeManifestsClients{cluster: cluster}
	renderer, err := helm.NewRenderOnly("../../../helmlib", logr.Discard())
	require.NoError(t, err)
	fake := &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}, renderer: renderer}
	m.helmClient = fake
	t.Cleanup(func() { m.Forget("proj") })
	require.NoError(t, mustHandle(t, m, c))
	require.Equal(t, v1alpha3.ProjectStateDeployed, storedProject(t, c).Status.State)

	fake.current = &helm.ProjectRelease{Revision: 1, Objects: []helm.ReleaseObject{mainNamespace}}
	fake.change = &helm.ReleaseChange{
		Deployed: fake.current,
		Targets: []helm.ReleaseObject{mainNamespace, ref("v1", "ConfigMap", "settings"),
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj-extra", Name: "settings"}},
		Native:       true,
		NextRevision: 2,
	}
	// What Helm would report if the upgrade ran: the object of the manifests.
	fake.upgradeErr = errors.New(`ConfigMap "settings" in namespace "proj" exists and cannot be imported into the current release`)
	*calls = nil

	want := `upgrade project resources: ConfigMap "settings" in namespace "proj-extra" exists and cannot be imported ` +
		`into the release of the project: not an object of the manifests, without the ownership marks of the release. ` +
		`The upgrade does not run, and the objects of the manifests it would take over stay with the manifests: ` +
		`ConfigMap "settings" in namespace "proj"`
	for range 2 {
		require.Error(t, mustHandle(t, m, c))
		failed := storedProject(t, c)
		assert.Equal(t, v1alpha3.ProjectStateError, failed.Status.State)
		upgraded := conditionByType(failed, v1alpha3.ProjectConditionProjectResourcesUpgraded)
		require.NotNil(t, upgraded)
		assert.Equal(t, want, upgraded.Message)
		assert.Empty(t, failed.Status.HandingOver)
	}
	assert.Empty(t, *calls, "the objects of the manifests are not touched")
	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	assert.Equal(t, v1alpha3.ResourceSourceManifests, settings.Labels[v1alpha3.ResourceLabelSource])
}

func mustHandle(t *testing.T, m *Manager, c client.Client) error {
	t.Helper()
	_, err := m.Handle(context.Background(), storedProject(t, c))
	return err
}

// The objects of a release manifest are read by the keys asked for, and a key the manifest does not
// hold is an error rather than an empty apply.
func TestReleaseObjectsOf(t *testing.T) {
	isolated := keyOf(isolatedRef, "proj")

	objects, err := releaseObjectsOf(isolatedRendered, "proj", []releaseKey{isolated})
	require.NoError(t, err)
	require.Contains(t, objects, isolated)
	assert.Equal(t, "isolated", objects[isolated].GetName())

	missing := keyOf(ref("deckhouse.io/v1alpha1", "PodLoggingConfig", "default"), "proj")
	_, err = releaseObjectsOf(isolatedRendered, "proj", []releaseKey{missing})
	require.Error(t, err)
}

// releasedConfigMap is the ConfigMap of the name as the release of the project created it.
func releasedConfigMap(name string) *corev1.ConfigMap {
	object := configMap("proj", name)
	object.Labels = releaseObjectLabels()
	object.Annotations = releaseAnnotations(nil)
	object.Data = map[string]string{"team": "from the release"}
	// The release writes with Update operations; leaving the object adds the keep annotation and the
	// mark of a left object with the identity of the controller, which the release shares.
	object.ManagedFields = []metav1.ManagedFieldsEntry{{
		Manager: releaseFieldManager, Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "v1", FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:data":{"f:team":{}},"f:metadata":{` +
			`"f:annotations":{"f:` + helm.ResourceAnnotationReleaseName + `":{},"f:` + helmResourcePolicy + `":{}},` +
			`"f:labels":{"f:` + v1alpha3.ResourceLabelHeritage + `":{},"f:` + v1alpha3.ResourceLabelProject + `":{},"f:` +
			v1alpha3.ResourceLabelTemplate + `":{},"f:` + v1alpha3.ResourceLabelKeptFromHelmTemplate + `":{}}}}`)},
	}}
	return object
}

// dropConfigMap is the upgrade of a native release that drops the ConfigMap of the name.
func dropConfigMap(name string) *helm.ReleaseChange {
	return &helm.ReleaseChange{
		Deployed: &helm.ProjectRelease{Revision: 4, Objects: []helm.ReleaseObject{mainNamespace, ref("v1", "ConfigMap", name)}},
		Targets:  []helm.ReleaseObject{mainNamespace},
		Native:   true,
	}
}

// A move the other way, of an object of the release to spec.manifests in one change of the template,
// leaves the object in place for the upgrade, which would delete it, and the manifests take it over in
// the same reconcile: the project is never without it.
func TestHandle_ManifestsTakeOverWhatTheReleaseDrops(t *testing.T) {
	ctx := context.Background()
	m, c, clients := newManifestsManager(t, append(manifestsCluster(manifestsText), releasedConfigMap("settings"))...)
	fake, ok := m.helmClient.(*fakeHelmClient)
	require.True(t, ok)
	fake.change = dropConfigMap("settings")

	require.NoError(t, m.prepareUpgrade(ctx, storedProject(t, c), *fake.change))
	assertLeft(t, c, configMap("proj", "settings"))

	_, err := m.Handle(ctx, storedProject(t, c))
	require.NoError(t, err)

	settings := configMap("proj", "settings")
	require.True(t, exists(t, c, settings))
	assert.Contains(t, clients.verbs("apply"), manifestsUser+" ConfigMap proj/settings")
	assert.Equal(t, map[string]string{"team": "platform"}, settings.Data, "the object is what the manifests render")
	assert.Equal(t, manifestLabels(), settings.Labels, "the labels of the release and of a left object go")
	stored := storedProject(t, c)
	assert.Empty(t, stored.Status.KeptObjects, "taken over in the same reconcile")
	assert.Equal(t, manifestsObjects, stored.Status.ManifestObjects)
}

// An object that the release drops and the manifests do not render is the upgrade's to delete, as
// before.
func TestManager_PrepareUpgradeLeavesOnlyWhatTheManifestsRender(t *testing.T) {
	ctx := context.Background()
	other := releasedConfigMap("other")
	m, c, _ := newManifestsManager(t, append(manifestsCluster(manifestsText), other)...)

	require.NoError(t, m.prepareUpgrade(ctx, storedProject(t, c), *dropConfigMap("other")))

	assertUntouched(t, c, other)
	assert.Empty(t, storedProject(t, c).Status.KeptObjects)
}

// isolatedIn is the NetworkPolicy named isolated in the namespace, with the labels and annotations.
func isolatedIn(namespace string, labels, annotations map[string]string) *unstructured.Unstructured {
	policy := new(unstructured.Unstructured)
	policy.SetAPIVersion("networking.k8s.io/v1")
	policy.SetKind("NetworkPolicy")
	policy.SetNamespace(namespace)
	policy.SetName("isolated")
	policy.SetLabels(labels)
	policy.SetAnnotations(annotations)
	policy.Object["spec"] = map[string]any{"podSelector": map[string]any{}}
	return policy
}

// addIsolatedTwice is addIsolated with the NetworkPolicy named isolated in an additional namespace
// too, as the structured render puts it into every namespace of the project.
func addIsolatedTwice() helm.ReleaseChange {
	change := addIsolated()
	change.Targets = append(change.Targets, helm.ReleaseObject{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj-extra", Name: "isolated"})
	return change
}

// An upgrade that adds an object Helm refuses, one that exists without the ownership marks of the
// release, fails whatever the handover does. The objects of the manifests are not touched then: no
// write, no journal, still on status.manifestObjects. Otherwise every retry of the failing upgrade
// would move them to the release and back. An object that carries the marks of the release is no
// such conflict, and the handover goes on.
func TestManager_PrepareUpgradeLeavesManifestsAloneOnAnOwnershipConflict(t *testing.T) {
	ctx := context.Background()

	t.Run("an object of somebody else", func(t *testing.T) {
		project := listedIsolated()
		foreign := isolatedIn("proj-extra", map[string]string{"app": "mine"}, nil)
		m, c, calls := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()), foreign)

		// The error names the object that blocks the upgrade, not the object of the manifests, which
		// Helm would refuse too and might report first.
		err := m.prepareUpgrade(ctx, project, addIsolatedTwice())
		require.EqualError(t, err, `NetworkPolicy "isolated" in namespace "proj-extra" exists and cannot be imported `+
			`into the release of the project: not an object of the manifests, without the ownership marks of the release. `+
			`The upgrade does not run, and the objects of the manifests it would take over stay with the manifests: `+
			`NetworkPolicy "isolated" in namespace "proj"`)

		assert.Empty(t, *calls, "no write into the objects of the manifests")
		live := manifestsNetworkPolicy(nil)
		require.True(t, exists(t, c, live))
		assert.Equal(t, manifestsLabels(), live.GetLabels())
		stored := storedProject(t, c)
		assert.Empty(t, stored.Status.HandingOver)
		assert.Contains(t, stored.Status.ManifestObjects, v1alpha3.ManifestObject{
			APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Namespace: "proj", Name: "isolated",
		})
	})

	t.Run("a cluster-scoped object of somebody else", func(t *testing.T) {
		project := listedIsolated()
		foreign := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "proj-reader"}}
		m, _, calls := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()), foreign)
		change := addIsolated()
		// The post-renderer pins every object of the release to the main namespace, this one too.
		change.Targets = append(change.Targets, helm.ReleaseObject{
			APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole", Namespace: "proj", Name: "proj-reader",
		})

		err := m.prepareUpgrade(ctx, project, change)
		require.EqualError(t, err, `ClusterRole "proj-reader" exists and cannot be imported `+
			`into the release of the project: not an object of the manifests, without the ownership marks of the release. `+
			`The upgrade does not run, and the objects of the manifests it would take over stay with the manifests: `+
			`NetworkPolicy "isolated" in namespace "proj"`)
		assert.Empty(t, *calls)
	})

	t.Run("an object with the marks of the release", func(t *testing.T) {
		project := listedIsolated()
		ours := isolatedIn("proj-extra",
			map[string]string{helm.ResourceLabelManagedBy: helm.ManagedByHelm},
			map[string]string{helm.ResourceAnnotationReleaseName: "proj", helm.ResourceAnnotationReleaseNamespace: ""})
		m, _, calls := newHandoverManager(t, nil, project, projectNamespace(), manifestsNetworkPolicy(manifestsLabels()), ours)

		require.NoError(t, m.prepareUpgrade(ctx, project, addIsolatedTwice()))

		assert.Equal(t, []applyCall{{name: "isolated", fieldManager: manifestsFieldManager, force: true}}, *calls)
	})

	t.Run("nothing of the manifests to hand over", func(t *testing.T) {
		project := listedIsolated()
		foreign := isolatedIn("proj-extra", map[string]string{"app": "mine"}, nil)
		m, _, calls := newHandoverManager(t, nil, project, projectNamespace(), foreign)

		require.NoError(t, m.prepareUpgrade(ctx, project, addIsolatedTwice()), "the upgrade reports the object itself")
		assert.Empty(t, *calls)
	})
}

// The error of a blocked handover is the same whatever order the objects come in, and names at most
// maxBlockingObjectsInError of the objects that block it.
func TestHandoverBlockedError(t *testing.T) {
	np := func(namespace, name string) releaseKey {
		return releaseKey{group: "networking.k8s.io", kind: "NetworkPolicy", namespace: namespace, name: name}
	}
	handed := []releaseKey{np("proj", "isolated")}
	blocking := []releaseKey{np("proj-d", "x"), np("proj-b", "x"), np("proj-a", "x"), np("proj-c", "x")}

	err := handoverBlockedError(handed, blocking)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), `NetworkPolicy "x" in namespace "proj-a", NetworkPolicy "x" in namespace "proj-b", `+
		`NetworkPolicy "x" in namespace "proj-c", 1 more exist and cannot be imported`), err.Error())

	reversed := slices.Clone(blocking)
	slices.Reverse(reversed)
	assert.EqualError(t, handoverBlockedError(handed, reversed), err.Error())
}
