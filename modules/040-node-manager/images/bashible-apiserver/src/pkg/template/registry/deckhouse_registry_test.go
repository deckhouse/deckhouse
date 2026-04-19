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

func TestDeckhouseRegistrySecretToRegistryData(t *testing.T) {
	tests := []struct {
		name             string
		input            deckhouseRegistrySecret
		wantRegistryData RegistryData
	}{
		{
			name: "With non-empty fields",
			input: deckhouseRegistrySecret{
				Address:      "example.com",
				Path:         "/base",
				Scheme:       "https",
				CA:           "",
				DockerConfig: []byte(`{"auths":{"example.com":{"auth":"dXNlcjpwYXNz"}}}`),
			},
			wantRegistryData: RegistryData{
				RegistryModuleEnable: false,
				Mode:                 "unmanaged",
				ImagesBase:           "example.com/base",
				Version:              "unknown",
				Hosts: map[string]bashible.ContextHosts{
					"example.com": {
						Mirrors: []bashible.ContextMirrorHost{
							{
								Host:   "example.com",
								Auth:   bashible.ContextAuth{Auth: "dXNlcjpwYXNz"},
								Scheme: "https",
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

			registryData, err := tt.input.toRegistryData()
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
