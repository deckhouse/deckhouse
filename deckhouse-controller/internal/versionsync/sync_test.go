// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package versionsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	return fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}).
		WithObjects(objects...).
		Build()
}

func testModule(name, source string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Properties: v1alpha1.ModuleProperties{Source: source},
	}
}

func testRelease(module, source, version, phase string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:   module + "-v" + version,
			Labels: map[string]string{"source": source},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: module, Version: version},
		Status: v1alpha1.ModuleReleaseStatus{Phase: phase},
	}
}

func listVersionNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	list := new(v1alpha1.ModulePackageVersionList)
	require.NoError(t, cl.List(context.Background(), list))

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}

	return names
}

func getVersion(t *testing.T, cl client.Client, name string) *v1alpha1.ModulePackageVersion {
	t.Helper()

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, mpv))

	return mpv
}

func TestSync(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a draft for every package of the old stack", func(t *testing.T) {
		cl := newTestClient(t,
			testModule("echo", v1alpha1.ModuleSourceEmbedded),
			testModule("parca", "deckhouse"),
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
			testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhasePending),
			testRelease("foo", "external", "2.0.0", v1alpha1.ModuleReleasePhaseDeployed),
		)

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		assert.ElementsMatch(t, []string{
			"embedded-echo-v1.80.0",
			"deckhouse-modules-parca-v1.4.3",
			"deckhouse-modules-console-v1.60.1",
			"external-foo-v2.0.0",
		}, listVersionNames(t, cl))
	})

	t.Run("a stub carries the scan labels and the release spec", func(t *testing.T) {
		cl := newTestClient(t, testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed))

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		mpv := getVersion(t, cl, "deckhouse-modules-parca-v1.4.3")
		assert.Equal(t, "parca", mpv.Spec.PackageName)
		assert.Equal(t, "deckhouse-modules", mpv.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.4.3", mpv.Spec.PackageVersion)
		assert.True(t, mpv.IsDraft(), "the stub must wait for the metadata as a draft")
		assert.True(t, mpv.IsLegacy())
		assert.Equal(t, "deckhouse", mpv.Labels["heritage"])
		assert.Equal(t, "deckhouse-modules", mpv.Labels[v1alpha1.ModulePackageVersionLabelRepository])
		assert.Equal(t, "parca", mpv.Labels[v1alpha1.ModulePackageVersionLabelPackage])
		assert.Empty(t, mpv.OwnerReferences, "no owner until a repository adopts the version")
		assert.Nil(t, mpv.Status.PackageMetadata)
	})

	t.Run("a dev build version is sanitized in the name only", func(t *testing.T) {
		cl := newTestClient(t, testModule("echo", v1alpha1.ModuleSourceEmbedded))

		require.NoError(t, Sync(ctx, cl, cl, "v1.78.0-pr22453+1b47ed2", log.NewNop()))

		mpv := getVersion(t, cl, "embedded-echo-v1.78.0-pr22453-1b47ed2")
		assert.Equal(t, "embedded", mpv.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.78.0-pr22453+1b47ed2", mpv.Spec.PackageVersion, "the spec keeps the raw version")
	})

	t.Run("skips what names no version", func(t *testing.T) {
		cl := newTestClient(t,
			testModule("parca", "deckhouse"),
			testRelease("parca", "deckhouse", "1.4.2", v1alpha1.ModuleReleasePhaseSuperseded),
			testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhaseSuspended),
			testRelease("orphan", "", "1.0.0", v1alpha1.ModuleReleasePhaseDeployed),
			testRelease("bad", "deckhouse", "latest", v1alpha1.ModuleReleasePhaseDeployed),
		)

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		assert.Empty(t, listVersionNames(t, cl))
	})

	t.Run("keeps an existing version untouched", func(t *testing.T) {
		existing := &v1alpha1.ModulePackageVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "deckhouse-modules-parca-v1.4.3",
				Labels: map[string]string{v1alpha1.ModulePackageVersionLabelRepository: "deckhouse-modules"},
			},
			Spec: v1alpha1.ModulePackageVersionSpec{
				PackageName:           "parca",
				PackageRepositoryName: "deckhouse-modules",
				PackageVersion:        "v1.4.3",
			},
			Status: v1alpha1.ModulePackageVersionStatus{
				PackageMetadata: &v1alpha1.ModulePackageVersionStatusMetadata{Weight: 910},
			},
		}
		cl := newTestClient(t, existing,
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		)
		before := getVersion(t, cl, existing.Name)

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		after := getVersion(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
		assert.False(t, after.IsDraft(), "a complete version must not become a draft")
		require.NotNil(t, after.Status.PackageMetadata)
		assert.Equal(t, int32(910), after.Status.PackageMetadata.Weight)
	})

	t.Run("a second run changes nothing", func(t *testing.T) {
		cl := newTestClient(t,
			testModule("echo", v1alpha1.ModuleSourceEmbedded),
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
			testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhasePending),
		)

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		versions := make(map[string]string)
		for _, name := range listVersionNames(t, cl) {
			versions[name] = getVersion(t, cl, name).ResourceVersion
		}
		require.Len(t, versions, 3)

		require.NoError(t, Sync(ctx, cl, cl, "v1.80.0", log.NewNop()))

		assert.Len(t, listVersionNames(t, cl), 3)
		for name, rv := range versions {
			assert.Equal(t, rv, getVersion(t, cl, name).ResourceVersion, name)
		}
	})
}
