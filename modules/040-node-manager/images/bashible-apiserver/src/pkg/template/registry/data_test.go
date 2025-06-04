/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
)

func validRegistryData() *RegistryData {
	return &RegistryData{
		Mode:       "managed",
		ImagesBase: "example.com/base",
		Version:    "1.0",
		Hosts: map[string]registryHosts{
			"host1": validRegistryHost(),
		},
	}
}

func validRegistryHost() registryHosts {
	return registryHosts{
		Mirrors: []registryMirrorHost{
			validRegistryMirrorHost(),
		},
	}
}

func validRegistryMirrorHost() registryMirrorHost {
	return registryMirrorHost{
		Host:     "mirror1.example.com",
		Scheme:   "https",
		Auth:     registryAuth{},
		Rewrites: []registryRewrite{},
	}
}

func TestRegistryDataValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *RegistryData
		wantErr bool
	}{
		{
			name:    "Valid config",
			input:   validRegistryData(),
			wantErr: false,
		},
		{
			name: "Missing required hosts",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.Hosts = map[string]registryHosts{}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required mirror hosts",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.Hosts = map[string]registryHosts{"host1": {}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Mode",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.Mode = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required ImagesBase",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.ImagesBase = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Version",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.Version = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty ProxyEndpoint is invalid",
			input: func() *RegistryData {
				cfg := validRegistryData()
				cfg.ProxyEndpoints = []string{""}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Host is invalid",
			input: func() *RegistryData {
				cfg := validRegistryData()
				host := validRegistryHost()
				mirror := validRegistryMirrorHost()
				mirror.Host = ""
				host.Mirrors = append(host.Mirrors, mirror)
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Scheme is invalid",
			input: func() *RegistryData {
				cfg := validRegistryData()
				host := validRegistryHost()
				mirror := validRegistryMirrorHost()
				mirror.Scheme = ""
				host.Mirrors = append(host.Mirrors, mirror)
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected validation errors but got none")
			} else {
				assert.NoError(t, err, "Expected no validation errors but got some")
			}
		})
	}
}

func TestRegistryDataLoadFromInput(t *testing.T) {
	tests := []struct {
		name                    string
		deckhouseRegistrySecret deckhouseRegistrySecret
		bashibleConfigSecret    *bashibleConfigSecret
		wantRegistryData        *RegistryData
		wantErr                 bool
	}{
		{
			name: "Empty registry bashible config",
			deckhouseRegistrySecret: deckhouseRegistrySecret{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			bashibleConfigSecret: nil,
			wantRegistryData: &RegistryData{
				Mode:       "unmanaged",
				ImagesBase: "registry-1.com/test",
				Version:    "unknown",
				Hosts: map[string]registryHosts{
					"registry-1.com": {
						Mirrors: []registryMirrorHost{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "With registry bashible config",
			deckhouseRegistrySecret: deckhouseRegistrySecret{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			bashibleConfigSecret: &bashibleConfigSecret{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashibleConfigHosts{
					"registry-2.com": {
						Mirrors: []bashibleConfigMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			wantRegistryData: &RegistryData{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]registryHosts{
					"registry-2.com": {
						Mirrors: []registryMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rData := &RegistryData{}
			err := rData.loadFromInput(tt.deckhouseRegistrySecret, tt.bashibleConfigSecret)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}

			assert.Equal(t, tt.wantRegistryData, rData, "Expected and actual configurations do not match")
		})
	}
}
