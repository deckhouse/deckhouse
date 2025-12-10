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

func TestConfig_UseDefault(t *testing.T) {
	type input struct {
		criSupported bool
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
			name: "criSupported -> direct && no error",
			input: input{
				criSupported: true,
			},
			output: output{
				mode:       constant.ModeDirect,
				legacyMode: false,
				err:        false,
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
				err:        false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseDefault(tt.input.criSupported)

			if tt.output.err {
				require.Error(t, err)

				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)

				assert.Equal(t, tt.output.mode, config.Settings.Mode)
				assert.Equal(t, tt.output.legacyMode, config.LegacyMode)
			}
		})
	}
}

func TestConfig_UseInitConfig(t *testing.T) {
	type input struct {
		initConfig init_config.Config
	}

	type output struct {
		mode   constant.ModeType
		err    bool
		errMsg string
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
				err:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseInitConfig(tt.input.initConfig)
			if tt.output.err {
				require.Error(t, err)

				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)

				assert.Equal(t, tt.output.mode, config.Settings.Mode)
				assert.True(t, config.LegacyMode, "should be legacy mode")
			}
		})
	}
}

func TestConfig_UseDeckhouseSettings(t *testing.T) {
	type input struct {
		deckhouse module_config.DeckhouseSettings
	}

	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		input  input
		output output
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
			output: output{
				err: false,
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
			output: output{
				err: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config

			err := config.UseDeckhouseSettings(tt.input.deckhouse)
			if tt.output.err {
				require.Error(t, err)

				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)

				assert.Equal(t, tt.input.deckhouse.Mode, config.Settings.Mode)
				assert.False(t, config.LegacyMode, "should not be legacy mode")
			}
		})
	}
}
