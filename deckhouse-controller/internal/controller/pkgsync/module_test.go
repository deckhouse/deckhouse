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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// testVersion is the Deckhouse version the module tests run under; embedded
// module specs carry its normalized form, "v1.77.0".
const testVersion = "v1.77.0-test"

func writeEmbeddedModule(t *testing.T, embeddedDir, dirName, moduleName string) {
	t.Helper()

	moduleDir := filepath.Join(embeddedDir, dirName)
	require.NoError(t, os.MkdirAll(moduleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "module.yaml"),
		[]byte("name: "+moduleName+"\nweight: 380\n"), 0o644))
}

func testReadyPullOverride(name, imageTag string) *v1alpha2.ModulePullOverride {
	return &v1alpha2.ModulePullOverride{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha2.ModulePullOverrideSpec{ImageTag: imageTag},
		Status:     v1alpha2.ModulePullOverrideStatus{Message: v1alpha1.ModulePullOverrideMessageReady},
	}
}

func testDeployedRelease(module, sourceName, version string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: module + "-v" + version,
			Labels: map[string]string{
				"module": module,
				"source": sourceName,
			},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: module, Version: version},
		Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhaseDeployed},
	}
}

func testModuleConfig(name string) *v1alpha1.ModuleConfig {
	enabled := true

	return &v1alpha1.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled:      &enabled,
			Version:      2,
			Settings:     v1alpha1.MakeMappedFields(map[string]any{"logLevel": "Debug"}),
			UpdatePolicy: "test-alpha",
		},
	}
}

func getV2Module(t *testing.T, cl client.Client, name string) *v1alpha2.Module {
	t.Helper()

	module := new(v1alpha2.Module)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}

func TestSyncOriginPrecedence(t *testing.T) {
	t.Run("ready pull override beats deployed release", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testReadyPullOverride("echo", "dev-tag"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName,
			"the repository comes from the module's deployed release")
		assert.True(t, module.IsDev(), "override-owned module must carry the dev annotation")
		assert.False(t, module.IsEmbedded())
	})

	t.Run("embedded module beats a deployed release", func(t *testing.T) {
		embeddedDir := t.TempDir()
		writeEmbeddedModule(t, embeddedDir, "380-echo", "echo")

		s, cl := newTestSyncer(t, testVersion, embeddedDir,
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, repositoryNameEmbedded, module.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.77.0", module.Spec.PackageVersion,
			"the spec carries the version the embedded package version is named with")
		assert.True(t, module.IsEmbedded())

		assert.Contains(t, listVersionNames(t, cl), "embedded-echo-v1.77.0",
			"the spec triple must compose the embedded version name")
	})

	t.Run("a non-semver build version fills the spec verbatim", func(t *testing.T) {
		embeddedDir := t.TempDir()
		writeEmbeddedModule(t, embeddedDir, "380-echo", "echo")

		s, cl := newTestSyncer(t, "latest", embeddedDir)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "latest", module.Spec.PackageVersion,
			"a version that names no package version must still place the module")
		assert.Empty(t, listVersionNames(t, cl))
	})

	t.Run("newest deployed release wins and supersedes the older one", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testDeployedRelease("echo", "example", "1.0.0"),
			testDeployedRelease("echo", "example", "1.1.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.1.0", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)

		older := new(v1alpha1.ModuleRelease)
		require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: "echo-v1.0.0"}, older))
		assert.Equal(t, v1alpha1.ModuleReleasePhaseSuperseded, older.Status.Phase)

		assert.ElementsMatch(t, []string{"example-echo-v1.1.0"}, listVersionNames(t, cl),
			"a superseded release names no version stub")
	})
}

func TestSyncModules(t *testing.T) {
	enabled := true

	t.Run("creates a module with a known origin and config fields", func(t *testing.T) {
		conf := &v1alpha1.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec: v1alpha1.ModuleConfigSpec{
				Enabled:      &enabled,
				Version:      2,
				Settings:     v1alpha1.MakeMappedFields(map[string]any{"logLevel": "Debug"}),
				Maintenance:  "NoResourceReconciliation",
				UpdatePolicy: "test-alpha",
			},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testDeployedRelease("echo", "example", "1.0.0"),
			conf,
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(t, 2, module.Spec.SettingsVersion)
		assert.Equal(t, "NoResourceReconciliation", module.Spec.Maintenance)
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled)
		assert.Equal(t, map[string]any{"logLevel": "Debug"}, module.Spec.Settings.GetMap())
	})

	t.Run("patches an existing module to the origin version", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v0.9.0"},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			existing,
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
	})

	t.Run("deletes an orphaned module in bootstrap mode", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(), existing)
		WithOrphanDeletion()(s)

		syncOK(t, s)

		module := new(v1alpha2.Module)
		err := cl.Get(context.Background(), client.ObjectKey{Name: "echo"}, module)
		assert.True(t, client.IgnoreNotFound(err) == nil && err != nil, "an orphaned module must be deleted in bootstrap mode")
	})

	t.Run("leaves an orphaned module to the module stack by default", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(), existing)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Empty(t, module.Spec.PackageVersion, "the sync must not touch a module it does not own")
	})

	t.Run("deckhouse source maps to the deckhouse-modules repository", func(t *testing.T) {
		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			testDeployedRelease("parca", "deckhouse", "1.4.3"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "parca")
		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName)

		name := v1alpha1.MakeModulePackageVersionName(module.Spec.PackageRepositoryName, module.Name, module.Spec.PackageVersion)
		assert.Contains(t, listVersionNames(t, cl), name, "the spec triple must compose the stub name")
	})

	t.Run("heals a repository recorded under its source name", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "parca"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse", PackageVersion: "v1.4.3"},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(), existing)

		syncOK(t, s)

		module := getV2Module(t, cl, "parca")
		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName,
			"a raw source name left by an older write must heal")
	})

	t.Run("clears the dev annotation once the override is gone", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "echo",
				Annotations: map[string]string{v1alpha2.ModuleAnnotationDev: "true"},
			},
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "dev-tag"},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(),
			existing,
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.False(t, module.IsDev(), "the release origin must clear the leftover dev annotation")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
	})

	t.Run("keeps a module no source claims but a repository backs", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		s, cl := newTestSyncer(t, testVersion, t.TempDir(), existing)

		syncOK(t, s)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion, "a module another writer placed must survive")
	})
}

func TestFillModuleV2(t *testing.T) {
	t.Run("module of unknown origin keeps its spec", func(t *testing.T) {
		module := &v1alpha2.Module{
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		fillModuleV2(module, Origin{}, nil)

		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
	})

	t.Run("embedded annotation is reconciled both ways", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: repositoryNameEmbedded, PackageVersion: "v1.77.0", Embedded: true}, nil)
		assert.True(t, module.IsEmbedded())

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, nil)
		assert.False(t, module.IsEmbedded(), "a module no longer shipped in the image loses the annotation")
	})

	t.Run("unknown origin leaves the annotations alone", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: repositoryNameEmbedded, PackageVersion: "v1.77.0", Embedded: true}, nil)
		require.True(t, module.IsEmbedded())

		fillModuleV2(module, Origin{}, testModuleConfig("echo"))
		assert.True(t, module.IsEmbedded(), "a config mirror must not strip the embedded annotation")
	})

	t.Run("dev annotation is reconciled both ways", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "dev-tag", Dev: true}, nil)
		assert.True(t, module.IsDev())

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, nil)
		assert.False(t, module.IsDev(), "a module no longer overridden loses the dev annotation")
	})

	t.Run("unknown origin keeps the dev annotation", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "dev-tag", Dev: true}, nil)
		require.True(t, module.IsDev())

		fillModuleV2(module, Origin{}, testModuleConfig("echo"))
		assert.True(t, module.IsDev(), "a config mirror must not strip the dev annotation")
	})

	t.Run("a half-filled origin is not known and leaves the spec alone", func(t *testing.T) {
		module := &v1alpha2.Module{
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		// an override whose module carries no source
		fillModuleV2(module, Origin{PackageVersion: "dev-tag", Dev: true}, nil)

		assert.Equal(t, "example", module.Spec.PackageRepositoryName, "the repository must not be blanked")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.False(t, module.IsDev(), "a half-filled origin sets no annotations")
	})

	t.Run("config source overrides the origin repository", func(t *testing.T) {
		module := &v1alpha2.Module{}

		conf := testModuleConfig("echo")
		conf.Spec.Source = "chosen"

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, conf)

		assert.Equal(t, "chosen", module.Spec.PackageRepositoryName)
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion, "the version still comes from the origin")
	})

	t.Run("config source goes through the repository mapping", func(t *testing.T) {
		module := &v1alpha2.Module{}

		conf := testModuleConfig("echo")
		conf.Spec.Source = "deckhouse"

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, conf)

		assert.Equal(t, "deckhouse-modules", module.Spec.PackageRepositoryName)
	})

	t.Run("config without a source keeps the origin repository", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, testModuleConfig("echo"))

		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
	})

	t.Run("embedded module ignores the config source", func(t *testing.T) {
		module := &v1alpha2.Module{}

		conf := testModuleConfig("echo")
		conf.Spec.Source = "chosen"

		fillModuleV2(module, Origin{RepositoryName: repositoryNameEmbedded, PackageVersion: "v1.77.0", Embedded: true}, conf)

		assert.Equal(t, repositoryNameEmbedded, module.Spec.PackageRepositoryName)
	})

	t.Run("config mirror alone moves the repository", func(t *testing.T) {
		module := &v1alpha2.Module{
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		conf := testModuleConfig("echo")
		conf.Spec.Source = "chosen"

		fillModuleV2(module, Origin{}, conf)

		assert.Equal(t, "chosen", module.Spec.PackageRepositoryName, "a config event alone repoints the module")
	})
}
