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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newEnsureClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	_, cl := newTestSyncer(t, testVersion, t.TempDir(), objects...)

	return cl
}

func TestEnsureModule(t *testing.T) {
	t.Run("creates a missing module and seeds the config fields", func(t *testing.T) {
		cl := newEnsureClient(t, testModuleConfig("echo"))

		err := EnsureModule(context.Background(), cl, cl, "echo", OriginFromDeployedRelease("example", "v1.0.0"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.0.0", module.Spec.PackageVersion)
		assert.Equal(t, "example", module.Spec.PackageRepositoryName)
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled)
	})

	t.Run("seeds the config fields when the version arrives on an empty module", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		cl := newEnsureClient(t, existing, testModuleConfig("echo"))

		err := EnsureModule(context.Background(), cl, cl, "echo", OriginFromPullOverride("example", "dev-tag"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "dev-tag", module.Spec.PackageVersion)
		assert.True(t, module.IsDev())
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy, "seeding must carry the config fields")
	})

	t.Run("updates only the origin fields on a module that already has a version", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0", UpdatePolicy: "manual"},
		}

		cl := newEnsureClient(t, existing, testModuleConfig("echo"))

		err := EnsureModule(context.Background(), cl, cl, "echo", OriginFromDeployedRelease("example", "v1.1.0"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "v1.1.0", module.Spec.PackageVersion)
		assert.Equal(t, "manual", module.Spec.UpdatePolicy, "config fields belong to the config mirror after seeding")
	})
}

func TestOriginConstructorsMapSourceNames(t *testing.T) {
	assert.Equal(t, "deckhouse-modules", OriginFromDeployedRelease("deckhouse", "v1.4.3").RepositoryName)
	assert.Equal(t, "deckhouse-modules", OriginFromPullOverride("deckhouse", "pr1234").RepositoryName)
	assert.Equal(t, "example", OriginFromDeployedRelease("example", "v1.0.0").RepositoryName)
}

func TestEnsureModuleConfig(t *testing.T) {
	t.Run("mirrors the config fields onto a versioned module", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "example", PackageVersion: "v1.0.0"},
		}

		cl := newEnsureClient(t, existing)

		err := EnsureModuleConfig(context.Background(), cl, cl, testModuleConfig("echo"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
		assert.Equal(t, map[string]any{"logLevel": "Debug"}, module.Spec.Settings.GetMap())
	})

	t.Run("keeps the embedded annotation of an embedded module", func(t *testing.T) {
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "echo",
				Annotations: map[string]string{v1alpha2.ModuleAnnotationEmbedded: "true"},
			},
			Spec: v1alpha2.ModuleSpec{PackageRepositoryName: repositoryNameEmbedded, PackageVersion: "v1.77.0"},
		}

		cl := newEnsureClient(t, existing)

		err := EnsureModuleConfig(context.Background(), cl, cl, testModuleConfig("echo"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.True(t, module.IsEmbedded(), "the config mirror must not strip the embedded annotation")
		assert.Equal(t, "test-alpha", module.Spec.UpdatePolicy)
	})

	t.Run("skips a module that has no package version yet", func(t *testing.T) {
		existing := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "echo"}}

		cl := newEnsureClient(t, existing)

		err := EnsureModuleConfig(context.Background(), cl, cl, testModuleConfig("echo"), log.NewNop())
		require.NoError(t, err)

		module := getV2Module(t, cl, "echo")
		assert.Nil(t, module.Spec.Settings, "config fields must not materialize a spec without a version")
	})

	t.Run("skips a config whose module does not exist", func(t *testing.T) {
		cl := newEnsureClient(t)

		err := EnsureModuleConfig(context.Background(), cl, cl, testModuleConfig("ghost"), log.NewNop())
		require.NoError(t, err)
	})
}
