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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestSyncModulePackages(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a package for every module the sources offer", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModule("parca", "deckhouse", "deckhouse"),
			testModule("foo", "external", "deckhouse", "external"),
			testModule("echo", v1alpha1.ModuleSourceEmbedded),
			testModule("bare", ""),
		)

		require.NoError(t, s.Sync(ctx))

		list := new(v1alpha1.ModulePackageList)
		require.NoError(t, cl.List(ctx, list))
		require.Len(t, list.Items, 2, "embedded and sourceless modules name no package")

		parca := getPackage(t, cl, "parca")
		assert.Equal(t, []string{"deckhouse-modules"}, parca.Status.AvailableRepositories, "the source maps to the repository name")
		assert.Equal(t, "deckhouse", parca.Labels["heritage"])
		assert.Empty(t, parca.OwnerReferences, "no owner until a repository adopts the package")

		foo := getPackage(t, cl, "foo")
		assert.Equal(t, []string{"deckhouse-modules", "external"}, foo.Status.AvailableRepositories)
	})

	t.Run("keeps an existing package untouched", func(t *testing.T) {
		existing := &v1alpha1.ModulePackage{
			ObjectMeta: metav1.ObjectMeta{Name: "parca"},
			Status:     v1alpha1.ModulePackageStatus{AvailableRepositories: []string{"other-repo"}},
		}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), existing,
			testModule("parca", "deckhouse", "deckhouse"),
		)
		before := getPackage(t, cl, existing.Name)

		require.NoError(t, s.Sync(ctx))

		after := getPackage(t, cl, existing.Name)
		assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
		assert.Equal(t, []string{"other-repo"}, after.Status.AvailableRepositories)
	})

	t.Run("completes a package left without repositories", func(t *testing.T) {
		interrupted := &v1alpha1.ModulePackage{ObjectMeta: metav1.ObjectMeta{Name: "parca"}}

		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(), interrupted,
			testModule("parca", "deckhouse", "deckhouse"),
		)

		require.NoError(t, s.Sync(ctx))

		assert.Equal(t, []string{"deckhouse-modules"}, getPackage(t, cl, "parca").Status.AvailableRepositories,
			"a create interrupted before the status patch must be completed")
	})
}
