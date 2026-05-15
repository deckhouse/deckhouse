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

// validDockerCfg is a base64-encoded docker config for registry.example.com.
// Decoded: {"auths":{"registry.example.com":{"username":"user","password":"pass","auth":"dXNlcjpwYXNz"}}}
const validDockerCfg = "eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlLmNvbSI6eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InBhc3MiLCJhdXRoIjoiZFhObGNqcHdZWE56In19fQ=="

func TestParseJSONInitConfig(t *testing.T) {
	type output struct {
		config *init_config.Config
		err    bool
	}

	tests := []struct {
		name   string
		input  []byte
		output output
	}{
		{
			name:  "empty input -> nil config",
			input: []byte{},
			output: output{
				config: nil,
			},
		},
		{
			name:  "empty deckhouse section -> nil config",
			input: []byte(`{"deckhouse":{}}`),
			output: output{
				config: nil,
			},
		},
		{
			name:  "missing deckhouse section -> nil config",
			input: []byte(`{}`),
			output: output{
				config: nil,
			},
		},
		{
			name: "imagesRepo only -> config with repo",
			input: []byte(`{
				"deckhouse": {
					"imagesRepo": "registry.example.com",
					"registryScheme": "HTTPS"
				}
			}`),
			output: output{
				config: &init_config.Config{
					ImagesRepo:     "registry.example.com",
					RegistryScheme: "HTTPS",
				},
			},
		},
		{
			name: "all fields -> full config",
			input: []byte(`{
				"deckhouse": {
					"imagesRepo": "registry.example.com",
					"registryScheme": "HTTPS",
					"registryDockerCfg": "` + validDockerCfg + `",
					"registryCA": "-----BEGIN CERTIFICATE-----"
				}
			}`),
			output: output{
				config: &init_config.Config{
					ImagesRepo:        "registry.example.com",
					RegistryScheme:    "HTTPS",
					RegistryDockerCfg: validDockerCfg,
					RegistryCA:        "-----BEGIN CERTIFICATE-----",
				},
			},
		},
		{
			name:  "invalid JSON -> error",
			input: []byte(`{not valid json`),
			output: output{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseJSONInitConfig(tt.input)

			if tt.output.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.config, config)
		})
	}
}

func TestParseYAMLInitConfig(t *testing.T) {
	type output struct {
		config *init_config.Config
		err    bool
	}

	tests := []struct {
		name   string
		input  []byte
		output output
	}{
		{
			name:  "empty input -> nil config",
			input: []byte{},
			output: output{
				config: nil,
			},
		},
		{
			name:  "empty deckhouse section -> nil config",
			input: []byte("deckhouse: {}"),
			output: output{
				config: nil,
			},
		},
		{
			name:  "missing deckhouse section -> nil config",
			input: []byte("{}"),
			output: output{
				config: nil,
			},
		},
		{
			name: "imagesRepo only -> config with repo",
			input: []byte(`
deckhouse:
  imagesRepo: registry.example.com
  registryScheme: HTTPS
`),
			output: output{
				config: &init_config.Config{
					ImagesRepo:     "registry.example.com",
					RegistryScheme: "HTTPS",
				},
			},
		},
		{
			name: "all fields -> full config",
			input: []byte(`
deckhouse:
  imagesRepo: registry.example.com
  registryScheme: HTTPS
  registryDockerCfg: ` + validDockerCfg + `
  registryCA: "-----BEGIN CERTIFICATE-----"
`),
			output: output{
				config: &init_config.Config{
					ImagesRepo:        "registry.example.com",
					RegistryScheme:    "HTTPS",
					RegistryDockerCfg: validDockerCfg,
					RegistryCA:        "-----BEGIN CERTIFICATE-----",
				},
			},
		},
		{
			name:  "invalid YAML -> error",
			input: []byte("deckhouse: [invalid: yaml"),
			output: output{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseYAMLInitConfig(tt.input)

			if tt.output.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.config, config)
		})
	}
}

func TestParseJSONDeckhouseMC(t *testing.T) {
	type output struct {
		settings *module_config.DeckhouseSettings
		err      bool
	}

	tests := []struct {
		name   string
		input  []byte
		output output
	}{
		{
			name:  "empty input -> nil settings",
			input: []byte{},
			output: output{
				settings: nil,
			},
		},
		{
			name:  "missing registry section -> nil settings",
			input: []byte(`{"spec":{"settings":{}}}`),
			output: output{
				settings: nil,
			},
		},
		{
			name: "direct mode -> settings with mode",
			input: []byte(`{
				"spec": {
					"settings": {
						"registry": {
							"mode": "Direct",
							"direct": {
								"imagesRepo": "registry.example.com",
								"scheme": "HTTPS"
							}
						}
					}
				}
			}`),
			output: output{
				settings: &module_config.DeckhouseSettings{
					Mode: constant.ModeDirect,
					Direct: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     constant.SchemeHTTPS,
					},
				},
			},
		},
		{
			name: "unmanaged mode -> settings with mode",
			input: []byte(`{
				"spec": {
					"settings": {
						"registry": {
							"mode": "Unmanaged",
							"unmanaged": {
								"imagesRepo": "registry.example.com",
								"scheme": "HTTPS"
							}
						}
					}
				}
			}`),
			output: output{
				settings: &module_config.DeckhouseSettings{
					Mode: constant.ModeUnmanaged,
					Unmanaged: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     constant.SchemeHTTPS,
					},
				},
			},
		},
		{
			name:  "invalid JSON -> error",
			input: []byte(`{not valid json`),
			output: output{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := ParseJSONDeckhouseMC(tt.input)

			if tt.output.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.settings, settings)
		})
	}
}

func TestParseYAMLDeckhouseMC(t *testing.T) {
	type output struct {
		settings *module_config.DeckhouseSettings
		err      bool
	}

	tests := []struct {
		name   string
		input  []byte
		output output
	}{
		{
			name:  "empty input -> nil settings",
			input: []byte{},
			output: output{
				settings: nil,
			},
		},
		{
			name:  "missing registry section -> nil settings",
			input: []byte("spec:\n  settings: {}"),
			output: output{
				settings: nil,
			},
		},
		{
			name: "direct mode -> settings with mode",
			input: []byte(`
spec:
  settings:
    registry:
      mode: Direct
      direct:
        imagesRepo: registry.example.com
        scheme: HTTPS
`),
			output: output{
				settings: &module_config.DeckhouseSettings{
					Mode: constant.ModeDirect,
					Direct: &module_config.RegistrySettings{
						ImagesRepo: "registry.example.com",
						Scheme:     constant.SchemeHTTPS,
					},
				},
			},
		},
		{
			name: "proxy mode with credentials -> settings with credentials",
			input: []byte(`
spec:
  settings:
    registry:
      mode: Proxy
      proxy:
        imagesRepo: registry.example.com
        scheme: HTTPS
        username: user
        password: pass
        ttl: 72h
`),
			output: output{
				settings: &module_config.DeckhouseSettings{
					Mode: constant.ModeProxy,
					Proxy: &module_config.ProxySettings{
						RegistrySettings: module_config.RegistrySettings{
							ImagesRepo: "registry.example.com",
							Scheme:     constant.SchemeHTTPS,
							Username:   "user",
							Password:   "pass",
						},
						TTL: "72h",
					},
				},
			},
		},
		{
			name: "local mode -> settings with mode only",
			input: []byte(`
spec:
  settings:
    registry:
      mode: Local
`),
			output: output{
				settings: &module_config.DeckhouseSettings{
					Mode: constant.ModeLocal,
				},
			},
		},
		{
			name:  "invalid YAML -> error",
			input: []byte("spec: [invalid: yaml"),
			output: output{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := ParseYAMLDeckhouseMC(tt.input)

			if tt.output.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.output.settings, settings)
		})
	}
}
