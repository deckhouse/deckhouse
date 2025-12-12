// Copyright 2025 Flant JSC
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/init-config"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type (
	updateRegistrySettings func(*module_config.RegistrySettings)
	updateLegacyMode       func() bool
	updateMode             func() constant.ModeType
)

func ConfigBuilder(opts ...any) Config {
	var (
		mode             = constant.ModeUnmanaged
		legacyMode       = false
		registrySettings = module_config.RegistrySettings{
			ImagesRepo: constant.CEImagesRepo,
			Scheme:     constant.CEScheme,
		}
	)

	for _, opt := range opts {
		switch fn := opt.(type) {
		case updateRegistrySettings:
			fn(&registrySettings)

		case updateLegacyMode:
			legacyMode = fn()

		case updateMode:
			mode = fn()
		}
	}

	var deckhouseSettings module_config.DeckhouseSettings

	switch mode {
	case constant.ModeDirect:
		deckhouseSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &registrySettings,
		}

	default:
		deckhouseSettings = module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		}
	}

	var config Config

	if err := config.Process(deckhouseSettings, legacyMode); err != nil {
		panic(err)
	}

	return config
}

func WithImagesRepo(repo string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.ImagesRepo = repo
	}
}

func WithSchemeHTTP() updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTP
	}
}

func WithSchemeHTTPS() updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTPS
	}
}

func WithCredentials(username, password string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Username = username
		rs.Password = password
	}
}

func WithCA(ca string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.CA = ca
	}
}

func WithLicense(license string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.License = license
	}
}

func WithModeDirect() updateMode {
	return func() constant.ModeType {
		return constant.ModeDirect
	}
}

func WithModeUnmanaged() updateMode {
	return func() constant.ModeType {
		return constant.ModeUnmanaged
	}
}

func WithLegacyMode() updateLegacyMode {
	return func() bool {
		return true
	}
}

func TestConfig_UseDefault(t *testing.T) {
	type input struct {
		criSupported bool
	}

	type output struct {
		mode       constant.ModeType
		legacyMode bool
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "criSupported -> direct && no error",
			input: input{
				criSupported: true,
			},
			output: output{
				mode:       constant.ModeDirect,
				legacyMode: false,
			},
		},
		{
			name: "not criSupported -> unmanaged && no error",
			input: input{
				criSupported: false,
			},
			output: output{
				mode:       constant.ModeUnmanaged,
				legacyMode: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseDefault(tt.input.criSupported)
			require.NoError(t, err)

			assert.Equal(t, tt.output.mode, config.Settings.Mode)
			assert.Equal(t, tt.output.legacyMode, config.LegacyMode)
		})
	}
}

func TestConfig_UseInitConfig(t *testing.T) {
	type input struct {
		initConfig init_config.Config
	}

	type output struct {
		mode constant.ModeType
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "iniConfig -> unmanaged && legacy && no error",
			input: input{
				initConfig: init_config.Config{
					ImagesRepo:     "registry.example.com",
					RegistryScheme: "HTTPS",
				},
			},
			output: output{
				mode: constant.ModeUnmanaged,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseInitConfig(tt.input.initConfig)
			require.NoError(t, err)

			assert.Equal(t, tt.output.mode, config.Settings.Mode)
			assert.True(t, config.LegacyMode, "should be legacy mode")
		})
	}
}

func TestConfig_UseDeckhouseSettings(t *testing.T) {
	type input struct {
		deckhouse module_config.DeckhouseSettings
	}

	tests := []struct {
		name  string
		input input
	}{
		{
			name: "direct -> not legacy && no errors",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeDirect,
					Direct: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
			},
		},
		{
			name: "unmanaged -> not legacy && no errors",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeUnmanaged,
					Unmanaged: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseDeckhouseSettings(tt.input.deckhouse)
			require.NoError(t, err)

			assert.Equal(t, tt.input.deckhouse.Mode, config.Settings.Mode)
			assert.False(t, config.LegacyMode, "should not be legacy mode")
		})
	}
}
