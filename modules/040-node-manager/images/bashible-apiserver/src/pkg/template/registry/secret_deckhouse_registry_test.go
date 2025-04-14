// Copyright 2024 Flant JSC
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
)

func TestDeckhouseRegistry_ConvertToRegistryData(t *testing.T) {
	tests := []struct {
		testName       string
		inputConfig    deckhouseRegistry
		expectedConfig RegistryData
	}{
		{
			testName: "With non-empty fields",
			inputConfig: deckhouseRegistry{
				Address:      "example.com",
				Path:         "/base",
				Scheme:       "https",
				CA:           "",
				DockerConfig: []byte(`{"auths":{"example.com":{"auth":"dXNlcjpwYXNz"}}}`),
			},
			expectedConfig: RegistryData{
				Mode:           "unmanaged",
				ImagesBase:     "example.com/base",
				Version:        "unknown",
				ProxyEndpoints: []string{},
				Hosts: []RegistryDataHostsObject{
					{
						Host: "example.com",
						CA:   []string{},
						Mirrors: []RegistryDataMirrorHostObject{
							{
								Host:   "example.com",
								Auth:   "dXNlcjpwYXNz",
								Scheme: "https",
							},
						},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host: "example.com",
						CA:   []string{},
						Mirrors: []RegistryDataMirrorHostObject{
							{
								Host:   "example.com",
								Auth:   "dXNlcjpwYXNz",
								Scheme: "https",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := tt.inputConfig.Validate()
			assert.NoError(t, err, "Expected no validation error")

			registryData, err := tt.inputConfig.ConvertToRegistryData()
			assert.NoError(t, err, "Expected no error in ToRegistryData")
			assert.Equal(t, tt.expectedConfig, *registryData, "RegistryData does not match expected")

			err = registryData.Validate()
			assert.NoError(t, err, "Expected no error in Validate")
		})
	}
}
