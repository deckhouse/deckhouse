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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func getModulePackage(t *testing.T, cl client.Client, name string) *v1alpha1.ModulePackage {
	t.Helper()

	pkg := new(v1alpha1.ModulePackage)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, pkg))

	return pkg
}

func TestSyncModulePackages(t *testing.T) {
	ctx := context.Background()

	sourceWithAvailableModules := func(sourceName string, moduleNames ...string) *v1alpha1.ModuleSource {
		moduleSource := testModuleSource(sourceName, "registry.example.com/modules")
		for _, moduleName := range moduleNames {
			moduleSource.Status.AvailableModules = append(moduleSource.Status.AvailableModules,
				v1alpha1.AvailableModule{Name: moduleName})
		}

		return moduleSource
	}

	echoOnDisk := func(t *testing.T) string {
		t.Helper()

		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		return dir
	}

	t.Run("creates an empty package for an embedded module no source lists", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t))
		require.NoError(t, s.sync(ctx))

		pkg := getModulePackage(t, cl, "echo")
		assert.Equal(t, map[string]string{"heritage": "deckhouse"}, pkg.Labels)
		assert.Empty(t, pkg.OwnerReferences, "no owner: an embedded package is available in no repository")
		assert.Empty(t, pkg.Status.AvailableRepositories)
	})

	t.Run("seeds the repositories of the sources listing the module", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t),
			sourceWithAvailableModules("example", "echo"),
			sourceWithAvailableModules("other", "parca"))
		require.NoError(t, s.sync(ctx))

		assert.Equal(t, []string{"example"}, getModulePackage(t, cl, "echo").Status.AvailableRepositories)
	})

	t.Run("several sources listing the module seed one entry each", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t),
			sourceWithAvailableModules("example", "echo"),
			sourceWithAvailableModules("deckhouse", "echo"))
		require.NoError(t, s.sync(ctx))

		assert.ElementsMatch(t, []string{"example", "deckhouse-modules"},
			getModulePackage(t, cl, "echo").Status.AvailableRepositories,
			"the deckhouse source serves the deckhouse-modules repository the platform ships itself")
	})

	t.Run("the global module is seeded like any other", func(t *testing.T) {
		globalDir := t.TempDir()
		writeLegacyOpenAPI(t, globalDir, "type: object\n", "type: object\n")

		s, cl := newTestSyncerWithGlobal(t, "v1.80.0", t.TempDir(), globalDir, sourceWithAvailableModules("example", "global"))
		require.NoError(t, s.sync(ctx))

		assert.Equal(t, []string{"example"}, getModulePackage(t, cl, "global").Status.AvailableRepositories,
			"the reserved name is seeded like any other when a source does name it")
	})

	t.Run("leaves an existing package untouched", func(t *testing.T) {
		existing := &v1alpha1.ModulePackage{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "echo",
				Labels: map[string]string{"user": "label"},
			},
			Status: v1alpha1.ModulePackageStatus{
				AvailableRepositories: []string{"deckhouse-modules"},
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", echoOnDisk(t), existing, sourceWithAvailableModules("example", "echo"))
		require.NoError(t, s.sync(ctx))

		pkg := getModulePackage(t, cl, "echo")
		assert.Equal(t, map[string]string{"user": "label"}, pkg.Labels)
		assert.Equal(t, []string{"deckhouse-modules"}, pkg.Status.AvailableRepositories,
			"an entry the scan already owns is never reseeded")
	})

	t.Run("a release stub creates no package", func(t *testing.T) {
		s, cl := newTestSyncer(t, "v1.80.0", t.TempDir(),
			testModuleRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		)

		require.NoError(t, s.sync(ctx))

		assert.Empty(t, listModulePackageNamesExceptGlobal(t, cl), "the repository scan builds the catalog of sourced packages")
	})
}
