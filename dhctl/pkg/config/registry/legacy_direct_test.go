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

package registry

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// commanderDirect is what Commander writes into the deckhouse ModuleConfig when nobody stated a
// registry: the mode of the previous implementation, carrying the registry the cluster uses.
func commanderDirect(settings module_config.RegistrySettings) *module_config.DeckhouseSettings {
	return &module_config.DeckhouseSettings{
		Mode:   constant.ModeDirect,
		Direct: &settings,
	}
}

func credentialsIn(t *testing.T, config *init_config.Config, address string) (string, string) {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(config.RegistryDockerCfg)
	require.NoError(t, err, "the docker configuration is stored the way InitConfiguration carries it")

	username, password, err := helpers.CredsFromDockerCfg(raw, address)
	require.NoError(t, err)

	return username, password
}

// TestFoldLegacyDirectIntoInit is the whole of the workaround: which configurations are read as an
// address and which are left to mean what they say.
func TestFoldLegacyDirectIntoInit(t *testing.T) {
	t.Run("Direct with credentials becomes the installer's registry", func(t *testing.T) {
		init, settings, err := FoldLegacyDirectIntoInit(nil, commanderDirect(module_config.RegistrySettings{
			ImagesRepo: "registry.example.com/deckhouse/ee",
			Scheme:     constant.SchemeHTTPS,
			CA:         "CA-CERT",
			Username:   "robot",
			Password:   "secret",
		}))
		require.NoError(t, err)

		assert.Nil(t, settings, "nothing is left to ask the previous implementation for")
		require.NotNil(t, init)
		assert.Equal(t, "registry.example.com/deckhouse/ee", init.ImagesRepo)
		assert.Equal(t, string(constant.SchemeHTTPS), init.RegistryScheme)
		assert.Equal(t, "CA-CERT", init.RegistryCA)

		username, password := credentialsIn(t, init, "registry.example.com")
		assert.Equal(t, "robot", username)
		assert.Equal(t, "secret", password)
	})

	t.Run("a license is folded as the credentials it stands for", func(t *testing.T) {
		init, _, err := FoldLegacyDirectIntoInit(nil, commanderDirect(module_config.RegistrySettings{
			ImagesRepo: "registry.example.com/deckhouse/ee",
			Scheme:     constant.SchemeHTTPS,
			License:    "LICENSE-KEY",
		}))
		require.NoError(t, err)
		require.NotNil(t, init)

		username, password := credentialsIn(t, init, "registry.example.com")
		assert.Equal(t, constant.LicenseUsername, username)
		assert.Equal(t, "LICENSE-KEY", password)
	})

	t.Run("no credentials, no docker configuration", func(t *testing.T) {
		init, _, err := FoldLegacyDirectIntoInit(nil, commanderDirect(module_config.RegistrySettings{
			ImagesRepo: "registry.example.com/deckhouse/ce",
			Scheme:     constant.SchemeHTTPS,
		}))
		require.NoError(t, err)
		require.NotNil(t, init)
		assert.Empty(t, init.RegistryDockerCfg, "an anonymous registry gets no credentials invented for it")
	})

	t.Run("an InitConfiguration that names a registry wins", func(t *testing.T) {
		stated := &init_config.Config{
			ImagesRepo:     "registry.stated.example.com/deckhouse/ee",
			RegistryScheme: string(constant.SchemeHTTPS),
		}

		init, settings, err := FoldLegacyDirectIntoInit(stated, commanderDirect(module_config.RegistrySettings{
			ImagesRepo: "registry.example.com/deckhouse/ee",
			Scheme:     constant.SchemeHTTPS,
		}))
		require.NoError(t, err)

		assert.Same(t, stated, init, "the operator's own statement is not rewritten")
		assert.Nil(t, settings, "and the section that would have overridden it is dropped")
	})

	t.Run("modes that mean what they say are left alone", func(t *testing.T) {
		for _, mode := range []constant.ModeType{
			constant.ModeUnmanaged,
			constant.ModeProxy,
			constant.ModeLocal,
		} {
			t.Run(string(mode), func(t *testing.T) {
				original := &module_config.DeckhouseSettings{Mode: mode}

				init, settings, err := FoldLegacyDirectIntoInit(nil, original)
				require.NoError(t, err)

				assert.Nil(t, init)
				assert.Same(t, original, settings)
			})
		}
	})

	t.Run("no settings at all", func(t *testing.T) {
		init, settings, err := FoldLegacyDirectIntoInit(nil, nil)
		require.NoError(t, err)

		assert.Nil(t, init)
		assert.Nil(t, settings)
	})
}

// TestAFoldedDirectInstallsWithTheModuleManagingNothing is the reason the fold is worth having: what
// the installation ends up with.
//
// LegacyMode is the load-bearing half. It is what keeps dhctl from writing the registry section back
// into the deckhouse ModuleConfig of the installed cluster, so the registry module comes up at its
// default — managing nothing — while the nodes pull from the registry the folded section named.
func TestAFoldedDirectInstallsWithTheModuleManagingNothing(t *testing.T) {
	init, settings, err := FoldLegacyDirectIntoInit(nil, commanderDirect(module_config.RegistrySettings{
		ImagesRepo: "registry.example.com/deckhouse/ee",
		Scheme:     constant.SchemeHTTPS,
		Username:   "robot",
		Password:   "secret",
	}))
	require.NoError(t, err)

	config, err := NewConfigProvider(init, settings).Config(constant.CRIContainerdV1, true, true)
	require.NoError(t, err)

	assert.True(t, config.LegacyMode, "the module is not asked to manage anything")
	assert.Equal(t, constant.ModeUnmanaged, config.Settings.Mode)
	assert.Equal(t, "registry.example.com/deckhouse/ee", config.Settings.RemoteData.ImagesRepo)
	assert.Equal(t, "robot", config.Settings.RemoteData.Username)
	assert.Equal(t, "secret", config.Settings.RemoteData.Password)
}

// TestAnUnfoldedDirectStillAsksForTheModule pins the behaviour the fold replaces, so that a change
// of mind about it shows up here rather than in a cluster.
func TestAnUnfoldedDirectStillAsksForTheModule(t *testing.T) {
	settings := commanderDirect(module_config.RegistrySettings{
		ImagesRepo: "registry.example.com/deckhouse/ee",
		Scheme:     constant.SchemeHTTPS,
	})

	config, err := NewConfigProvider(nil, settings).Config(constant.CRIContainerdV1, true, true)
	require.NoError(t, err)

	assert.False(t, config.LegacyMode)
	assert.Equal(t, constant.ModeDirect, config.Settings.Mode,
		"which is the mode nothing in this release renders the objects for")
}
