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

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

type (
	updateRegistrySettings func(*module_config.RegistrySettings)
	updateLegacyMode       func() bool
	updateMode             func() constant.ModeType
	updateTTL              func() string
)

func ConfigBuilder(opts ...any) Config {
	var (
		mode             = constant.ModeUnmanaged
		legacyMode       = false
		ttl              = ""
		registrySettings = module_config.NewRegistrySettings()
	)

	for _, opt := range opts {
		switch fn := opt.(type) {
		case updateRegistrySettings:
			fn(&registrySettings)

		case updateLegacyMode:
			legacyMode = fn()

		case updateMode:
			mode = fn()

		case updateTTL:
			ttl = fn()
		}
	}

	var userSettings module_config.DeckhouseSettings

	switch mode {
	case constant.ModeDirect:
		userSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &registrySettings,
		}

	case constant.ModeProxy:
		userSettings = module_config.DeckhouseSettings{
			Mode: constant.ModeProxy,
			Proxy: &module_config.ProxySettings{
				RegistrySettings: registrySettings,
				TTL:              ttl,
			},
		}

	case constant.ModeLocal:
		userSettings = module_config.DeckhouseSettings{
			Mode: constant.ModeLocal,
		}

	default:
		userSettings = module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		}
	}

	settings := module_config.
		New(userSettings.Mode).
		Merge(&userSettings)

	var config Config
	if err := config.Process(settings, legacyMode); err != nil {
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

func WithTTL(ttl string) updateTTL {
	return func() string {
		return ttl
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

func WithModeProxy() updateMode {
	return func() constant.ModeType {
		return constant.ModeProxy
	}
}

func WithModeLocal() updateMode {
	return func() constant.ModeType {
		return constant.ModeLocal
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

			err := config.useDefault(tt.input.criSupported)
			require.NoError(t, err)

			require.Equal(t, tt.output.mode, config.Settings.Mode)
			require.Equal(t, tt.output.legacyMode, config.LegacyMode)
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

			err := config.useInitConfig(tt.input.initConfig)
			require.NoError(t, err)

			require.Equal(t, tt.output.mode, config.Settings.Mode)
			require.True(t, config.LegacyMode, "should be legacy mode")
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
			name: "proxy -> not legacy && no errors",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeProxy,
					Proxy: &module_config.ProxySettings{
						RegistrySettings: module_config.RegistrySettings{
							ImagesRepo: "registry.example.com",
							Scheme:     "HTTPS",
						},
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
		{
			name: "local -> not legacy && no errors",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeLocal,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.useDeckhouseSettings(tt.input.deckhouse)
			require.NoError(t, err)

			require.Equal(t, tt.input.deckhouse.Mode, config.Settings.Mode)
			require.False(t, config.LegacyMode, "should not be legacy mode")
		})
	}
}

func TestConfig_DeepCopy(t *testing.T) {
	t.Run("should create a deep copy of Config", func(t *testing.T) {
		tests := []struct {
			name   string
			config *Config
		}{
			{
				name: "Direct mode",
				config: func() *Config {
					c := ConfigBuilder(WithModeDirect())
					return &c
				}(),
			},
			{
				name: "Proxy mode",
				config: func() *Config {
					c := ConfigBuilder(WithModeProxy())
					return &c
				}(),
			},
			{
				name: "Unmanaged mode",
				config: func() *Config {
					c := ConfigBuilder(WithModeUnmanaged())
					return &c
				}(),
			},
			{
				name: "Local mode",
				config: func() *Config {
					c := ConfigBuilder(WithModeLocal())
					return &c
				}(),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				original := tt.config
				copied := original.DeepCopy()

				require.NotNil(t, copied)
				require.NotSame(t, original, copied)
				require.EqualValues(t, original, copied)
			})
		}
	})

	t.Run("should handle nil receiver", func(t *testing.T) {
		var nilConfig *Config
		copied := nilConfig.DeepCopy()
		require.Nil(t, copied)
	})
}

func TestIsLocalBootstrapMode(t *testing.T) {
	directSettings := module_config.DeckhouseSettings{
		Mode: constant.ModeDirect,
		Direct: &module_config.RegistrySettings{
			ImagesRepo: "registry.example.com",
			Scheme:     constant.SchemeHTTPS,
		},
	}
	localSettings := module_config.DeckhouseSettings{
		Mode: constant.ModeLocal,
	}
	initCfg := init_config.Config{
		ImagesRepo:     "registry.example.com",
		RegistryScheme: "HTTPS",
	}

	type input struct {
		initConfig        *init_config.Config
		deckhouseSettings *module_config.DeckhouseSettings
	}

	type output struct {
		isLocal bool
		err     bool
		errMsg  string
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "both configs -> error",
			input: input{
				initConfig:        &initCfg,
				deckhouseSettings: &directSettings,
			},
			output: output{
				err:    true,
				errMsg: "duplicate registry configuration detected",
			},
		},
		{
			name: "deckhouseSettings Local mode -> true",
			input: input{
				deckhouseSettings: &localSettings,
			},
			output: output{
				isLocal: true,
			},
		},
		{
			name: "deckhouseSettings Direct mode -> false",
			input: input{
				deckhouseSettings: &directSettings,
			},
			output: output{
				isLocal: false,
			},
		},
		{
			name: "initConfig only -> false",
			input: input{
				initConfig: &initCfg,
			},
			output: output{
				isLocal: false,
			},
		},
		{
			name:  "no config -> false",
			input: input{},
			output: output{
				isLocal: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isLocal, err := NewConfigProvider(tt.input.initConfig, tt.input.deckhouseSettings).IsLocal()

			if tt.output.err {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.output.errMsg)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.isLocal, isLocal)
		})
	}
}

func TestBootstrapRemoteData(t *testing.T) {
	directSettings := module_config.DeckhouseSettings{
		Mode: constant.ModeDirect,
		Direct: &module_config.RegistrySettings{
			ImagesRepo: "registry.example.com",
			Scheme:     constant.SchemeHTTPS,
		},
	}
	initCfg := init_config.Config{
		ImagesRepo:     "registry.example.com",
		RegistryScheme: "HTTPS",
	}

	type input struct {
		initConfig        *init_config.Config
		deckhouseSettings *module_config.DeckhouseSettings
	}

	type output struct {
		imagesRepo string
		err        bool
		errMsg     string
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "both configs -> error",
			input: input{
				initConfig:        &initCfg,
				deckhouseSettings: &directSettings,
			},
			output: output{
				err:    true,
				errMsg: "duplicate registry configuration detected",
			},
		},
		{
			name: "deckhouseSettings -> remote data from settings",
			input: input{
				deckhouseSettings: &directSettings,
			},
			output: output{
				imagesRepo: "registry.example.com",
			},
		},
		{
			name: "initConfig -> remote data from initConfig",
			input: input{
				initConfig: &initCfg,
			},
			output: output{
				imagesRepo: "registry.example.com",
			},
		},
		{
			name:  "no config -> default remote data",
			input: input{},
			output: output{
				imagesRepo: constant.DefaultImagesRepo,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := NewConfigProvider(tt.input.initConfig, tt.input.deckhouseSettings).RemoteData()

			if tt.output.err {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.output.errMsg)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.imagesRepo, data.ImagesRepo)
		})
	}
}

func TestBootstrapConfig(t *testing.T) {
	directSettings := module_config.DeckhouseSettings{
		Mode: constant.ModeDirect,
		Direct: &module_config.RegistrySettings{
			ImagesRepo: "registry.example.com",
			Scheme:     constant.SchemeHTTPS,
		},
	}
	proxySettings := module_config.DeckhouseSettings{
		Mode: constant.ModeProxy,
		Proxy: &module_config.ProxySettings{
			RegistrySettings: module_config.RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.SchemeHTTPS,
			},
		},
	}
	localSettings := module_config.DeckhouseSettings{
		Mode: constant.ModeLocal,
	}
	initCfg := init_config.Config{
		ImagesRepo:     "registry.example.com",
		RegistryScheme: "HTTPS",
	}

	type input struct {
		initConfig        *init_config.Config
		deckhouseSettings *module_config.DeckhouseSettings
		defaultCRI        constant.CRIType
		isStatic          bool
	}

	type output struct {
		mode       constant.ModeType
		legacyMode bool
		err        bool
		errMsg     string
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "both configs -> error",
			input: input{
				initConfig:        &initCfg,
				deckhouseSettings: &directSettings,
				defaultCRI:        constant.CRIContainerdV1,
				isStatic:          true,
			},
			output: output{
				err:    true,
				errMsg: "duplicate registry configuration detected",
			},
		},
		{
			name: "deckhouseSettings + supported CRI -> direct mode",
			input: input{
				deckhouseSettings: &directSettings,
				defaultCRI:        constant.CRIContainerdV1,
				isStatic:          true,
			},
			output: output{
				mode:       constant.ModeDirect,
				legacyMode: false,
			},
		},
		{
			name: "deckhouseSettings + unsupported CRI -> error",
			input: input{
				deckhouseSettings: &directSettings,
				defaultCRI:        "Docker",
			},
			output: output{
				err:    true,
				errMsg: "registry module cannot be started with defaultCRI",
			},
		},
		{
			name: "deckhouseSettings proxy + non-static -> error",
			input: input{
				deckhouseSettings: &proxySettings,
				defaultCRI:        constant.CRIContainerdV1,
				isStatic:          false,
			},
			output: output{
				err:    true,
				errMsg: "bootstrap with registry mode",
			},
		},
		{
			name: "deckhouseSettings local + static -> local mode",
			input: input{
				deckhouseSettings: &localSettings,
				defaultCRI:        constant.CRIContainerdV1,
				isStatic:          true,
			},
			output: output{
				mode:       constant.ModeLocal,
				legacyMode: false,
			},
		},
		{
			name: "deckhouseSettings local + non-static -> error",
			input: input{
				deckhouseSettings: &localSettings,
				defaultCRI:        constant.CRIContainerdV1,
				isStatic:          false,
			},
			output: output{
				err:    true,
				errMsg: "bootstrap with registry mode",
			},
		},
		{
			name: "initConfig -> unmanaged legacy mode",
			input: input{
				initConfig: &initCfg,
				defaultCRI: constant.CRIContainerdV1,
			},
			output: output{
				mode:       constant.ModeUnmanaged,
				legacyMode: true,
			},
		},
		{
			name: "no config + supported CRI -> direct mode",
			input: input{
				defaultCRI: constant.CRIContainerdV1,
			},
			output: output{
				mode:       constant.ModeDirect,
				legacyMode: false,
			},
		},
		{
			name: "no config + unsupported CRI -> unmanaged legacy mode",
			input: input{
				defaultCRI: "Docker",
			},
			output: output{
				mode:       constant.ModeUnmanaged,
				legacyMode: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfigProvider(
				tt.input.initConfig,
				tt.input.deckhouseSettings,
			).MetaConfig(
				tt.input.defaultCRI,
				tt.input.isStatic,
			)

			if tt.output.err {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.output.errMsg)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.mode, config.Settings.Mode)
			require.Equal(t, tt.output.legacyMode, config.LegacyMode)
		})
	}
}
