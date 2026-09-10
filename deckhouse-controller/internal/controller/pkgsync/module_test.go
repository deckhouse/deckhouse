// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

func TestSyncEmbeddedModules(t *testing.T) {
	ctx := context.Background()

	t.Run("places every embedded module", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		writePackageYAML(t, filepath.Join(dir, "380-ingress-nginx"), "name: ingress-nginx\nweight: 380\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		assert.Equal(t, []string{"echo", "ingress-nginx"}, listModuleNames(t, cl))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "embedded", module.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.80.0", module.Spec.PackageVersion)
		assert.True(t, module.IsEmbedded(), "an embedded module carries the embedded annotation")
	})

	t.Run("the global module gets no object", func(t *testing.T) {
		dir := t.TempDir()
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\n", "type: object\n")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", dir, globalDir)
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleNames(t, cl), "the global module is placed by the package runtime, not here")
	})

	t.Run("a dir without a definition is skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "901-broken"), 0o755))

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		assert.Equal(t, []string{"echo"}, listModuleNames(t, cl))
	})

	t.Run("an existing module keeps the annotations of other writers", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "echo",
				Annotations: map[string]string{"en.meta.deckhouse.io/description": "echoes back"},
			},
			Spec: v1alpha2.ModuleSpec{
				PackageRepositoryName: "deckhouse-modules",
				PackageVersion:        "v1.2.3",
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", dir, existing)
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "embedded", module.Spec.PackageRepositoryName, "the embedded copy outranks the repository the module came from")
		assert.Equal(t, "v1.80.0", module.Spec.PackageVersion)
		assert.True(t, module.IsEmbedded())
		assert.Equal(t, "echoes back", module.Annotations["en.meta.deckhouse.io/description"])
	})

	t.Run("a second pass writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir)
		require.NoError(t, s.sync(ctx))

		version := getModule(t, cl, "echo").ResourceVersion

		require.NoError(t, s.sync(ctx))
		assert.Equal(t, version, getModule(t, cl, "echo").ResourceVersion)
	})
}

func TestSyncModulesFromModuleReleases(t *testing.T) {
	ctx := context.Background()

	t.Run("a deployed release names the repository and the version", func(t *testing.T) {
		release := testModuleRelease("echo", "deckhouse", "1.2.3", v1alpha1.ModuleReleasePhaseDeployed)

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), release)
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName, "the deckhouse source serves the deckhouse-modules repository")
		assert.Equal(t, "v1.2.3", module.Spec.PackageVersion)
		assert.False(t, module.IsEmbedded())
	})

	t.Run("the newest deployed release wins", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModuleRelease("echo", "example", "1.2.3", v1alpha1.ModuleReleasePhaseDeployed),
			testModuleRelease("echo", "example", "1.10.0", v1alpha1.ModuleReleasePhaseDeployed))
		require.NoError(t, s.sync(ctx))

		assert.Equal(t, "v1.10.0", getModule(t, cl, "echo").Spec.PackageVersion)
	})

	t.Run("a release that is not deployed places nothing", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModuleRelease("echo", "example", "1.2.3", v1alpha1.ModuleReleasePhasePending))
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleNames(t, cl))
	})

	t.Run("a release with no source or a broken version is skipped", func(t *testing.T) {
		sourceless := testModuleRelease("echo", "example", "1.2.3", v1alpha1.ModuleReleasePhaseDeployed)
		sourceless.Labels = nil

		broken := testModuleRelease("broken", "example", "not-a-semver", v1alpha1.ModuleReleasePhaseDeployed)

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), sourceless, broken)
		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModuleNames(t, cl))
	})

	t.Run("the embedded copy outranks a deployed release", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		s, cl := newTestSyncer(t, "v1.80.0", dir,
			testModuleRelease("echo", "example", "1.2.3", v1alpha1.ModuleReleasePhaseDeployed))
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "embedded", module.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.80.0", module.Spec.PackageVersion)
		assert.True(t, module.IsEmbedded())
	})

	t.Run("a module that left the image loses the embedded mark", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "echo",
				Annotations: map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"},
			},
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "embedded", PackageVersion: "v1.79.0"},
		}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), existing,
			testModuleRelease("echo", "example", "1.2.3", v1alpha1.ModuleReleasePhaseDeployed))
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
		assert.False(t, module.IsEmbedded())
	})
}

func TestSyncModulesFromModuleConfig(t *testing.T) {
	ctx := context.Background()

	echoOnDisk := func(t *testing.T) string {
		t.Helper()

		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		return dir
	}

	t.Run("mirrors the config into the spec", func(t *testing.T) {
		enabled := true
		moduleConfig := testModuleConfig("echo")
		moduleConfig.Spec.Enabled = &enabled
		moduleConfig.Spec.Version = 2
		moduleConfig.Spec.Maintenance = "NoResourceReconciliation"
		moduleConfig.Spec.Settings = &v1alpha1.MappedFields{Raw: []byte(`{"logLevel":"Debug"}`)}

		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t), moduleConfig)
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled)
		assert.Equal(t, 2, module.Spec.SettingsVersion)
		assert.Equal(t, "NoResourceReconciliation", module.Spec.Maintenance)
		require.NotNil(t, module.Spec.Settings)
		assert.JSONEq(t, `{"logLevel":"Debug"}`, string(module.Spec.Settings.Raw))
	})

	t.Run("clears the settings of a module with no config", func(t *testing.T) {
		enabled := true
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec: v1alpha2.ModuleSpec{
				Enabled:         &enabled,
				SettingsVersion: 2,
				Maintenance:     "NoResourceReconciliation",
				Settings:        &v1alpha1.MappedFields{Raw: []byte(`{"logLevel":"Debug"}`)},
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t), existing)
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Nil(t, module.Spec.Enabled)
		assert.Nil(t, module.Spec.Settings)
		assert.Zero(t, module.Spec.SettingsVersion)
		assert.Empty(t, module.Spec.Maintenance)
	})

	t.Run("a config under deletion counts as gone", func(t *testing.T) {
		enabled := true
		moduleConfig := testModuleConfig("echo")
		moduleConfig.Spec.Enabled = &enabled
		moduleConfig.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		moduleConfig.Finalizers = []string{"modules.deckhouse.io/test"}

		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t), moduleConfig)
		require.NoError(t, s.sync(ctx))

		assert.Nil(t, getModule(t, cl, "echo").Spec.Enabled)
	})
}

func testModuleConfig(name string) *v1alpha1.ModuleConfig {
	return &v1alpha1.ModuleConfig{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func listModuleNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	list := new(v1alpha2.ModuleList)
	require.NoError(t, cl.List(context.Background(), list))

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}

	return names
}

func getModule(t *testing.T, cl client.Client, name string) *v1alpha2.Module {
	t.Helper()

	module := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}
