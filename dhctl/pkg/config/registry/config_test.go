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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	registry_types "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

func TestNewConfig(t *testing.T) {
	type input struct {
		deckhouse  *registry_types.DeckhouseSettings
		initConfig *registry_types.InitConfig
		cri        registry_types.CRIType
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
			name: "mode: direct, containerd: v1 -> no errors",
			input: input{
				deckhouse: &registry_types.DeckhouseSettings{
					Mode: registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
						Username:   "user",
						Password:   "pass",
					},
				},
				initConfig: nil,
				cri:        registry_types.CRIContainerdV1,
			},
			output: output{
				err: false,
			},
		},
		{
			name: "mode: direct, containerd: unknown -> error",
			input: input{
				deckhouse: &registry_types.DeckhouseSettings{
					Mode: registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
						Username:   "user",
						Password:   "pass",
					},
				},
				initConfig: nil,
				cri:        registry_types.CRIType("unknown"),
			},
			output: output{
				err:    true,
				errMsg: "is not supported with defaultCRI",
			},
		},
		{
			name: "mode: unmanaged, containerd: unknown -> no errors",
			input: input{
				deckhouse: &registry_types.DeckhouseSettings{
					Mode: registry_const.ModeUnmanaged,
					Unmanaged: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
				initConfig: nil,
				cri:        registry_types.CRIType("unknown"),
			},
			output: output{
				err: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(tt.input.deckhouse, tt.input.initConfig, tt.input.cri)

			if tt.output.err {
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.input.deckhouse.Mode, config.Settings.Mode)
			}
		})
	}
}

func TestNewDeckhouseSettings(t *testing.T) {
	type input struct {
		deckhouse  *registry_types.DeckhouseSettings
		initConfig *registry_types.InitConfig
	}
	type output struct {
		err    bool
		errMsg string
		want   registry_types.DeckhouseSettings
	}

	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "empty inputs",
			input: input{
				deckhouse:  nil,
				initConfig: nil,
			},
			output: output{
				err: false,
				want: registry_types.DeckhouseSettings{
					Mode: registry_const.ModeUnmanaged,
					Unmanaged: &registry_types.RegistrySettings{
						ImagesRepo: registry_types.CEImagesRepo,
						Scheme:     registry_types.CEScheme,
					},
				},
			},
		},
		{
			name: "only deckhouse (with empty inputs)",
			input: input{
				deckhouse: &registry_types.DeckhouseSettings{
					Mode:   registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{},
				},
				initConfig: nil,
			},
			output: output{
				err: false,
				want: registry_types.DeckhouseSettings{
					Mode: registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{
						ImagesRepo: registry_types.CEImagesRepo,
						Scheme:     registry_types.CEScheme,
					},
				},
			},
		},
		{
			name: "only initConfig (with empty inputs)",
			input: input{
				deckhouse:  nil,
				initConfig: &registry_types.InitConfig{},
			},
			output: output{
				err: false,
				want: registry_types.DeckhouseSettings{
					Mode: registry_const.ModeUnmanaged,
					Unmanaged: &registry_types.RegistrySettings{
						ImagesRepo: registry_types.CEImagesRepo,
						Scheme:     registry_types.CEScheme,
					},
				},
			},
		},
		{
			name: "both - error",
			input: input{
				deckhouse:  &registry_types.DeckhouseSettings{},
				initConfig: &registry_types.InitConfig{},
			},
			output: output{
				err: true,
				errMsg: fmt.Sprintf(
					"duplicate registry configuration detected in initConfiguration.deckhouse " +
						"and moduleConfig/deckhouse.spec.settings.registry. Please specify registry settings in only one location."),
			},
		},
		{
			name: "deckhouse with trailing slashes - should be trimmed",
			input: input{
				deckhouse: &registry_types.DeckhouseSettings{
					Mode: registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com/",
						Scheme:     "HTTP",
					},
				},
				initConfig: nil,
			},
			output: output{
				err: false,
				want: registry_types.DeckhouseSettings{
					Mode: registry_const.ModeDirect,
					Direct: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTP",
					},
				},
			},
		},
		{
			name: "initConfig with trailing slashes - should be trimmed",
			input: input{
				deckhouse: nil,
				initConfig: &registry_types.InitConfig{
					ImagesRepo:     "registry.example.com/",
					RegistryScheme: "HTTP",
				},
			},
			output: output{
				err: false,
				want: registry_types.DeckhouseSettings{
					Mode: registry_const.ModeUnmanaged,
					Unmanaged: &registry_types.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTP",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewDeckhouseSettings(tt.input.deckhouse, tt.input.initConfig)

			if tt.output.err {
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.output.want, result)
			}
		})
	}
}
