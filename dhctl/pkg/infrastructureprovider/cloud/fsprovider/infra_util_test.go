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

package fsprovider

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registryconfig "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestAnInstallationThatNamesNoRegistryStillFindsItsImages is the failure this exists for, and it
// was measured rather than imagined: installing a cluster whose only statement about a registry is
// the registry module's own ModuleConfig died fourteen seconds in with
//
//	Cannot download infrastructure util … unmarshaling dockerconfig JSON: unexpected end of JSON input
//
// The images were there the whole time — served by the bundle registry on the loopback address — and
// nothing had gone looking for them: the legacy dockercfg was empty, and decoding it came first.
func TestAnInstallationThatNamesNoRegistryStillFindsItsImages(t *testing.T) {
	conf := &config.MetaConfig{}
	conf.Registry.Settings.RemoteData = registryconfig.Data{
		ImagesRepo: constant.BundleImagesRepo,
		Scheme:     constant.BundleScheme,
	}

	regConfig, prefix, err := imageSource(conf)
	require.NoError(t, err, "an installation from a bundle names no registry and must not be asked for one")
	require.Equal(t, constant.BundleImagesRepo, regConfig.GetRegistry())
	require.Equal(t, string(constant.BundleScheme), regConfig.GetScheme())
	require.Equal(t, constant.BundleImagesRepo+"@", prefix)

	// Anonymous, because the bundle registry has nobody to authenticate as.
	require.Empty(t, regConfig.GetUsername())
	require.Empty(t, regConfig.GetPassword())
}

// TestTheLegacyFieldKeepsPrecedenceWhenItStatesARegistry: the fallback above must not take over an
// ordinary installation. Everything that names a registry in `InitConfiguration.deckhouse` keeps
// being installed from exactly that registry, with exactly those credentials.
func TestTheLegacyFieldKeepsPrecedenceWhenItStatesARegistry(t *testing.T) {
	dockerCfg := base64.StdEncoding.EncodeToString([]byte(
		`{"auths":{"r.example.com":{"auth":"` +
			base64.StdEncoding.EncodeToString([]byte("user:secret")) + `"}}}`))

	conf := &config.MetaConfig{}
	conf.DeckhouseConfig.ImagesRepo = "r.example.com/deckhouse/ee"
	conf.DeckhouseConfig.RegistryDockerCfg = dockerCfg
	conf.DeckhouseConfig.RegistryScheme = "https"
	// Deliberately different, to show which of the two was read.
	conf.Registry.Settings.RemoteData = registryconfig.Data{
		ImagesRepo: constant.BundleImagesRepo,
		Scheme:     constant.BundleScheme,
	}

	regConfig, prefix, err := imageSource(conf)
	require.NoError(t, err)
	require.Equal(t, "r.example.com/deckhouse/ee", regConfig.GetRegistry())
	require.Equal(t, "HTTPS", regConfig.GetScheme())
	require.Equal(t, "user", regConfig.GetUsername())
	require.Equal(t, "secret", regConfig.GetPassword())
	require.Equal(t, "r.example.com/deckhouse/ee@", prefix)
}

// TestAHalfStatedRegistryIsNotAStatedRegistry pins the boundary. A `deckhouse` section carrying a
// devBranch and nothing else — which is what a bundle installation's InitConfiguration looks like —
// states no registry, and reading it as one is what produced the failure above.
func TestAHalfStatedRegistryIsNotAStatedRegistry(t *testing.T) {
	for _, half := range []config.DeckhouseClusterConfig{
		{ImagesRepo: "r.example.com/deckhouse/ee"}, // no credentials
		{RegistryDockerCfg: "e30="},                // credentials, no repo
	} {
		conf := &config.MetaConfig{DeckhouseConfig: half}
		conf.Registry.Settings.RemoteData = registryconfig.Data{
			ImagesRepo: constant.BundleImagesRepo,
			Scheme:     constant.BundleScheme,
		}

		regConfig, _, err := imageSource(conf)
		require.NoError(t, err)
		require.Equal(t, constant.BundleImagesRepo, regConfig.GetRegistry(),
			"a half-stated registry must not win over the resolved one")
	}
}
