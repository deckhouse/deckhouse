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

func TestRegistryBashibleConfig_Validate(t *testing.T) {
	tests := []struct {
		testName    string
		inputConfig registryBashibleConfig
		expectError bool
	}{
		{
			testName: "Valid config with full fields",
			inputConfig: registryBashibleConfig{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host1.example.com",
						CA:      []string{"ca-cert"},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host1.example.com",
						CA:      []string{"ca-cert"},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Valid config with empty optional fields",
			inputConfig: registryBashibleConfig{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host1.example.com",
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Missing required Mode",
			inputConfig: registryBashibleConfig{
				Mode:       "",
				ImagesBase: "example_base",
				Version:    "1.0",
			},
			expectError: true,
		},
		{
			testName: "Missing required ImagesBase",
			inputConfig: registryBashibleConfig{
				Mode:       "managed",
				ImagesBase: "",
				Version:    "1.0",
			},
			expectError: true,
		},
		{
			testName: "Missing required Version",
			inputConfig: registryBashibleConfig{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "",
			},
			expectError: true,
		},
		{
			testName: "Empty ProxyEndpoint is invalid",
			inputConfig: registryBashibleConfig{
				Mode:           "managed",
				ImagesBase:     "example_base",
				Version:        "1.0",
				ProxyEndpoints: []string{""},
			},
			expectError: true,
		},
		{
			testName: "Empty Host is invalid",
			inputConfig: registryBashibleConfig{
				Mode:         "managed",
				ImagesBase:   "example_base",
				Version:      "1.0",
				Hosts:        []registryBashibleConfigHostsObject{{Host: ""}},
				PrepullHosts: []registryBashibleConfigHostsObject{{Host: ""}},
			},
			expectError: true,
		},
		{
			testName: "Empty CA is invalid",
			inputConfig: registryBashibleConfig{
				Mode:         "managed",
				ImagesBase:   "example_base",
				Version:      "1.0",
				Hosts:        []registryBashibleConfigHostsObject{{Host: "host", CA: []string{""}}},
				PrepullHosts: []registryBashibleConfigHostsObject{{Host: "host", CA: []string{""}}},
			},
			expectError: true,
		},
		{
			testName: "Mirror with empty Host is invalid",
			inputConfig: registryBashibleConfig{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host",
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host",
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Mirror with empty Scheme is invalid",
			inputConfig: registryBashibleConfig{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host",
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "host",
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := tt.inputConfig.Validate()

			if tt.expectError {
				assert.Error(t, err, "Expected validation errors but got none")
			} else {
				assert.NoError(t, err, "Expected no validation errors but got some")
			}
		})
	}
}

func TestRegistryBashibleConfig_ConvertToRegistryData(t *testing.T) {
	tests := []struct {
		testName       string
		inputConfig    registryBashibleConfig
		expectedConfig RegistryData
	}{
		{
			testName: "With non-empty fields",
			inputConfig: registryBashibleConfig{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host: "host1.example.com",
						CA:   []string{"ca1"},
						Mirrors: []registryBashibleConfigMirrorHostObject{
							{
								Host:     "mirror1.example.com",
								Username: "username",
								Password: "password",
								Auth:     "auth",
								Scheme:   "https",
							},
						},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host: "host1.example.com",
						CA:   []string{"ca1"},
						Mirrors: []registryBashibleConfigMirrorHostObject{
							{
								Host:     "mirror1.example.com",
								Username: "username",
								Password: "password",
								Auth:     "auth",
								Scheme:   "https",
							},
						},
					},
				},
			},

			expectedConfig: RegistryData{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []RegistryDataHostsObject{
					{
						Host: "host1.example.com",
						CA:   []string{"ca1"},
						Mirrors: []RegistryDataMirrorHostObject{
							{
								Host:     "mirror1.example.com",
								Username: "username",
								Password: "password",
								Auth:     "auth",
								Scheme:   "https",
							},
						},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host: "host1.example.com",
						CA:   []string{"ca1"},
						Mirrors: []RegistryDataMirrorHostObject{
							{
								Host:     "mirror1.example.com",
								Username: "username",
								Password: "password",
								Auth:     "auth",
								Scheme:   "https",
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

			registryData := tt.inputConfig.ConvertToRegistryData()
			assert.NoError(t, err, "Expected no error in ToRegistryData")
			assert.Equal(t, tt.expectedConfig, *registryData, "RegistryData does not match expected")

			err = registryData.Validate()
			assert.NoError(t, err, "Expected no error in Validate")
		})
	}
}
