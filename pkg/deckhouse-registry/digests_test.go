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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
)

// flatDigests is the shape a module, package or installer image ships.
const flatDigests = `{
  "controller": "sha256:aaa",
  "webhook": "sha256:bbb"
}`

// nestedDigests is the shape the Deckhouse image ships: one file covering every
// module it bundles, keyed by CamelCase module name.
const nestedDigests = `{
  "ingressNginx": {"controller": "sha256:111", "kruise": "sha256:222"},
  "userAuthn": {"dexAuthenticator": "sha256:333"}
}`

// TestDigestsDeckhouseImage covers the Deckhouse image, which bundles every
// module of its edition and so keys its digests by module.
func TestDigestsDeckhouseImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe", "v1.73.0",
		fake.NewImageBuilder().WithFile(bundle.ModulesImagesDigestsPath, nestedDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Deckhouse().Digests(t.Context(), "v1.73.0")
	require.NoError(t, err)

	assert.Equal(t, bundle.ModulesImagesDigestsPath, got.Source)
	assert.True(t, got.IsNested())
	assert.ElementsMatch(t, []string{"ingressNginx", "userAuthn"}, got.Modules())
	assert.Equal(t, 3, got.Count())

	digest, ok := got.Lookup("ingressNginx", "kruise")
	assert.True(t, ok)
	assert.Equal(t, "sha256:222", digest)

	_, ok = got.Lookup("userAuthn", "absent")
	assert.False(t, ok)
}

// TestDigestsModuleImage covers a module image, which bundles only its own
// images and so keeps a flat image-to-digest map.
func TestDigestsModuleImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, flatDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Modules().Module("stronghold").Digests(t.Context(), "v1.0.1")
	require.NoError(t, err)

	assert.Equal(t, bundle.RootPath, got.Source)
	assert.False(t, got.IsNested())
	assert.Nil(t, got.Modules())
	assert.Equal(t, map[string]string{"controller": "sha256:aaa", "webhook": "sha256:bbb"}, got.Images)

	digest, ok := got.Lookup("", "controller")
	assert.True(t, ok)
	assert.Equal(t, "sha256:aaa", digest)
}

// TestDigestsPackageImage covers the v2 package bundle, which shares the module
// layout.
func TestDigestsPackageImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/packages/elma", "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, flatDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Packages().Package("elma").Digests(t.Context(), "v1.0.1")
	require.NoError(t, err)

	assert.Equal(t, bundle.RootPath, got.Source)
	assert.Equal(t, 2, got.Count())
}

// TestDigestsInstallerImages covers all three installer bundles. An installer
// carries the images of many modules, so its file is nested like the Deckhouse
// image's — the shape follows what is bundled, not the kind of bundle.
func TestDigestsInstallerImages(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	img := fake.NewImageBuilder().WithFile(bundle.CandiImagesDigestsPath, nestedDigests).MustBuild()
	reg.MustAddImage("deckhouse/fe/install", "v1.73.0", img)
	reg.MustAddImage("deckhouse/fe/install-standalone", "v1.73.0", img)
	reg.MustAddImage("deckhouse/installer", "v1.73.0", img)

	dhreg := newFakeRegistry(t, reg)

	for name, svc := range map[string]*bundle.Service{
		"install":            dhreg.Deckhouse().Install(),
		"install-standalone": dhreg.Deckhouse().InstallStandalone(),
		"installer":          dhreg.Installer(),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := svc.Digests(t.Context(), "v1.73.0")
			require.NoError(t, err)

			assert.Equal(t, bundle.CandiImagesDigestsPath, got.Source)
			assert.True(t, got.IsNested())
			assert.Equal(t, 3, got.Count())
		})
	}
}

// TestDigestsMissingFile covers a bundle image that does not carry the file.
func TestDigestsMissingFile(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1",
		fake.NewImageBuilder().WithFile("module.yaml", "name: stronghold\n").MustBuild())

	_, err := newFakeRegistry(t, reg).Modules().Module("stronghold").Digests(t.Context(), "v1.0.1")
	require.ErrorIs(t, err, dhregistry.ErrNoDigests)
}

func TestDigestsMissingImage(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1", fake.NewImageBuilder().MustBuild())

	_, err := newFakeRegistry(t, reg).Modules().Module("stronghold").Digests(t.Context(), "v9.9.9")
	require.Error(t, err)
	assert.True(t, dhregistry.IsNotFound(err), "expected a not-found error, got %v", err)
}

// TestDigestsPathsPerBundle pins where each bundle keeps its file. The
// locations differ, and reading the wrong one must miss rather than silently
// succeed — a module's root file cannot satisfy a Deckhouse-image read.
func TestDigestsPathsPerBundle(t *testing.T) {
	assert.Equal(t, "images_digests.json", bundle.RootPath)
	assert.Equal(t, "deckhouse/modules/images_digests.json", bundle.ModulesImagesDigestsPath)
	assert.Equal(t, "deckhouse/candi/images_digests.json", bundle.CandiImagesDigestsPath)

	reg := newFE(t)
	assert.Equal(t, bundle.ModulesImagesDigestsPath, reg.Deckhouse().DigestsPath())
	assert.Equal(t, bundle.CandiImagesDigestsPath, reg.Deckhouse().Install().DigestsPath())
	assert.Equal(t, bundle.CandiImagesDigestsPath, reg.Deckhouse().InstallStandalone().DigestsPath())
	assert.Equal(t, bundle.CandiImagesDigestsPath, reg.Installer().DigestsPath())
	assert.Equal(t, bundle.RootPath, reg.Modules().Module("stronghold").DigestsPath())
	assert.Equal(t, bundle.RootPath, reg.Packages().Package("elma").DigestsPath())
}

// TestDigestsWrongPathMisses covers a Deckhouse image that only carries a
// root-level file: the read must not fall back to it.
func TestDigestsWrongPathMisses(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe", "v1.73.0",
		fake.NewImageBuilder().WithFile(bundle.RootPath, flatDigests).MustBuild())

	_, err := newFakeRegistry(t, reg).Deckhouse().Digests(t.Context(), "v1.73.0")
	require.ErrorIs(t, err, dhregistry.ErrNoDigests)
}
