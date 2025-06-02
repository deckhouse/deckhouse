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

func validBashibleConfigSecret() *bashibleConfigSecret {
	return &bashibleConfigSecret{
		Mode:       "managed",
		ImagesBase: "example.com/base",
		Version:    "1.0",
		Hosts: map[string]bashibleConfigHosts{
			"host1": validSecretHost(),
		},
	}
}

func validSecretHost() bashibleConfigHosts {
	return bashibleConfigHosts{
		Mirrors: []bashibleConfigMirrorHost{
			validbashibleConfigMirrorHost(),
		},
	}
}

func validbashibleConfigMirrorHost() bashibleConfigMirrorHost {
	return bashibleConfigMirrorHost{
		Host:     "mirror1.example.com",
		Scheme:   "https",
		Auth:     bashibleConfigAuth{},
		Rewrites: []bashibleConfigRewrite{},
	}
}

func TestBashibleConfigSecretValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *bashibleConfigSecret
		wantErr bool
	}{
		{
			name:    "Valid config",
			input:   validBashibleConfigSecret(),
			wantErr: false,
		},
		{
			name: "Missing required hosts",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.Hosts = map[string]bashibleConfigHosts{}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required mirror hosts",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.Hosts = map[string]bashibleConfigHosts{"host1": {}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Mode",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.Mode = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required ImagesBase",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.ImagesBase = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Version",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.Version = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty ProxyEndpoint is invalid",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				cfg.ProxyEndpoints = []string{""}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty CA is invalid",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				host := validSecretHost()
				host.CA = []string{""}
				cfg.Hosts["host2"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Host is invalid",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				host := validSecretHost()
				mirror := validbashibleConfigMirrorHost()
				mirror.Host = ""
				host.Mirrors = append(host.Mirrors, mirror)
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Scheme is invalid",
			input: func() *bashibleConfigSecret {
				cfg := validBashibleConfigSecret()
				host := validSecretHost()
				mirror := validbashibleConfigMirrorHost()
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

func TestBashibleConfigSecretToRegistryData(t *testing.T) {
	tests := []struct {
		name             string
		input            bashibleConfigSecret
		wantRegistryData RegistryData
	}{
		{
			name: "With non-empty fields",
			input: bashibleConfigSecret{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashibleConfigHosts{
					"host1.example.com": {
						CA: []string{"ca1"},
						Mirrors: []bashibleConfigMirrorHost{
							{
								Host:   "mirror1.example.com",
								Scheme: "https",
								Auth: bashibleConfigAuth{
									Username: "username",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []bashibleConfigRewrite{{
									From: "from",
									To:   "to",
								}},
							},
						},
					},
				},
			},

			wantRegistryData: RegistryData{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]registryHosts{
					"host1.example.com": {
						CA: []string{"ca1"},
						Mirrors: []registryMirrorHost{
							{
								Host:   "mirror1.example.com",
								Scheme: "https",
								Auth: registryAuth{
									Username: "username",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []registryRewrite{{
									From: "from",
									To:   "to",
								}},
							},
						},
					},
				},
			},
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
			assert.NoError(t, err, "Expected no validation error")

			registryData := tt.input.toRegistryData()
			assert.NoError(t, err, "Expected no error in ToRegistryData")
			assert.Equal(t, tt.wantRegistryData, *registryData, "RegistryData does not match expected")

			err = registryData.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}
			assert.NoError(t, err, "Expected no validation error")
		})
	}
}
