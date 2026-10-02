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

package gc

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

func release(name, version, phase string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "DeckhouseRelease",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"version": version},
		"status":     map[string]any{"phase": phase},
	}}
	return object
}

func releaseClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "DeckhouseRelease"},
		&unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(releaseGVK, &unstructured.UnstructuredList{})

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestFromCluster(t *testing.T) {
	c := releaseClient(t,
		release("v1-76-6", "v1.76.6", phaseDeployed),
		release("v1-75-0", "v1.75.0", phaseSuperseded),
		release("v1-74-3", "v1.74.3", phaseSuperseded),
		release("v1-77-0", "v1.77.0", "Pending"),
	)

	releases, err := FromCluster(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "v1.76.6", releases.Deployed)
	assert.Equal(t, "v1.75.0", releases.Previous)
}

// TestFromClusterPicksTheMostRecentSuperseded is the ordering trap: the API returns objects in
// name order, names are versions as strings, and v1.9.0 sorts after v1.10.0 that way.
func TestFromClusterPicksTheMostRecentSuperseded(t *testing.T) {
	c := releaseClient(t,
		release("a", "v1.76.6", phaseDeployed),
		release("b", "v1.10.0", phaseSuperseded),
		release("c", "v1.9.0", phaseSuperseded),
	)

	releases, err := FromCluster(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "v1.10.0", releases.Previous, "the previous release was picked by string order")
}

// TestFromClusterWithoutADeployedRelease: the collector needs that version to justify any
// deletion, and an empty answer would read as "keep nothing".
func TestFromClusterWithoutADeployedRelease(t *testing.T) {
	c := releaseClient(t, release("a", "v1.75.0", phaseSuperseded))

	_, err := FromCluster(context.Background(), c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no release is deployed")
}

func TestFromClusterWithNoReleasesAtAll(t *testing.T) {
	_, err := FromCluster(context.Background(), releaseClient(t))
	require.Error(t, err)
}

// TestFromClusterWithoutAPreviousRelease is a cluster that has never updated. One version to
// judge against, and that is not an error.
func TestFromClusterWithoutAPreviousRelease(t *testing.T) {
	c := releaseClient(t, release("a", "v1.76.6", phaseDeployed))

	releases, err := FromCluster(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "v1.76.6", releases.Deployed)
	assert.Empty(t, releases.Previous)
}

// TestFromClusterIgnoresReleasesWithoutAVersion keeps a malformed object from becoming the
// version everything else is judged against.
func TestFromClusterIgnoresReleasesWithoutAVersion(t *testing.T) {
	broken := release("broken", "", phaseDeployed)
	unstructured.RemoveNestedField(broken.Object, "spec", "version")

	c := releaseClient(t, broken, release("good", "v1.76.6", phaseDeployed))

	releases, err := FromCluster(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, "v1.76.6", releases.Deployed)
}

// deployment is how a cluster says what it runs when no release object does.
func deployment(image string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "deckhouse", "namespace": "d8-system"},
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "kube-rbac-proxy", "image": "example.com/proxy:v1"},
				map[string]any{"name": "deckhouse", "image": image},
			},
		}}},
	}}
}

// TestFromClusterFallsBackToTheRunningImage is the case that left a whole class of clusters with
// an empty cache.
//
// A release object is not the only way to be running a version. On a cluster installed from a
// development or pull-request image none is ever deployed, and this function used to fail on every
// pass: every replica reported "no release is deployed" and the store held nothing at all. The cache
// was empty by construction on the very axis the module exists for.
//
// The fallback answers with the same set by a shorter route: the version is only used to find the
// installer image that declares the image set, the same file `d8 mirror pull` reads, and the running
// image names its own installer.
func TestFromClusterFallsBackToTheRunningImage(t *testing.T) {
	t.Run("no release at all", func(t *testing.T) {
		c := releaseClient(t, deployment("registry.d8-system.svc:5001/system/deckhouse:pr1234"))

		releases, err := FromCluster(context.Background(), c)
		require.NoError(t, err)
		assert.Equal(t, "pr1234", releases.Deployed,
			"the tag names the installer image that declares the set")
		assert.Empty(t, releases.Previous, "there is no previous version to roll back to")
	})

	t.Run("releases exist but none is deployed", func(t *testing.T) {
		c := releaseClient(t,
			release("a", "v1.75.0", phaseSuperseded),
			deployment("registry.d8-system.svc:5001/system/deckhouse:v1.78.0"),
		)

		releases, err := FromCluster(context.Background(), c)
		require.NoError(t, err)
		assert.Equal(t, "v1.78.0", releases.Deployed)
		assert.Equal(t, "v1.75.0", releases.Previous,
			"a superseded release is still worth keeping for a rollback")
	})

	t.Run("a deployed release still wins", func(t *testing.T) {
		c := releaseClient(t,
			release("a", "v1.76.6", phaseDeployed),
			deployment("registry.d8-system.svc:5001/system/deckhouse:pr1234"),
		)

		releases, err := FromCluster(context.Background(), c)
		require.NoError(t, err)
		assert.Equal(t, "v1.76.6", releases.Deployed,
			"where the cluster records its version, that record is the answer")
	})

	t.Run("a digest-pinned image cannot answer", func(t *testing.T) {
		c := releaseClient(t, deployment("registry.d8-system.svc:5001/system/deckhouse@sha256:abc"))

		_, err := FromCluster(context.Background(), c)
		require.Error(t, err, "an unknown set must not justify deleting anything")
		assert.Contains(t, err.Error(), "names no tag")
	})

	t.Run("no deployment either", func(t *testing.T) {
		_, err := FromCluster(context.Background(), releaseClient(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no release is deployed")
	})
}

// TestImageTagTellsAPortFromATag: the two colons in a reference mean different things, and reading
// the wrong one would send the fill looking for an installer named after a port number.
func TestImageTagTellsAPortFromATag(t *testing.T) {
	tests := []struct {
		image string
		want  string
		ok    bool
	}{
		{image: "registry.d8-system.svc:5001/system/deckhouse:v1.78.0", want: "v1.78.0", ok: true},
		{image: "registry.example.com/deckhouse/ee:pr123", want: "pr123", ok: true},
		{image: "deckhouse:stable", want: "stable", ok: true},
		// A port with no tag must not be read as one.
		{image: "registry.d8-system.svc:5001/system/deckhouse", want: "", ok: false},
		{image: "registry.d8-system.svc:5001/system/deckhouse@sha256:abc", want: "", ok: false},
		{image: "", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			tag, ok := imageTag(tt.image)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, tag)
		})
	}
}

func moduleRelease(name, module, version, phase, source string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleRelease",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"moduleName": module, "version": version},
		"status":     map[string]any{"phase": phase},
	}}
	if source != "" {
		object.SetLabels(map[string]string{"source": source})
	}
	return object
}

func moduleSource(name, repo string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"registry": map[string]any{"repo": repo}},
	}}
}

func moduleClient(t *testing.T, withSources bool, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleRelease"},
		&unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(moduleReleaseGVK, &unstructured.UnstructuredList{})
	if withSources {
		scheme.AddKnownTypeWithName(
			schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleSource"},
			&unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(moduleSourceGVK, &unstructured.UnstructuredList{})
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func names(modules []fill.ModuleRef) []string {
	out := make([]string, 0, len(modules))
	for _, module := range modules {
		out = append(out, module.Name+"@"+module.Version+"<"+module.Source.Name)
	}
	sort.Strings(out)
	return out
}

// TestEveryModuleIsPartOfTheSetWithItsSource: a module the operator installed from a source of their
// own is one the cluster needs, and after the move to air-gap the store is the only place left to
// get it from. So it is kept, and what its source says travels with it.
func TestEveryModuleIsPartOfTheSetWithItsSource(t *testing.T) {
	c := moduleClient(t, true,
		moduleSource("deckhouse", "registry.d8-system.svc:5001/system/deckhouse/modules"),
		moduleSource("sds-node-configurator", "registry.deckhouse.io/deckhouse/ee/modules"),

		moduleRelease("console-v1.2.0", "console", "1.2.0", "Deployed", "deckhouse"),
		moduleRelease("sds-node-configurator-v0.7.5", "sds-node-configurator", "v0.7.5", "Deployed", "sds-node-configurator"),
		moduleRelease("stronghold-v1.0.0", "stronghold", "1.0.0", "Deployed", ""),
		moduleRelease("sds-node-configurator-v0.7.4", "sds-node-configurator", "v0.7.4", "Superseded", "sds-node-configurator"),
	)

	modules, err := ModulesFromCluster(context.Background(), c)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"console@v1.2.0<deckhouse",
		"sds-node-configurator@v0.7.5<sds-node-configurator",
		"stronghold@v1.0.0<",
	}, names(modules))
	for _, module := range modules {
		if module.Name == "sds-node-configurator" {
			assert.Equal(t, "registry.deckhouse.io/deckhouse/ee/modules", module.Source.Repository)
		}
	}
}

// TestWithoutSourcesEveryModuleIsKept: sources that cannot be read leave the modules without one,
// which a fill reads from the upstream, as it did before sources were looked at.
func TestWithoutSourcesEveryModuleIsKept(t *testing.T) {
	c := moduleClient(t, false,
		moduleRelease("console-v1.2.0", "console", "1.2.0", "Deployed", "deckhouse"),
		moduleRelease("sds-node-configurator-v0.7.5", "sds-node-configurator", "v0.7.5", "Deployed", "sds-node-configurator"),
	)

	modules, err := ModulesFromCluster(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, []string{"console@v1.2.0<", "sds-node-configurator@v0.7.5<"}, names(modules))
}
