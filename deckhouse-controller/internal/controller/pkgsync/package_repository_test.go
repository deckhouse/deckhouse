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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestSyncPackageRepositories(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a repository from a module source", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModuleSource("external", "registry.example.io/external"),
		)

		require.NoError(t, s.sync(ctx))

		repo := getRepository(t, cl, "external")
		assert.Equal(t, "deckhouse", repo.Labels["heritage"])
		assert.Equal(t, "HTTPS", repo.Spec.Registry.Scheme)
		assert.Equal(t, "registry.example.io/external", repo.Spec.Registry.Repo)
		assert.Equal(t, "ZG9ja2VyY2Zn", repo.Spec.Registry.DockerCFG)
		assert.Equal(t, "test-ca", repo.Spec.Registry.CA)
		assert.Empty(t, repo.Spec.Registry.Login, "a module source carries no login")
		assert.Empty(t, repo.Spec.Registry.Password, "a module source carries no password")
		assert.Nil(t, repo.Spec.ScanInterval, "the package-repository controller fills the default itself")
	})

	t.Run("skips the excluded module sources", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModuleSource("deckhouse", "registry.deckhouse.io/deckhouse/ce/modules"),
			testModuleSource("flant", "registry.flant.com/modules"),
		)

		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listRepositoryNames(t, cl))
	})

	t.Run("skips a source being deleted", func(t *testing.T) {
		doomed := testModuleSource("doomed", "registry.example.io/doomed")
		now := metav1.Now()
		doomed.DeletionTimestamp = &now
		doomed.Finalizers = []string{v1alpha1.ModuleSourceFinalizerReleaseExists}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), doomed)

		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listRepositoryNames(t, cl))
	})

	t.Run("refreshes the registry of an existing repository from the source", func(t *testing.T) {
		interval := metav1.Duration{Duration: 30 * time.Minute}
		existing := &v1alpha1.PackageRepository{
			ObjectMeta: metav1.ObjectMeta{Name: "external"},
			Spec: v1alpha1.PackageRepositorySpec{
				ScanInterval: &interval,
				Registry: v1alpha1.PackageRepositorySpecRegistry{
					Scheme:    "http",
					Repo:      "mirror.example.io/external",
					DockerCFG: "b2xk",
					Login:     "admin",
					Password:  "secret",
				},
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), existing,
			testModuleSource("external", "registry.example.io/external"),
		)

		require.NoError(t, s.sync(ctx))

		after := getRepository(t, cl, existing.Name)
		assert.Equal(t, "registry.example.io/external", after.Spec.Registry.Repo, "the registry follows the source")
		assert.Equal(t, "HTTPS", after.Spec.Registry.Scheme)
		assert.Equal(t, "ZG9ja2VyY2Zn", after.Spec.Registry.DockerCFG)
		assert.Equal(t, "test-ca", after.Spec.Registry.CA)
		assert.Equal(t, "admin", after.Spec.Registry.Login, "the fields the source does not carry survive")
		assert.Equal(t, "secret", after.Spec.Registry.Password)
		require.NotNil(t, after.Spec.ScanInterval)
		assert.Equal(t, 30*time.Minute, after.Spec.ScanInterval.Duration, "the scan interval survives")
	})

	t.Run("keeps a repository matching the source untouched", func(t *testing.T) {
		existing := &v1alpha1.PackageRepository{
			ObjectMeta: metav1.ObjectMeta{Name: "external"},
			Spec: v1alpha1.PackageRepositorySpec{
				Registry: v1alpha1.PackageRepositorySpecRegistry{
					Scheme:    "HTTPS",
					Repo:      "registry.example.io/external",
					DockerCFG: "ZG9ja2VyY2Zn",
					CA:        "test-ca",
					Login:     "admin",
				},
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), existing,
			testModuleSource("external", "registry.example.io/external"),
		)
		before := getRepository(t, cl, existing.Name)

		require.NoError(t, s.sync(ctx))

		after := getRepository(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion, "a matching repository is not rewritten")
	})
}
