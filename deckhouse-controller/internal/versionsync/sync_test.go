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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestSyncer(t *testing.T, version, embeddedDir string, objects ...client.Object) (*Syncer, client.Client) {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}).
		WithObjects(objects...).
		Build()

	return New(cl, cl, dependency.NewMockedContainer(), version, embeddedDir, log.NewNop()), cl
}

func writeModuleYAML(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "module.yaml"), []byte(content), 0o644))
}

func writePackageYAML(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"), []byte(content), 0o644))
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

func TestSyncEmbedded(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a complete version from package.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writePackageYAML(t, filepath.Join(dir, "900-echo"),
			"name: echo\nstage: General Availability\nweight: 900\ndescriptions:\n  en: en description\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		assert.Equal(t, "echo", mpv.Spec.PackageName)
		assert.Equal(t, "embedded", mpv.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.80.0", mpv.Spec.PackageVersion)
		assert.False(t, mpv.IsDraft(), "an embedded version must come out complete")
		assert.True(t, mpv.IsLegacy())
		assert.Empty(t, mpv.OwnerReferences, "no owner: no repository ever serves an embedded version")

		require.NotNil(t, mpv.Status.PackageMetadata)
		assert.Equal(t, "General Availability", mpv.Status.PackageMetadata.Stage)
		assert.Equal(t, int32(900), mpv.Status.PackageMetadata.Weight)
		assert.Equal(t, "en description", mpv.Status.PackageMetadata.Description.En)

		cond := metautils.FindStatusCondition(mpv.Status.Conditions, v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded)
		require.NotNil(t, cond)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, v1alpha1.ModulePackageVersionConditionReasonFilledFromDisk, cond.Reason)
	})

	t.Run("falls back to module.yaml and takes the weight from the dir prefix", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "910-parca"), "name: parca\nstage: Experimental\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-parca-v1.80.0")
		assert.False(t, mpv.IsDraft())
		require.NotNil(t, mpv.Status.PackageMetadata)
		assert.Equal(t, "Experimental", mpv.Status.PackageMetadata.Stage)
		assert.Equal(t, int32(910), mpv.Status.PackageMetadata.Weight, "the weight comes from the directory name")
	})

	t.Run("a dev build version is sanitized in the name only", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.78.0-pr22453+1b47ed2", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.78.0-pr22453-1b47ed2")
		assert.Equal(t, "v1.78.0-pr22453+1b47ed2", mpv.Spec.PackageVersion, "the spec keeps the raw version")
	})

	t.Run("skips dummy and unreadable dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "000-common"), "name: common\n")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "900-broken"), 0o755))

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		assert.Empty(t, listVersionNames(t, cl))
	})

	t.Run("completes an existing draft in place", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: General Availability\n")

		stub := &v1alpha1.ModulePackageVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: "embedded-echo-v1.80.0",
				Labels: map[string]string{
					v1alpha1.ModulePackageVersionLabelDraft:  "true",
					v1alpha1.ModulePackageVersionLabelLegacy: "true",
				},
			},
			Spec: v1alpha1.ModulePackageVersionSpec{
				PackageName:           "echo",
				PackageRepositoryName: "embedded",
				PackageVersion:        "v1.80.0",
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", dir, stub)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, stub.Name)
		assert.False(t, mpv.IsDraft(), "the leftover draft must be completed")
		require.NotNil(t, mpv.Status.PackageMetadata)
		assert.Equal(t, "General Availability", mpv.Status.PackageMetadata.Stage)
	})

	t.Run("keeps an existing complete version untouched", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: Experimental\n")

		existing := &v1alpha1.ModulePackageVersion{
			ObjectMeta: metav1.ObjectMeta{Name: "embedded-echo-v1.80.0"},
			Spec: v1alpha1.ModulePackageVersionSpec{
				PackageName:           "echo",
				PackageRepositoryName: "embedded",
				PackageVersion:        "v1.80.0",
			},
			Status: v1alpha1.ModulePackageVersionStatus{
				PackageMetadata: &v1alpha1.ModulePackageVersionStatusMetadata{Stage: "General Availability"},
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", dir, existing)
		before := getVersion(t, cl, existing.Name)

		require.NoError(t, s.Sync(ctx))

		after := getVersion(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
		assert.Equal(t, "General Availability", after.Status.PackageMetadata.Stage)
	})
}

func TestSyncReleases(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a draft stub for deployed and pending releases", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
			testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhasePending),
			testRelease("foo", "external", "2.0.0", v1alpha1.ModuleReleasePhaseDeployed),
		)

		require.NoError(t, s.Sync(ctx))

		assert.ElementsMatch(t, []string{
			"deckhouse-modules-parca-v1.4.3",
			"deckhouse-modules-console-v1.60.1",
			"external-foo-v2.0.0",
		}, listVersionNames(t, cl))

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

	t.Run("skips what names no version", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testRelease("parca", "deckhouse", "1.4.2", v1alpha1.ModuleReleasePhaseSuperseded),
			testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhaseSuspended),
			testRelease("orphan", "", "1.0.0", v1alpha1.ModuleReleasePhaseDeployed),
			testRelease("bad", "deckhouse", "latest", v1alpha1.ModuleReleasePhaseDeployed),
		)

		require.NoError(t, s.Sync(ctx))

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

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), existing,
			testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		)
		before := getVersion(t, cl, existing.Name)

		require.NoError(t, s.Sync(ctx))

		after := getVersion(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
		assert.False(t, after.IsDraft(), "a complete version must not become a draft")
	})
}

func TestSyncIsIdempotent(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: General Availability\n")

	s, cl := newTestSyncer(t, "v1.80.0", dir,
		testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhasePending),
	)

	require.NoError(t, s.Sync(ctx))

	versions := make(map[string]string)
	for _, name := range listVersionNames(t, cl) {
		versions[name] = getVersion(t, cl, name).ResourceVersion
	}
	require.Len(t, versions, 3)

	require.NoError(t, s.Sync(ctx))

	assert.Len(t, listVersionNames(t, cl), 3)
	for name, rv := range versions {
		assert.Equal(t, rv, getVersion(t, cl, name).ResourceVersion, name)
	}
}
