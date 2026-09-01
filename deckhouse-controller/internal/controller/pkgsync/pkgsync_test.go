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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// newTestSyncer builds a syncer whose global hooks dir holds no schemas, so the global version
// it writes carries none either and only the embedded dir and the objects under test are asserted on.
func newTestSyncer(t *testing.T, version, embeddedDir string, objects ...client.Object) (*syncer, client.Client) {
	t.Helper()

	return newTestSyncerWithGlobal(t, version, embeddedDir, t.TempDir(), objects...)
}

// newTestSyncerWithGlobal builds a syncer over both dirs, for the tests that drive the global module.
func newTestSyncerWithGlobal(t *testing.T, version, embeddedDir, globalDir string, objects ...client.Object) (*syncer, client.Client) {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}, &v1alpha1.ModulePackage{}).
		WithObjects(objects...).
		Build()

	return newSyncer(cl, cl, dependency.NewMockedContainer(), version, embeddedDir, globalDir, log.NewNop()), cl
}

// writeLegacyOpenAPI writes the openapi files under the legacy config-values.yaml name the
// global hooks dir still uses.
func writeLegacyOpenAPI(t *testing.T, dir, settings, values string) {
	t.Helper()
	openAPIDir := filepath.Join(dir, "openapi")
	require.NoError(t, os.MkdirAll(openAPIDir, 0o755))
	if settings != "" {
		require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "config-values.yaml"), []byte(settings), 0o644))
	}
	if values != "" {
		require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte(values), 0o644))
	}
}

func writeModuleYAML(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "module.yaml"), []byte(content), 0o644))
}

func writePackageYAML(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"), []byte(content), 0o644))
}

func writeOpenAPI(t *testing.T, dir, settings, values string) {
	t.Helper()
	openAPIDir := filepath.Join(dir, "openapi")
	require.NoError(t, os.MkdirAll(openAPIDir, 0o755))
	if settings != "" {
		require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "settings.yaml"), []byte(settings), 0o644))
	}
	if values != "" {
		require.NoError(t, os.WriteFile(filepath.Join(openAPIDir, "values.yaml"), []byte(values), 0o644))
	}
}

func testModuleSource(name, repo string) *v1alpha1.ModuleSource {
	return &v1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.ModuleSourceSpec{
			Registry: v1alpha1.ModuleSourceSpecRegistry{
				Scheme:    "HTTPS",
				Repo:      repo,
				DockerCFG: "ZG9ja2VyY2Zn",
				CA:        "test-ca",
			},
		},
	}
}

func testRelease(module, source, version, phase string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:   module + "-v" + version,
			Labels: map[string]string{"source": source},
		},
		Spec:   v1alpha1.ModuleReleaseSpec{ModuleName: module, Version: version},
		Status: v1alpha1.ModuleReleaseStatus{Phase: phase},
	}
}

func listVersionNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	list := new(v1alpha1.ModulePackageVersionList)
	require.NoError(t, cl.List(context.Background(), list))

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}

	return names
}

func listPackageNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	list := new(v1alpha1.ModulePackageList)
	require.NoError(t, cl.List(context.Background(), list))

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}

	return names
}

// listModuleVersionNames lists every version except the global module's, whose name carries
// the running Deckhouse version and so is matched by prefix. Every sync writes that one, so a
// test about the embedded modules dir or the releases drops it to keep asserting only on its
// own producer; TestSyncGlobalVersion covers it directly.
func listModuleVersionNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	return slices.DeleteFunc(listVersionNames(t, cl), func(name string) bool {
		return strings.HasPrefix(name, repositoryNameEmbedded+"-"+packageNameGlobal+"-")
	})
}

// listModulePackageNames is listModuleVersionNames for the catalog entries.
func listModulePackageNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	return slices.DeleteFunc(listPackageNames(t, cl), func(name string) bool {
		return name == packageNameGlobal
	})
}

func getVersion(t *testing.T, cl client.Client, name string) *v1alpha1.ModulePackageVersion {
	t.Helper()

	mpv := new(v1alpha1.ModulePackageVersion)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, mpv))

	return mpv
}

func getRepository(t *testing.T, cl client.Client, name string) *v1alpha1.PackageRepository {
	t.Helper()

	repo := new(v1alpha1.PackageRepository)
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Name: name}, repo))

	return repo
}

func listRepositoryNames(t *testing.T, cl client.Client) []string {
	t.Helper()

	list := new(v1alpha1.PackageRepositoryList)
	require.NoError(t, cl.List(context.Background(), list))

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}

	return names
}

func TestRepositoryNameForSource(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{source: "deckhouse", want: "deckhouse-modules"},
		{source: "example", want: "example"},
		{source: "deckhouse-prod", want: "deckhouse-prod"},
		{source: "", want: ""},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, RepositoryNameForSource(c.source), c.source)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	writeModuleYAML(t, filepath.Join(dir, "900-echo"), "name: echo\nstage: General Availability\n")

	s, cl := newTestSyncer(t, "v1.80.0", dir,
		testModuleSource("external", "registry.example.io/external"),
		testRelease("parca", "deckhouse", "1.4.3", v1alpha1.ModuleReleasePhaseDeployed),
		testRelease("console", "deckhouse", "1.60.1", v1alpha1.ModuleReleasePhasePending),
	)

	require.NoError(t, s.sync(ctx))

	versions := make(map[string]string)
	for _, name := range listVersionNames(t, cl) {
		versions[name] = getVersion(t, cl, name).ResourceVersion
	}
	require.Len(t, versions, 4, "two releases, one embedded module and the global one")
	repositoryRV := getRepository(t, cl, "external").ResourceVersion

	require.NoError(t, s.sync(ctx))

	assert.Len(t, listVersionNames(t, cl), 4)
	for name, rv := range versions {
		assert.Equal(t, rv, getVersion(t, cl, name).ResourceVersion, name)
	}
	assert.Equal(t, repositoryRV, getRepository(t, cl, "external").ResourceVersion)
}
