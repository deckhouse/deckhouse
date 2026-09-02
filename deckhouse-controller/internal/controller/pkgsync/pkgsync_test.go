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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestSyncer(t *testing.T, version, embeddedDir string, objects ...client.Object) (*syncer, client.Client) {
	t.Helper()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithStatusSubresource(&v1alpha1.ModulePackageVersion{}, &v1alpha1.ModulePackage{}, &v1alpha1.ModuleRelease{}, &v1alpha2.Module{}).
		WithObjects(objects...).
		Build()

	return newSyncer(cl, cl, dependency.NewMockedContainer(), version, embeddedDir, log.NewNop()), cl
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

// testRelease builds a release the way the release controller leaves it: the phase is
// mirrored into the status label the placement pass selects on.
func testRelease(module, source, version, phase string) *v1alpha1.ModuleRelease {
	return &v1alpha1.ModuleRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: module + "-v" + version,
			Labels: map[string]string{
				v1alpha1.ModuleReleaseLabelSource: source,
				v1alpha1.ModuleReleaseLabelModule: module,
				v1alpha1.ModuleReleaseLabelStatus: strings.ToLower(phase),
			},
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

func TestSourceNameForRepository(t *testing.T) {
	cases := []struct {
		repository string
		want       string
	}{
		{repository: "deckhouse-modules", want: "deckhouse"},
		{repository: "embedded", want: ""},
		{repository: "example", want: "example"},
		{repository: "", want: ""},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, SourceNameForRepository(c.repository), c.repository)
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
	require.Len(t, versions, 3)
	repositoryRV := getRepository(t, cl, "external").ResourceVersion

	modules := make(map[string]string)
	for _, name := range listModuleNames(t, cl) {
		modules[name] = getModule(t, cl, name).ResourceVersion
	}
	require.Len(t, modules, 2)

	require.NoError(t, s.sync(ctx))

	assert.Len(t, listVersionNames(t, cl), 3)
	for name, rv := range versions {
		assert.Equal(t, rv, getVersion(t, cl, name).ResourceVersion, name)
	}
	assert.Equal(t, repositoryRV, getRepository(t, cl, "external").ResourceVersion)

	assert.Len(t, listModuleNames(t, cl), 2)
	for name, rv := range modules {
		assert.Equal(t, rv, getModule(t, cl, name).ResourceVersion, name)
	}
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
