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

package bashible

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
)

func validConfig() *Config {
	return &Config{
		Mode:       "managed",
		ImagesBase: "example.com/base",
		Version:    "1.0",
		Hosts: map[string]Hosts{
			"host1": validRegistryHost(),
		},
	}
}

func validRegistryHost() Hosts {
	return Hosts{
		Mirrors: []MirrorHost{
			validMirrorHost(),
		},
	}
}

func validMirrorHost() MirrorHost {
	return MirrorHost{
		Host:     "mirror1.example.com",
		Scheme:   "https",
		Auth:     Auth{},
		Rewrites: []Rewrite{},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *Config
		wantErr bool
	}{
		{
			name:    "Valid config",
			input:   validConfig(),
			wantErr: false,
		},
		{
			name: "Missing required hosts",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]Hosts{}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required mirror hosts",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]Hosts{"host1": {}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Mode",
			input: func() *Config {
				cfg := validConfig()
				cfg.Mode = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required ImagesBase",
			input: func() *Config {
				cfg := validConfig()
				cfg.ImagesBase = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Version",
			input: func() *Config {
				cfg := validConfig()
				cfg.Version = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty ProxyEndpoint is invalid",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{""}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Host is invalid",
			input: func() *Config {
				cfg := validConfig()
				host := validRegistryHost()
				mirror := validMirrorHost()
				mirror.Host = ""
				host.Mirrors = []MirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Scheme is invalid",
			input: func() *Config {
				cfg := validConfig()
				host := validRegistryHost()
				mirror := validMirrorHost()
				mirror.Scheme = ""
				host.Mirrors = []MirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Duplicate Mirrors",
			input: func() *Config {
				cfg := validConfig()
				host := validRegistryHost()
				mirror := validMirrorHost()
				host.Mirrors = []MirrorHost{mirror, mirror}
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
