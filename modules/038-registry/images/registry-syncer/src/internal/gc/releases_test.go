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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
