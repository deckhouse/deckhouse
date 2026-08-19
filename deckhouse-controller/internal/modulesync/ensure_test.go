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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

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

func TestEnsureModule(t *testing.T) {
	t.Run("creates a missing module and seeds the config fields", func(t *testing.T) {
		s := newTestSyncer(t, emptyEmbeddedDir(t), testModuleConfig("echo"))

		err := s.EnsureModule(context.Background(), "echo", OriginFromDeployedRelease("example", "v1.0.0"))
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled)
	})

	t.Run("seeds the config fields when the version arrives on an empty module", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing, testModuleConfig("echo"))

		err := s.EnsureModule(context.Background(), "echo", OriginFromPullOverride("example", "dev-tag"))
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
		assert.True(t, module.IsDev())
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy, "seeding must carry the config fields")
	})

	t.Run("updates only the origin fields on a module that already has a version", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0", UpdatePolicy: "manual"},
		}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing, testModuleConfig("echo"))

		err := s.EnsureModule(context.Background(), "echo", OriginFromDeployedRelease("example", "v1.1.0"))
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "v1.1.0", module.Spec.PackageVersion)
		assert.Equal(t, "manual", module.Spec.UpdatePolicy, "config fields belong to the config mirror after seeding")
	})
}

func TestEnsureModuleConfig(t *testing.T) {
	t.Run("mirrors the config fields onto a versioned module", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing)

		err := s.EnsureModuleConfig(context.Background(), testModuleConfig("echo"))
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
		assert.Equal(t, map[string]any{"logLevel": "Debug"}, module.Spec.Settings.GetMap())
	})

	t.Run("skips a module that has no package version yet", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		s := newTestSyncer(t, emptyEmbeddedDir(t), existing)

		err := s.EnsureModuleConfig(context.Background(), testModuleConfig("echo"))
		require.NoError(t, err)

		module := getV2Module(t, s, "echo")
		assert.Nil(t, module.Spec.Settings, "config fields must not materialize a spec without a version")
	})

	t.Run("skips a config whose module does not exist", func(t *testing.T) {
		s := newTestSyncer(t, emptyEmbeddedDir(t))

		err := s.EnsureModuleConfig(context.Background(), testModuleConfig("ghost"))
		require.NoError(t, err)
	})
}
