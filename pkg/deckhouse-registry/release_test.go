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

package dhregistry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
)

// deckhouseVersionJSON is what a Deckhouse release image declares: the rollout
// fields are populated, and there is no definition file alongside it.
const deckhouseVersionJSON = `{
  "version": "v1.73.0",
  "suspend": false,
  "requirements": {"k8s": ">= 1.27", "deckhouse": ">= 1.70"},
  "disruptions": {"1.73": ["ingressNginx"]},
  "canary": {"stable": {"enabled": true, "waves": 5, "interval": "15m"}}
}`

const changelogYAML = `
ingress-nginx:
  features:
    - summary: something changed
`

// newFakeRegistry wires a fake registry into the service tree. The fake client
// reports only the host from GetRegistry, so these tests exercise content
// reads; path construction is covered against the real client in registry_test.
func newFakeRegistry(t *testing.T, reg *fake.Registry) *dhregistry.Registry {
	t.Helper()

	return dhregistry.New(
		fake.NewClient(reg).WithSegment("deckhouse"),
		dhregistry.WithEdition(dhregistry.FEEdition),
	)
}

// TestDeckhouseRelease covers the Deckhouse release image, whose version.json
// carries the rollout fields.
func TestDeckhouseRelease(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/release-channel", "stable",
		fake.NewImageBuilder().
			WithFile(release.VersionFile, deckhouseVersionJSON).
			WithFile(release.ChangelogFile, changelogYAML).
			MustBuild())

	releases := newFakeRegistry(t, reg).Deckhouse().Releases()

	meta, err := releases.Metadata(t.Context(), "stable")
	require.NoError(t, err)

	assert.Equal(t, "v1.73.0", meta.Version)
	assert.False(t, meta.Suspend)
	assert.Equal(t, map[string]string{"k8s": ">= 1.27", "deckhouse": ">= 1.70"}, meta.Requirements)
	assert.Equal(t, map[string][]string{"1.73": {"ingressNginx"}}, meta.Disruptions)

	require.Contains(t, meta.Canary, "stable")
	assert.True(t, meta.Canary["stable"].Enabled)
	assert.Equal(t, uint(5), meta.Canary["stable"].Waves)
	assert.Equal(t, 15*time.Minute, meta.Canary["stable"].Interval)

	// Raw is the escape hatch for consumers applying their own schema.
	assert.JSONEq(t, deckhouseVersionJSON, string(meta.Raw))

	changelog, err := releases.Changelog(t.Context(), "stable")
	require.NoError(t, err)
	assert.Contains(t, changelog, "ingress-nginx")
}

// TestModuleRelease covers the module release image, which ships module.yaml
// and leaves the rollout fields empty.
func TestModuleRelease(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
		fake.NewImageBuilder().
			WithFile(release.VersionFile, `{"version": "v1.0.1"}`).
			WithFile(definition.ModuleFile, "name: stronghold\nweight: 910\n").
			MustBuild())

	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	version, err := releases.Version(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", version)

	// A module release's version.json declares only the version — the rollout
	// controls of a Deckhouse release have no counterpart here, which is why
	// the two map to different types.
	meta, err := releases.Metadata(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", meta.Version)
	assert.JSONEq(t, `{"version": "v1.0.1"}`, string(meta.Raw))

	def, err := releases.Definition(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, "stronghold", def.Name)
	assert.Equal(t, uint32(910), def.Weight)
}

// TestPackageRelease covers the package release image, the v2 counterpart:
// same role, package.yaml instead of module.yaml.
func TestPackageRelease(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/packages/elma/version", "v1.0.1",
		fake.NewImageBuilder().
			WithFile(release.VersionFile, `{"version": "v1.0.1"}`).
			WithFile(definition.PackageFile, "name: elma\n").
			MustBuild())

	versions := newFakeRegistry(t, reg).Packages().Package("elma").Versions()

	version, err := versions.Version(t.Context(), "v1.0.1")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", version)

	def, err := versions.Definition(t.Context(), "v1.0.1")
	require.NoError(t, err)
	assert.Equal(t, "elma", def.Name)
}

// TestReleaseDefinitionAbsent covers a release image that declares a version
// but ships no definition — older module releases, where the definition has to
// be read from the module image instead.
func TestReleaseDefinitionAbsent(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
		fake.NewImageBuilder().WithFile(release.VersionFile, `{"version": "v1.0.1"}`).MustBuild())

	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	_, err := releases.Definition(t.Context(), "alpha")
	require.ErrorIs(t, err, dhregistry.ErrFileNotFound)

	// The version is still readable.
	version, err := releases.Version(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", version)
}

// TestReleaseFiles covers reading several metadata files in a single image
// pull, which is what callers needing more than one should use.
func TestReleaseFiles(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
		fake.NewImageBuilder().
			WithFile(release.VersionFile, `{"version": "v1.0.1"}`).
			WithFile(definition.ModuleFile, "name: stronghold\n").
			MustBuild())

	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	files, err := releases.Files(t.Context(), "alpha", release.VersionFile, definition.ModuleFile, release.ChangelogFile)
	require.NoError(t, err)

	// Files that are present come back; the absent changelog simply is not
	// there, rather than failing the whole read.
	assert.Len(t, files, 2)
	assert.Contains(t, files, release.VersionFile)
	assert.Contains(t, files, definition.ModuleFile)
	assert.NotContains(t, files, release.ChangelogFile)
}

func TestReleaseNoVersionMetadata(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
		fake.NewImageBuilder().WithFile("unrelated.txt", "x").MustBuild())

	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	_, err := releases.Version(t.Context(), "alpha")
	require.ErrorIs(t, err, dhregistry.ErrNoVersionMetadata)
}

func TestReleaseSuspend(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/release-channel", "alpha",
		fake.NewImageBuilder().WithFile(release.VersionFile, `{"version":"v1.74.0","suspend":true}`).MustBuild())

	meta, err := newFakeRegistry(t, reg).Deckhouse().Releases().Metadata(t.Context(), "alpha")
	require.NoError(t, err)
	assert.True(t, meta.Suspend)
}

func TestReleaseNotFound(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/release-channel", "stable",
		fake.NewImageBuilder().WithFile(release.VersionFile, `{"version":"v1.73.0"}`).MustBuild())

	_, err := newFakeRegistry(t, reg).Deckhouse().Releases().Metadata(t.Context(), "rock-solid")
	require.Error(t, err)
	assert.True(t, dhregistry.IsNotFound(err), "expected a not-found error, got %v", err)
}

func TestChannels(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	img := fake.NewImageBuilder().WithFile(release.VersionFile, `{"version":"v1.73.0"}`).MustBuild()

	for _, tag := range []string{"alpha", "stable", "lts", "v1.73.0", "v1.72.0"} {
		reg.MustAddImage("deckhouse/fe/release-channel", tag, img)
	}

	channels, err := newFakeRegistry(t, reg).Deckhouse().Releases().Channels(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"alpha", "stable", "lts"}, channels)
}

func TestExists(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules", "stronghold", fake.NewImageBuilder().MustBuild())

	modules := newFakeRegistry(t, reg).Modules()

	found, err := modules.Exists(t.Context(), "stronghold")
	require.NoError(t, err)
	assert.True(t, found)

	found, err = modules.Exists(t.Context(), "absent")
	require.NoError(t, err)
	assert.False(t, found)
}

// TestModulesList covers enumerating the module catalog, whose tags are the
// module names the edition publishes.
func TestModulesList(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	scratch := fake.NewImageBuilder().MustBuild()
	reg.MustAddImage("deckhouse/fe/modules", "stronghold", scratch)
	reg.MustAddImage("deckhouse/fe/modules", "neuvector", scratch)

	modules, err := newFakeRegistry(t, reg).Modules().List(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"stronghold", "neuvector"}, modules)
}

func TestExtraImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/neuvector/extra/scanner", "3",
		fake.NewImageBuilder().WithLabel("component", "scanner").MustBuild())

	cfg, err := newFakeRegistry(t, reg).
		Modules().Module("neuvector").Extra().Image("scanner").
		GetImageConfig(t.Context(), "3")
	require.NoError(t, err)
	assert.Equal(t, "scanner", cfg.Config.Labels["component"])
}

func TestDurationUnmarshal(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/release-channel", "stable",
		fake.NewImageBuilder().
			WithFile(release.VersionFile, `{"version":"v1.73.0","canary":{"stable":{"interval":900000000000}}}`).
			MustBuild())

	meta, err := newFakeRegistry(t, reg).Deckhouse().Releases().Metadata(t.Context(), "stable")
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, meta.Canary["stable"].Interval)
}
