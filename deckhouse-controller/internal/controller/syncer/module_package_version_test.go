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

package syncer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestEmbeddedVersion(t *testing.T) {
	cases := []struct {
		version string
		want    string
		ok      bool
	}{
		{version: "v1.78.0", want: "v1.78.0", ok: true},
		{version: "v1.78.0-pr22189+8776a42", want: "v1.78.0", ok: true},
		{version: "v1.0.0-RC1", want: "v1.0.0", ok: true},
		{version: "dev", want: "v2.0.0", ok: true},
		{version: "latest", want: "", ok: false},
		{version: "", want: "", ok: false},
	}

	for _, c := range cases {
		s := &Syncer{deckhouseVersion: c.version, logger: log.NewNop()}

		got, ok := s.embeddedVersion()
		assert.Equal(t, c.ok, ok, c.version)
		assert.Equal(t, c.want, got, c.version)
	}
}

func TestSyncVersionsFromImage(t *testing.T) {
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
		assert.Equal(t, "Succeeded", cond.Reason)

		assert.Nil(t, mpv.Status.PackageSchemas, "no openapi dir, no schemas")
	})

	t.Run("fills the settings and values schemas from openapi", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"),
			"type: object\nproperties:\n  logLevel:\n    type: string\n",
			"type: object\nproperties:\n  internal:\n    type: object\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		require.NotNil(t, mpv.Status.PackageSchemas)
		require.NotNil(t, mpv.Status.PackageSchemas.SettingsSchema)
		assert.Contains(t, mpv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties, "logLevel")
		require.NotNil(t, mpv.Status.PackageSchemas.ValuesSchema)
		assert.Contains(t, mpv.Status.PackageSchemas.ValuesSchema.OpenAPIV3Schema.Properties, "internal")
	})

	t.Run("a broken schema skips the version", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"), "{not a schema", "")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		assert.Empty(t, listVersionNames(t, cl))
	})

	t.Run("carries the disable options of a module.yaml module", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"),
			"name: echo\ndisable:\n  confirmation: true\n  message: \"fallback text\"\n  messages:\n    ru: \"ru text\"\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		opts := mpv.Status.PackageMetadata.DisableOptions
		require.NotNil(t, opts, "the module.yaml disable section must survive")
		assert.True(t, opts.Confirmation)
		require.NotNil(t, opts.Messages)
		assert.Equal(t, "ru text", opts.Messages.Ru)
		assert.Equal(t, "fallback text", opts.Messages.En, "the deprecated message fills the missing language")
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
		assert.Nil(t, mpv.Status.PackageMetadata.Description, "no descriptions in the file, no description block")
	})

	t.Run("a build version is normalized in the name and the spec", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.78.0-pr22453+1b47ed2", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.78.0")
		assert.Equal(t, "v1.78.0", mpv.Spec.PackageVersion, "prerelease and metadata are dropped")
	})

	t.Run("a dev binary counts as v2.0.0", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "dev", dir)
		require.NoError(t, s.Sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v2.0.0")
		assert.Equal(t, "v2.0.0", mpv.Spec.PackageVersion)
	})

	t.Run("a broken deckhouse version skips the embedded sync", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "latest", dir)
		require.NoError(t, s.Sync(ctx))

		assert.Empty(t, listVersionNames(t, cl))
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

	t.Run("refreshes a complete version whose status drifted from the disk", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: Experimental\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"), "type: object\n", "")

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

		require.NoError(t, s.Sync(ctx))

		after := getVersion(t, cl, existing.Name)
		assert.False(t, after.IsDraft(), "a refresh must not bring the draft label back")
		assert.Equal(t, "Experimental", after.Status.PackageMetadata.Stage, "the status follows the disk")
		assert.Equal(t, int32(900), after.Status.PackageMetadata.Weight)
		require.NotNil(t, after.Status.PackageSchemas, "the refresh brings the schemas along")
		assert.NotNil(t, after.Status.PackageSchemas.SettingsSchema)
	})

	t.Run("keeps a version matching the disk untouched", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: Experimental\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"), "type: object\n", "")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.Sync(ctx))
		before := getVersion(t, cl, "embedded-echo-v1.80.0")

		require.NoError(t, s.Sync(ctx))

		after := getVersion(t, cl, before.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion, "a no-change pass rewrites nothing")
	})
}

func TestSyncVersionsFromReleases(t *testing.T) {
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
