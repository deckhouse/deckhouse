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
		fake.NewImageBuilder().WithFile(bundle.DigestsPath, nestedDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Deckhouse().Digests(t.Context(), "v1.73.0")
	require.NoError(t, err)

	assert.Equal(t, bundle.DigestsPath, got.Source)
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
		fake.NewImageBuilder().WithFile(bundle.DigestsPath, flatDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Modules().Module("stronghold").Digests(t.Context(), "v1.0.1")
	require.NoError(t, err)

	assert.Equal(t, bundle.DigestsPath, got.Source)
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
		fake.NewImageBuilder().WithFile(bundle.DigestsPath, flatDigests).MustBuild())

	got, err := newFakeRegistry(t, reg).Packages().Package("elma").Digests(t.Context(), "v1.0.1")
	require.NoError(t, err)

	assert.Equal(t, bundle.DigestsPath, got.Source)
	assert.Equal(t, 2, got.Count())
}

// TestDigestsInstallerImages covers all three installer bundles.
func TestDigestsInstallerImages(t *testing.T) {
	reg := fake.NewRegistry("registry.deckhouse.io")
	img := fake.NewImageBuilder().WithFile(bundle.DigestsPath, flatDigests).MustBuild()
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

			assert.Equal(t, bundle.DigestsPath, got.Source)
			assert.False(t, got.IsNested())
			assert.Equal(t, 2, got.Count())
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
