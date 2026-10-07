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

package helm

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// helmTextManifest is the manifest of a release rendered from a copy of the former default Helm
// text: an AuthorizationRule per administrator and the all-pods quota.
const helmTextManifest = `---
# Source: proj/templates/template.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: proj
---
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: alice
  namespace: proj
spec:
  accessLevel: Admin
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: all-pods
  namespace: proj
`

// helmTextRelease is a revision rendered from a Helm text: its chart renders templates/template.yaml,
// and the values carry the project the way buildValues lays it out.
func helmTextRelease(manifest string) *release.Release {
	return &release.Release{
		Name:    "proj",
		Version: 4,
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{Name: "proj", Version: "0.0.1"},
			Templates: []*chart.File{
				{Name: "templates/_helpers.tpl"},
				{Name: helmTextChartTemplate, Data: []byte("{{- tpl .Values.projectTemplate.resourcesTemplate .Values.project }}")},
			},
		},
		Config: map[string]any{
			"projectTemplate": map[string]any{"resourcesTemplate": "kind: Namespace\n"},
			"project": map[string]any{
				"projectName":         "proj",
				"projectTemplateName": "custom",
				"parameters":          map[string]any{},
			},
		},
		Manifest: manifest,
		Info:     &release.Info{Status: release.StatusDeployed},
	}
}

func TestNewProjectRelease(t *testing.T) {
	t.Parallel()

	t.Run("a revision rendered from a Helm text", func(t *testing.T) {
		t.Parallel()
		got, err := newProjectRelease(helmTextRelease(helmTextManifest))
		require.NoError(t, err)

		assert.Equal(t, 4, got.Revision)
		assert.True(t, got.HelmText)
		assert.Equal(t, []ReleaseObject{
			{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "alice"},
			{APIVersion: "v1", Kind: "Namespace", Name: "proj"},
			{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods"},
		}, got.Objects)
	})

	// The text read the administrators from .parameters as a list of {subject, name}; an entry of
	// another shape names nobody.
	t.Run("the administrators a Helm text was rendered with", func(t *testing.T) {
		t.Parallel()
		rel := helmTextRelease(helmTextManifest)
		rel.Config["project"].(map[string]any)["parameters"] = map[string]any{
			"administrators": []any{
				map[string]any{"subject": "User", "name": "alice"},
				map[string]any{"subject": "Group", "name": "admins", "accessLevel": "Editor"},
				map[string]any{"subject": "User"},
				map[string]any{"name": "nobody"},
				"carol",
			},
		}

		got, err := newProjectRelease(rel)
		require.NoError(t, err)
		assert.Equal(t, []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "admins"}}, got.Administrators)
	})

	t.Run("administrators of another shape", func(t *testing.T) {
		t.Parallel()
		rel := helmTextRelease(helmTextManifest)
		rel.Config["project"].(map[string]any)["parameters"] = map[string]any{"administrators": "alice"}

		got, err := newProjectRelease(rel)
		require.NoError(t, err)
		assert.Empty(t, got.Administrators)
	})

	t.Run("a natively rendered revision", func(t *testing.T) {
		t.Parallel()
		native := &release.Release{
			Name:     "proj",
			Version:  5,
			Chart:    buildEmptyChart("proj"),
			Config:   map[string]any{},
			Manifest: "---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: proj\n",
		}
		got, err := newProjectRelease(native)
		require.NoError(t, err)

		assert.False(t, got.HelmText)
		assert.Empty(t, got.Administrators)
		assert.Equal(t, []ReleaseObject{{APIVersion: "v1", Kind: "Namespace", Name: "proj"}}, got.Objects)
	})

	t.Run("the values of another chart are not read", func(t *testing.T) {
		t.Parallel()
		foreign := helmTextRelease("")
		foreign.Chart.Templates = []*chart.File{{Name: "templates/deployment.yaml"}}
		foreign.Config["project"].(map[string]any)["parameters"] = map[string]any{
			"administrators": []any{map[string]any{"subject": "User", "name": "alice"}},
		}

		got, err := newProjectRelease(foreign)
		require.NoError(t, err)
		assert.False(t, got.HelmText)
		assert.Empty(t, got.Administrators)
	})

	// The take-back of an object the release renders again restores what the manifest says.
	t.Run("the marks of the manifest", func(t *testing.T) {
		t.Parallel()
		manifest := "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: labelled\n  namespace: proj\n" +
			"  labels:\n    heritage: multitenancy-manager\n" +
			"---\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: kept\n  namespace: proj\n" +
			"  annotations:\n    helm.sh/resource-policy: keep\n"
		got, err := newProjectRelease(helmTextRelease(manifest))
		require.NoError(t, err)
		assert.Equal(t, []ReleaseObject{
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "labelled", Heritage: true},
			{APIVersion: "v1", Kind: "PersistentVolumeClaim", Namespace: "proj", Name: "kept", Keep: true},
		}, got.Objects)
	})

	t.Run("a manifest that is not YAML is an error", func(t *testing.T) {
		t.Parallel()
		_, err := newProjectRelease(helmTextRelease("---\nkind: [\n"))
		require.Error(t, err)
	})
}

// The revision an upgrade replaces is the one Helm diffs against: the latest deployed one, or the
// latest when it failed or was superseded and none is deployed. A release that never deployed
// anything installs anew.
func TestDeployedRevision(t *testing.T) {
	t.Parallel()

	revision := func(version int, status release.Status) *release.Release {
		return &release.Release{Version: version, Info: &release.Info{Status: status}}
	}
	tests := []struct {
		name     string
		releases []*release.Release
		expected int
	}{
		{name: "the latest is deployed", releases: []*release.Release{revision(3, release.StatusDeployed), revision(2, release.StatusSuperseded)}, expected: 3},
		{name: "the latest failed over a deployed one", releases: []*release.Release{revision(3, release.StatusFailed), revision(2, release.StatusDeployed)}, expected: 2},
		{name: "a pending upgrade over a deployed one", releases: []*release.Release{revision(3, release.StatusPendingUpgrade), revision(2, release.StatusDeployed)}, expected: 2},
		{name: "the only one failed", releases: []*release.Release{revision(1, release.StatusFailed)}, expected: 1},
		{name: "a pending install", releases: []*release.Release{revision(1, release.StatusPendingInstall)}},
		{name: "no revisions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := deployedRevision(tt.releases)
			if tt.expected == 0 {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.expected, got.Version)
		})
	}
}

// changeTemplate is a Helm text that renders a quota, a List with an AuthorizationRule and a
// ConfigMap in another project, an object of a kind the cluster does not serve, an unmanaged object
// and a second namespace.
const changeTemplate = `---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .projectName }}
---
apiVersion: v1
kind: Namespace
metadata:
  name: elsewhere
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: all-pods
spec:
  hard:
    pods: "10"
---
apiVersion: v1
kind: List
items:
  - apiVersion: deckhouse.io/v1alpha1
    kind: AuthorizationRule
    metadata:
      name: alice
    spec:
      accessLevel: Admin
      subjects:
        - kind: User
          name: alice
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: moved
      namespace: another-project
---
apiVersion: example.com/v1
kind: Unserved
metadata:
  name: nothing
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: once
  labels:
    projects.deckhouse.io/unmanaged: "true"
`

// changeVersions are the kinds the cluster serves in the releaseChange tests.
var changeVersions = map[string]struct{}{
	"v1/Namespace":     {},
	"v1/ResourceQuota": {},
	"v1/ConfigMap":     {},
	"deckhouse.io/v1alpha1/AuthorizationRule": {},
}

// The targets of an upgrade are what the post-renderer passes of the render: List items on their
// own, every namespaced object pinned to a namespace of the project, the main namespace only, no
// kind the cluster does not serve and no unmanaged object, which an upgrade does not render.
func TestClient_ReleaseChange(t *testing.T) {
	t.Parallel()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)
	c := &Client{templates: templates, logger: ctrl.Log.WithName("test")}
	project := &v1alpha3.Project{}
	project.Name = "proj"
	project.Spec.ProjectTemplateName = "custom"
	template := &v1alpha1.ProjectTemplate{Spec: v1alpha1.ProjectTemplateSpec{ResourcesTemplate: changeTemplate}}

	t.Run("a Helm text", func(t *testing.T) {
		t.Parallel()
		values := buildValues(project, template, templateParameters(project, template))
		change, err := c.releaseChange(project, helmTextRelease(helmTextManifest), buildChart(templates, project.Name), values, "", changeVersions)
		require.NoError(t, err)

		assert.False(t, change.Native)
		assert.True(t, change.Deployed.HelmText)
		assert.Equal(t, []ReleaseObject{
			{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "alice", Heritage: true},
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "moved", Heritage: true},
			{APIVersion: "v1", Kind: "Namespace", Name: "proj", Heritage: true},
			{APIVersion: "v1", Kind: "ResourceQuota", Namespace: "proj", Name: "all-pods", Heritage: true},
		}, change.Targets)
		assert.Empty(t, project.Status.Resources, "the status of the project is the upgrade's to fill")
	})

	t.Run("native manifests", func(t *testing.T) {
		t.Parallel()
		manifests := "---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: proj\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: native\n"
		change, err := c.releaseChange(project, helmTextRelease(helmTextManifest), buildEmptyChart(project.Name), map[string]any{}, manifests, changeVersions)
		require.NoError(t, err)

		assert.True(t, change.Native)
		assert.Equal(t, []ReleaseObject{
			{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "native", Heritage: true},
			{APIVersion: "v1", Kind: "Namespace", Name: "proj", Heritage: true},
		}, change.Targets)
	})
}

// staticRESTClientGetter serves the discovery of a fixed set of kinds and no REST mapping.
type staticRESTClientGetter struct {
	resources []*metav1.APIResourceList
}

func (g staticRESTClientGetter) ToRESTConfig() (*rest.Config, error) { return &rest.Config{}, nil }

func (g staticRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return cachedDiscovery{&fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: g.resources}}}, nil
}

func (g staticRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return meta.NewDefaultRESTMapper(nil), nil
}

// cachedDiscovery is a fake discovery that is always fresh.
type cachedDiscovery struct {
	*fakediscovery.FakeDiscovery
}

func (cachedDiscovery) Fresh() bool { return true }
func (cachedDiscovery) Invalidate() {}

// newReleaseClient is a client that releases into memory, over a cluster that serves Namespace and
// ConfigMap objects.
func newReleaseClient(t *testing.T) *Client {
	t.Helper()
	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)
	return &Client{
		templates: templates,
		logger:    ctrl.Log.WithName("test"),
		opts:      &options{HistoryMax: 3, Timeout: time.Second},
		conf: &action.Configuration{
			RESTClientGetter: staticRESTClientGetter{resources: []*metav1.APIResourceList{{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{{Name: "namespaces", Kind: "Namespace"}, {Name: "configmaps", Kind: "ConfigMap", Namespaced: true}},
			}}},
			Releases:     storage.Init(driver.NewMemory()),
			KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
			Capabilities: chartutil.DefaultCapabilities,
			Log:          func(string, ...any) {},
		},
	}
}

// BeforeUpgrade runs where an upgrade replaces a deployed revision, with that revision and the objects
// the release holds after the upgrade, and an error from it stops the upgrade. It does not run on the
// first install, nor on a reconcile that finds the release up to date.
func TestClient_ReleaseRunsBeforeUpgrade(t *testing.T) {
	t.Parallel()

	c := newReleaseClient(t)
	ctx := context.Background()
	project := &v1alpha3.Project{}
	project.Name = "proj"
	project.Spec.ProjectTemplateName = "custom"
	text := func(configMap string) *v1alpha1.ProjectTemplate {
		return &v1alpha1.ProjectTemplate{Spec: v1alpha1.ProjectTemplateSpec{
			ResourcesTemplate: "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + configMap + "\n",
		}}
	}

	var changes []ReleaseChange
	record := func(_ context.Context, change ReleaseChange) error {
		changes = append(changes, change)
		return nil
	}

	_, err := c.Upgrade(ctx, project, text("first"), record)
	require.NoError(t, err)
	assert.Empty(t, changes, "a first install replaces nothing")

	_, err = c.Upgrade(ctx, project, text("first"), record)
	require.NoError(t, err)
	assert.Empty(t, changes, "a release that is up to date is not upgraded")

	outcome, err := c.Upgrade(ctx, project, text("second"), record)
	require.NoError(t, err)
	assert.True(t, outcome.Applied)
	require.Len(t, changes, 1)
	assert.False(t, changes[0].Native)
	assert.True(t, changes[0].Deployed.HelmText)
	assert.Equal(t, 1, changes[0].Deployed.Revision)
	assert.Contains(t, changes[0].Deployed.Objects, ReleaseObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "first", Heritage: true})
	assert.Equal(t, []ReleaseObject{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "second", Heritage: true},
		{APIVersion: "v1", Kind: "Namespace", Name: "proj", Heritage: true},
	}, changes[0].Targets)

	current, err := c.CurrentRelease(ctx, project.Name)
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, 2, current.Revision)

	manifests := "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: native\n"
	refused := errors.New("refused")
	_, err = c.UpgradeManifests(ctx, project, manifests, func(context.Context, ReleaseChange) error { return refused })
	require.ErrorIs(t, err, refused)
	current, err = c.CurrentRelease(ctx, project.Name)
	require.NoError(t, err)
	assert.Equal(t, 2, current.Revision, "an error before the upgrade leaves the release as it is")

	_, err = c.UpgradeManifests(ctx, project, manifests, record)
	require.NoError(t, err)
	require.Len(t, changes, 2)
	assert.True(t, changes[1].Native)
	assert.True(t, changes[1].Deployed.HelmText, "the revision replaced is the one the Helm text rendered")

	current, err = c.CurrentRelease(ctx, project.Name)
	require.NoError(t, err)
	assert.False(t, current.HelmText)
	assert.Equal(t, 3, current.Revision)

	missing, err := c.CurrentRelease(ctx, "no-such-project")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

// A pending revision is rolled back before BeforeUpgrade runs, so BeforeUpgrade sees the revision the
// upgrade replaces: the rollback deploys the latest revision that is not pending, here a failed one
// newer than the deployed one.
func TestClient_ReleaseRollsBackBeforeUpgrade(t *testing.T) {
	t.Parallel()

	c := newReleaseClient(t)
	ctx := context.Background()
	project := &v1alpha3.Project{}
	project.Name = "proj"
	project.Spec.ProjectTemplateName = "custom"
	text := func(configMap string) *v1alpha1.ProjectTemplate {
		return &v1alpha1.ProjectTemplate{Spec: v1alpha1.ProjectTemplateSpec{
			ResourcesTemplate: "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + configMap + "\n",
		}}
	}
	_, err := c.Upgrade(ctx, project, text("first"), nil)
	require.NoError(t, err)
	_, err = c.Upgrade(ctx, project, text("second"), nil)
	require.NoError(t, err)

	deployed, err := c.conf.Releases.Get("proj", 2)
	require.NoError(t, err)
	revision := func(version int, status release.Status, configMap string) *release.Release {
		rel := *deployed
		rel.Version = version
		rel.Info = &release.Info{Status: status}
		rel.Manifest = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + configMap + "\n  namespace: proj\n"
		return &rel
	}
	require.NoError(t, c.conf.Releases.Create(revision(3, release.StatusFailed, "failed")))
	require.NoError(t, c.conf.Releases.Create(revision(4, release.StatusPendingUpgrade, "pending")))

	var changes []ReleaseChange
	_, err = c.UpgradeManifests(ctx, project, "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: native\n",
		func(_ context.Context, change ReleaseChange) error {
			changes = append(changes, change)
			return nil
		})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, 5, changes[0].Deployed.Revision, "the revision the rollback deployed")
	assert.Contains(t, changes[0].Deployed.Objects, ReleaseObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "failed"})
	assert.NotContains(t, changes[0].Deployed.Objects, ReleaseObject{APIVersion: "v1", Kind: "ConfigMap", Namespace: "proj", Name: "second"})
}
