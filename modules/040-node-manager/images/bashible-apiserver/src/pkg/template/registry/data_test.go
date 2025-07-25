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
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

func TestRegistryDataValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *RegistryData
		wantErr bool
	}{
		{
			name: "Valid data",
			input: &RegistryData{
				RegistryModuleEnable: true,
				Mode:                 "managed",
				ImagesBase:           "example.com/base",
				Version:              "1.0",
				Hosts: map[string]bashible.ContextHosts{
					"host1": {
						Mirrors: []bashible.ContextMirrorHost{
							{
								Host:     "mirror1.example.com",
								Scheme:   "https",
								Auth:     bashible.ContextAuth{},
								Rewrites: []bashible.ContextRewrite{},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Epmty data",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
			}
		})
	}
}

func TestRegistryDataToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   *RegistryData
		wantErr bool
	}{
		{
			name: "Valid data",
			input: &RegistryData{
				RegistryModuleEnable: true,
				Mode:                 "unmanaged",
				Version:              "unknown",
				ImagesBase:           "registry.d8-system.svc/deckhouse/system",
				ProxyEndpoints:       []string{"192.168.1.1"},
				Hosts: map[string]bashible.ContextHosts{
					"registry.d8-system.svc": {
						Mirrors: []bashible.ContextMirrorHost{{
							Host:   "r.example.com",
							Scheme: "https",
							CA:     "==exampleCA==",
							Auth: bashible.ContextAuth{
								Username: "user",
								Password: "password",
								Auth:     "auth"},
							Rewrites: []bashible.ContextRewrite{{
								From: "^deckhouse/system",
								To:   "deckhouse/ce"}}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Enpty data",
			input:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.input.toMap()
			if tt.wantErr {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
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
				RegistryModuleEnable: false,
				Mode:                 "unmanaged",
				ImagesBase:           "registry-1.com/test",
				Version:              "unknown",
				Hosts: map[string]bashible.ContextHosts{
					"registry-1.com": {
						Mirrors: []bashible.ContextMirrorHost{{Host: "registry-1.com", Scheme: "https"}},
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
				Hosts: map[string]bashible.ConfigHosts{
					"registry-2.com": {
						Mirrors: []bashible.ConfigMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			wantRegistryData: &RegistryData{
				RegistryModuleEnable: true,
				Mode:                 "proxy",
				ImagesBase:           "registry-2.com/test",
				Version:              "1",
				ProxyEndpoints:       []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashible.ContextHosts{
					"registry-2.com": {
						Mirrors: []bashible.ContextMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
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

func TestRegistryDatahHashSum(t *testing.T) {
	data := RegistryData{
		RegistryModuleEnable: true,
		Mode:                 "unmanaged",
		Version:              "unknown",
		ImagesBase:           "registry.d8-system.svc/deckhouse/system",
		ProxyEndpoints:       []string{"192.168.1.1"},
		Hosts: map[string]bashible.ContextHosts{
			"registry.d8-system.svc": {
				Mirrors: []bashible.ContextMirrorHost{{
					Host:   "r.example.com",
					Scheme: "https",
					CA:     "==exampleCA==",
					Auth: bashible.ContextAuth{
						Username: "user",
						Password: "password",
						Auth:     "auth"},
					Rewrites: []bashible.ContextRewrite{{
						From: "^deckhouse/system",
						To:   "deckhouse/ce"}}},
				},
			},
		},
	}

	t.Run("Equal hash", func(t *testing.T) {
		hash1, err1 := data.hashSum()
		hash2, err2 := data.hashSum()

		require.NoError(t, err1, "unexpected error while computing first hash")
		require.NoError(t, err2, "unexpected error while computing second hash")

		require.NotEmpty(t, hash1, "hash should not be empty")
		assert.Equal(t, hash1, hash2, "hash should be deterministic and equal on repeated calls")
	})
}
