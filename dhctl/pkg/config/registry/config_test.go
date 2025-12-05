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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/init-config"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

func TestConfig_UseDefault(t *testing.T) {
	type input struct {
		cri constant.CRIType
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
			name: "containerd: v1 -> Direct",
			input: input{
				cri: constant.CRIContainerdV1,
			},
			output: output{
				mode: constant.ModeDirect,
				err:  false,
			},
		},
		{
			name: "containerd: unknown -> Unmanaged",
			input: input{
				cri: constant.CRIType("unknown"),
			},
			output: output{
				mode: constant.ModeUnmanaged,
				err:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			err := config.UseDefault(tt.input.cri)

			if tt.output.err {
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.output.mode, config.Settings.Mode)
			}
		})
	}
}

func TestConfig_UseInitConfig(t *testing.T) {
	type input struct {
		initConfig init_config.Config
		cri        constant.CRIType
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
			name: "containerd: v1 -> Unmanaged",
			input: input{
				initConfig: init_config.Config{
					ImagesRepo:     "registry.example.com",
					RegistryScheme: "HTTPS",
				},
				cri: constant.CRIContainerdV1,
			},
			output: output{
				mode: constant.ModeUnmanaged,
				err:  false,
			},
		},
		{
			name: "containerd: unknown -> Unmanaged",
			input: input{
				initConfig: init_config.Config{
					ImagesRepo:     "registry.example.com",
					RegistryScheme: "HTTPS",
				},
				cri: constant.CRIType("unknown"),
			},
			output: output{
				mode: constant.ModeUnmanaged,
				err:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			err := config.UseInitConfig(tt.input.initConfig, tt.input.cri)

			if tt.output.err {
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.output.mode, config.Settings.Mode)
			}
		})
	}
}

func TestConfig_UseDeckhouseSettings(t *testing.T) {
	type input struct {
		deckhouse module_config.DeckhouseSettings
		cri       constant.CRIType
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
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeDirect,
					Direct: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
				cri: constant.CRIContainerdV1,
			},
			output: output{
				err: false,
			},
		},
		{
			name: "mode: direct, containerd: unknown -> error",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeDirect,
					Direct: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
				cri: constant.CRIType("unknown"),
			},
			output: output{
				err:    true,
				errMsg: "is not supported with defaultCRI",
			},
		},
		{
			name: "mode: unmanaged, containerd: unknown -> no errors",
			input: input{
				deckhouse: module_config.DeckhouseSettings{
					Mode: constant.ModeUnmanaged,
					Unmanaged: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     "HTTPS",
					},
				},
				cri: constant.CRIType("unknown"),
			},
			output: output{
				err: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			err := config.UseDeckhouseSettings(tt.input.deckhouse, tt.input.cri)

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
