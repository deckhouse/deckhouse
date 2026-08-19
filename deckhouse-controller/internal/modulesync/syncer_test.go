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

package modulesync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/testclient"
)

func newTestSyncer(t *testing.T, embeddedDir string, objects ...client.Object) *Syncer {
	t.Helper()

	cli, err := testclient.New(log.NewNop(), objects)
	require.NoError(t, err)

	return New(cli, cli, dependency.NewMockedContainer().GetClock(), log.NewNop(), WithEmbeddedModulesDir(embeddedDir))
}

// emptyEmbeddedDir returns a directory with no modules in it.
func emptyEmbeddedDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

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

func testV1Module(name, source string) *v1alpha1.Module {
	return &v1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Properties: v1alpha1.ModuleProperties{Source: source},
	}
}

func testDeployedRelease(module, sourceName, version string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: module + "-v" + version,
			Labels: map[string]string{
				"module":                          module,
				"source":                          sourceName,
				v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
			},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: module, Version: version, Weight: 900},
		Status: v1alpha1.ModuleReleaseStatus{Phase: v1alpha1.ModuleReleasePhaseDeployed},
	}
}

func getV2Module(t *testing.T, s *Syncer, name string) *v1alpha2.Module {
	t.Helper()

	module := new(v1alpha2.Module)
	require.NoError(t, s.reader.Get(context.Background(), client.ObjectKey{Name: name}, module))

	return module
}

func TestSyncOriginPrecedence(t *testing.T) {
	t.Run("ready pull override beats deployed release", func(t *testing.T) {
		s := newTestSyncer(t, emptyEmbeddedDir(t),
			testV1Module("echo", "example"),
			testReadyPullOverride("echo", "dev-tag"),
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
		assert.True(t, module.IsDev(), "override-owned module must carry the dev annotation")
		assert.False(t, module.IsEmbedded())
	})

	t.Run("embedded module beats a deployed release", func(t *testing.T) {
		embeddedDir := emptyEmbeddedDir(t)
		writeEmbeddedModule(t, embeddedDir, "380-echo", "echo")

		s := newTestSyncer(t, embeddedDir,
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, embeddedRepositoryName, module.Spec.PackageRepositoryName)
		assert.Equal(t, app.Version, module.Spec.PackageVersion)
		assert.True(t, module.IsEmbedded())
	})

	t.Run("newest deployed release wins and supersedes the older one", func(t *testing.T) {
		s := newTestSyncer(t, emptyEmbeddedDir(t),
			testDeployedRelease("echo", "example", "1.0.0"),
			testDeployedRelease("echo", "example", "1.1.0"),
		)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "v1.1.0", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)

		older := new(v1alpha1.ModuleRelease)
		require.NoError(t, s.reader.Get(context.Background(), client.ObjectKey{Name: "echo-v1.0.0"}, older))
		assert.Equal(t, v1alpha1.ModuleReleasePhaseSuperseded, older.Status.Phase)
	})
}

func TestSyncModules(t *testing.T) {
	enabled := true

	t.Run("creates a module with a known origin and config fields", func(t *testing.T) {
		conf := &v1alpha1.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec: v1alpha1.ModuleConfigSpec{
				Enabled:     &enabled,
				Version:     2,
				Settings:    v1alpha1.MakeMappedFields(map[string]any{"logLevel": "Debug"}),
				Maintenance: "NoResourceReconciliation",
			},
		}

		s := newTestSyncer(t, emptyEmbeddedDir(t),
			testDeployedRelease("echo", "example", "1.0.0"),
			conf,
		)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(t, 2, module.Spec.SettingsVersion)
		assert.Equal(t, "NoResourceReconciliation", module.Spec.Maintenance)
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled)
		assert.Equal(t, map[string]any{"logLevel": "Debug"}, module.Spec.Settings.GetMap())
	})

	t.Run("patches an existing module to the origin version", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v0.9.0"},
		}

		s := newTestSyncer(t, emptyEmbeddedDir(t),
			existing,
			testDeployedRelease("echo", "example", "1.0.0"),
		)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
	})

	t.Run("deletes a module no source claims and nothing backs", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := new(v1alpha2.Module)
		err = s.reader.Get(context.Background(), client.ObjectKey{Name: "echo"}, module)
		assert.True(t, client.IgnoreNotFound(err) == nil && err != nil, "a module no source claims and nothing backs must be deleted")
	})

	t.Run("keeps a module no source claims but a repository backs", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing)

		_, err := s.Sync(context.Background())
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
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

		fillModuleV2(module, Origin{RepositoryName: embeddedRepositoryName, PackageVersion: "v1.77.0", Embedded: true}, nil)
		assert.True(t, module.IsEmbedded())

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, nil)
		assert.False(t, module.IsEmbedded(), "a module no longer shipped in the image loses the annotation")
	})

	t.Run("dev annotation is only ever set", func(t *testing.T) {
		module := &v1alpha2.Module{}

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "dev-tag", Dev: true}, nil)
		assert.True(t, module.IsDev())

		fillModuleV2(module, Origin{RepositoryName: "example", PackageVersion: "v1.0.0"}, nil)
		assert.True(t, module.IsDev(), "the dev annotation is never cleared by the sync")
	})
}
