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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	t.Run("an existing module keeps what the sync does not own", func(t *testing.T) {
		dir := t.TempDir()
		writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\n")

		enabled := true
		existing := &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "echo",
				Annotations: map[string]string{"en.meta.deckhouse.io/description": "echoes back"},
			},
			Spec: v1alpha2.ModuleSpec{
				PackageRepositoryName: "deckhouse-modules",
				PackageVersion:        "v1.2.3",
				Enabled:               &enabled,
			},
		}

		s, cl := newTestSyncer(t, "v1.80.0", dir, existing)
		require.NoError(t, s.sync(ctx))

		module := getModule(t, cl, "echo")
		assert.Equal(t, "embedded", module.Spec.PackageRepositoryName, "the embedded copy outranks the repository the module came from")
		assert.Equal(t, "v1.80.0", module.Spec.PackageVersion)
		assert.True(t, module.IsEmbedded())
		assert.Equal(t, "echoes back", module.Annotations["en.meta.deckhouse.io/description"])
		require.NotNil(t, module.Spec.Enabled)
		assert.True(t, *module.Spec.Enabled, "the module config owns this field, the sync does not")
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
