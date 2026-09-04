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

package pkgsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestSyncVersionsFromImage(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a complete version from package.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writePackageYAML(t, filepath.Join(dir, "900-echo"),
			"name: echo\nstage: General Availability\nweight: 900\ndescriptions:\n  en: en description\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

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
		require.NoError(t, s.sync(ctx))

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
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleVersionNames(t, cl))
	})

	t.Run("multi-type schema fields parse", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		// real modules spell alternatives this way, e.g. cni-cilium and node-manager
		writeOpenAPI(t, filepath.Join(dir, "900-echo"),
			"type: object\nproperties:\n  timeout:\n    type: [integer, string]\n",
			"type: object\nproperties:\n  policies:\n    type: ['null', array]\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		require.NotNil(t, mpv.Status.PackageSchemas, "a multi-type field is valid JSON Schema, the version must not be skipped")
		timeout := mpv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties["timeout"]
		assert.Equal(t, openapi.StringOrArray{"integer", "string"}, timeout.Type)
		policies := mpv.Status.PackageSchemas.ValuesSchema.OpenAPIV3Schema.Properties["policies"]
		assert.Equal(t, openapi.StringOrArray{"null", "array"}, policies.Type)
	})

	t.Run("keeps CEL validation expressions", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"),
			"type: object\nproperties:\n  replicas:\n    type: integer\nx-deckhouse-validations:\n  - expression: \"self.replicas >= 1\"\n    message: \"replicas must be >= 1\"\n",
			"")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		require.NotNil(t, mpv.Status.PackageSchemas)
		rules := mpv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.XValidations
		require.Len(t, rules, 1)
		assert.Equal(t, "self.replicas >= 1", rules[0].Expression, "the expression key of the schema files must land in the stored schema")
		assert.Equal(t, "replicas must be >= 1", rules[0].Message)
	})

	t.Run("carries the disable options of a module.yaml module", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"),
			"name: echo\ndisable:\n  confirmation: true\n  message: \"fallback text\"\n  messages:\n    ru: \"ru text\"\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		opts := mpv.Status.PackageMetadata.DisableOptions
		require.NotNil(t, opts, "the module.yaml disable section must survive")
		assert.True(t, opts.Confirmation)
		require.NotNil(t, opts.Messages)
		assert.Equal(t, "ru text", opts.Messages.Ru)
		assert.Equal(t, "fallback text", opts.Messages.En, "the deprecated message fills the missing language")
	})

	t.Run("parent module dependencies come out sorted by name", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"),
			"name: echo\nrequirements:\n  modules:\n    delta: \">= 1\"\n    alpha: \">= 1\"\n    charlie: \">= 1\"\n    bravo: \">= 1\"\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.80.0")
		require.NotNil(t, mpv.Status.PackageMetadata.Requirements)
		require.NotNil(t, mpv.Status.PackageMetadata.Requirements.Modules)
		names := make([]string, 0, 4)
		for _, dep := range mpv.Status.PackageMetadata.Requirements.Modules.Mandatory {
			names = append(names, dep.Name)
		}
		assert.Equal(t, []string{"alpha", "bravo", "charlie", "delta"}, names,
			"a stable order keeps the converge from seeing false drift")
	})

	t.Run("falls back to module.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "910-parca"), "name: parca\nstage: Experimental\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-parca-v1.80.0")
		assert.False(t, mpv.IsDraft())
		require.NotNil(t, mpv.Status.PackageMetadata)
		assert.Equal(t, "Experimental", mpv.Status.PackageMetadata.Stage)
		assert.Nil(t, mpv.Status.PackageMetadata.Description, "no descriptions in the file, no description block")
	})

	t.Run("a build version is normalized in the name and the spec", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.78.0-pr22453+1b47ed2", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v1.78.0")
		assert.Equal(t, "v1.78.0", mpv.Spec.PackageVersion, "prerelease and metadata are dropped")
	})

	t.Run("a dev binary counts as v2.0.0", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "dev", dir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-echo-v2.0.0")
		assert.Equal(t, "v2.0.0", mpv.Spec.PackageVersion)
	})

	t.Run("a non-semver deckhouse version names the version verbatim", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "latest", dir)
		require.NoError(t, s.sync(ctx))

		// the bootstrap places the module on the same string, so both sides still agree
		mpv := getVersion(t, cl, "embedded-echo-latest")
		assert.Equal(t, "latest", mpv.Spec.PackageVersion)
	})

	t.Run("a deckhouse version that is no legal object name skips the version", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "Latest_Build", dir)
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listVersionNames(t, cl))
	})

	t.Run("skips dummy and unreadable dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "000-common"), "name: common\n")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "900-broken"), 0o755))

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleVersionNames(t, cl))
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
		require.NoError(t, s.sync(ctx))

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

		require.NoError(t, s.sync(ctx))

		after := getVersion(t, cl, existing.Name)
		assert.False(t, after.IsDraft(), "a refresh must not bring the draft label back")
		assert.Equal(t, "Experimental", after.Status.PackageMetadata.Stage, "the status follows the disk")
		require.NotNil(t, after.Status.PackageSchemas, "the refresh brings the schemas along")
		assert.NotNil(t, after.Status.PackageSchemas.SettingsSchema)
	})

	t.Run("keeps a version matching the disk untouched", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: Experimental\n")
		writeOpenAPI(t, filepath.Join(dir, "900-echo"), "type: object\n", "")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))
		before := getVersion(t, cl, "embedded-echo-v1.80.0")

		require.NoError(t, s.sync(ctx))

		after := getVersion(t, cl, before.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion, "a no-change pass rewrites nothing")
	})
}

func TestSyncGlobalVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a complete version from the legacy openapi files", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir,
			"type: object\nproperties:\n  modules:\n    type: object\n",
			"type: object\nproperties:\n  discovery:\n    type: object\n")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-global-v1.80.0")
		assert.Equal(t, "global", mpv.Spec.PackageName)
		assert.Equal(t, "embedded", mpv.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.80.0", mpv.Spec.PackageVersion)
		assert.False(t, mpv.IsDraft(), "the global version must come out complete")
		assert.True(t, mpv.IsLegacy())

		require.NotNil(t, mpv.Status.PackageSchemas)
		require.NotNil(t, mpv.Status.PackageSchemas.SettingsSchema)
		assert.Contains(t, mpv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties, "modules")
		require.NotNil(t, mpv.Status.PackageSchemas.ValuesSchema)
		assert.Contains(t, mpv.Status.PackageSchemas.ValuesSchema.OpenAPIV3Schema.Properties, "discovery")

		cond := metautils.FindStatusCondition(mpv.Status.Conditions, v1alpha1.ModulePackageVersionConditionTypeMetadataLoaded)
		require.NotNil(t, cond)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
	})

	t.Run("carries empty metadata", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\n", "")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-global-v1.80.0")
		require.NotNil(t, mpv.Status.PackageMetadata, "an empty object, not none: readers reach into it")
		assert.Equal(t, new(v1alpha1.ModulePackageVersionStatusMetadata), mpv.Status.PackageMetadata,
			"the global hooks dir holds no definition to fill any of it from")
	})

	t.Run("creates the catalog entry no repository offers", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\n", "")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))

		pkg := getPackage(t, cl, "global")
		assert.Equal(t, map[string]string{"heritage": "deckhouse"}, pkg.Labels)
	})

	t.Run("prefers settings.yaml over the legacy name", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\nproperties:\n  legacy:\n    type: string\n", "")
		writeOpenAPI(t, globalDir, "type: object\nproperties:\n  modern:\n    type: string\n", "")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-global-v1.80.0")
		require.NotNil(t, mpv.Status.PackageSchemas.SettingsSchema)
		assert.Contains(t, mpv.Status.PackageSchemas.SettingsSchema.OpenAPIV3Schema.Properties, "modern")
	})

	t.Run("a dir holding no schemas still names the package and the version", func(t *testing.T) {
		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), t.TempDir())
		require.NoError(t, s.sync(ctx))

		// the image always ships the schemas; withholding the objects over a dir that
		// somehow holds none would leave the module controller unable to register global
		mpv := getVersion(t, cl, "embedded-global-v1.80.0")
		assert.Nil(t, mpv.Status.PackageSchemas, "no schemas on disk, none in the status")
		assert.False(t, mpv.IsDraft(), "nothing is left to fill in, so the version is complete")
		assert.NotEmpty(t, getPackage(t, cl, "global"))
	})

	t.Run("a missing global hooks dir still names the package and the version", func(t *testing.T) {
		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), filepath.Join(t.TempDir(), "absent"))
		require.NoError(t, s.sync(ctx))

		mpv := getVersion(t, cl, "embedded-global-v1.80.0")
		assert.Nil(t, mpv.Status.PackageSchemas)
	})

	t.Run("a broken schema skips the version", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "{not a schema", "")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listVersionNames(t, cl), "an unparsable schema is a broken image, not an empty one")
		assert.Empty(t, listPackageNames(t, cl), "no version, no catalog entry either")
	})

	t.Run("keeps a version matching the disk untouched", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\n", "")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir)
		require.NoError(t, s.sync(ctx))
		before := getVersion(t, cl, "embedded-global-v1.80.0")

		require.NoError(t, s.sync(ctx))

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

		require.NoError(t, s.sync(ctx))

		assert.ElementsMatch(t, []string{
			"deckhouse-modules-parca-v1.4.3",
			"deckhouse-modules-console-v1.60.1",
			"external-foo-v2.0.0",
		}, listModuleVersionNames(t, cl))

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

		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleVersionNames(t, cl))
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

		require.NoError(t, s.sync(ctx))

		after := getVersion(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
		assert.False(t, after.IsDraft(), "a complete version must not become a draft")
	})
}
