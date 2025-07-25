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

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
)

func TestBashibleConfigSecretValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *bashibleConfigSecret
		wantErr bool
	}{
		{
			name: "Valid data",
			input: &bashibleConfigSecret{
				Mode:       "managed",
				ImagesBase: "example.com/base",
				Version:    "1.0",
				Hosts: map[string]bashible.ConfigHosts{
					"host1": {
						Mirrors: []bashible.ConfigMirrorHost{
							{
								Host:     "mirror1.example.com",
								Scheme:   "https",
								Auth:     bashible.ConfigAuth{},
								Rewrites: []bashible.ConfigRewrite{},
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
				Hosts: map[string]bashible.ConfigHosts{
					"host1.example.com": {
						Mirrors: []bashible.ConfigMirrorHost{
							{
								Host:   "mirror1.example.com",
								Scheme: "https",
								CA:     "ca1",
								Auth: bashible.ConfigAuth{
									Username: "username",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []bashible.ConfigRewrite{{
									From: "from",
									To:   "to",
								}},
							},
						},
					},
				},
			},

			wantRegistryData: RegistryData{
				RegistryModuleEnable: true,
				Mode:                 "managed",
				ImagesBase:           "example.com/base",
				Version:              "1.0",
				ProxyEndpoints:       []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashible.ContextHosts{
					"host1.example.com": {
						Mirrors: []bashible.ContextMirrorHost{
							{
								Host:   "mirror1.example.com",
								Scheme: "https",
								CA:     "ca1",
								Auth: bashible.ContextAuth{
									Username: "username",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []bashible.ContextRewrite{{
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
			err := tt.input.validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}
			assert.NoError(t, err, "Expected no validation error")

			registryData := tt.input.toRegistryData()
			assert.NoError(t, err, "Expected no error in ToRegistryData")
			assert.Equal(t, tt.wantRegistryData, *registryData, "RegistryData does not match expected")

			err = registryData.validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}
			assert.NoError(t, err, "Expected no validation error")
		})
	}
}
