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

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
)

func TestRegistryData_FromInputData(t *testing.T) {
	tests := []struct {
		testName                  string
		inputDeckhouseRegistryCfg deckhouseRegistry
		inputRegistryBashibleCfg  *registryBashibleConfig
		expectedConfig            *RegistryData
		expectError               bool
	}{
		{
			testName: "Empty registry bashible config",
			inputDeckhouseRegistryCfg: deckhouseRegistry{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			inputRegistryBashibleCfg: nil,
			expectedConfig: &RegistryData{
				Mode:           "unmanaged",
				ImagesBase:     "registry-1.com/test",
				Version:        "unknown",
				ProxyEndpoints: []string{},
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "registry-1.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "registry-1.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "With registry bashible config",
			inputDeckhouseRegistryCfg: deckhouseRegistry{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			inputRegistryBashibleCfg: &registryBashibleConfig{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			expectedConfig: &RegistryData{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
					{
						Host:    "registry-1.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
					{
						Host:    "registry-1.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "With registry bashible config, unique hosts",
			inputDeckhouseRegistryCfg: deckhouseRegistry{
				Address: "registry-2.com",
				Path:    "/test",
				Scheme:  "https",
			},
			inputRegistryBashibleCfg: &registryBashibleConfig{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []registryBashibleConfigHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []registryBashibleConfigHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []registryBashibleConfigMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			expectedConfig: &RegistryData{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "registry-2.com",
						CA:      []string{},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			outputData := &RegistryData{}
			outputErr := outputData.FromInputData(tt.inputDeckhouseRegistryCfg, tt.inputRegistryBashibleCfg)

			if tt.expectError {
				assert.Error(t, outputErr, "Expected an error but got none")
			} else {
				assert.NoError(t, outputErr, "Expected no error but got one")
			}

			assert.Equal(t, tt.expectedConfig, outputData, "Expected and actual configurations do not match")
		})
	}
}

func TestRegistryData_Validate(t *testing.T) {
	tests := []struct {
		testName    string
		inputConfig RegistryData
		expectError bool
	}{
		{
			testName: "Valid config with full fields",
			inputConfig: RegistryData{
				Mode:           "managed",
				ImagesBase:     "example.com/base",
				Version:        "1.0",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "host1.example.com",
						CA:      []string{"ca-cert"},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "host1.example.com",
						CA:      []string{"ca-cert"},
						Mirrors: []RegistryDataMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Valid config with empty optional fields",
			inputConfig: RegistryData{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "host1.example.com",
						Mirrors: []RegistryDataMirrorHostObject{{Host: "mirror1.example.com", Scheme: "https"}},
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Missing required Mode",
			inputConfig: RegistryData{
				Mode:       "",
				ImagesBase: "example_base",
				Version:    "1.0",
			},
			expectError: true,
		},
		{
			testName: "Missing required ImagesBase",
			inputConfig: RegistryData{
				Mode:       "managed",
				ImagesBase: "",
				Version:    "1.0",
			},
			expectError: true,
		},
		{
			testName: "Missing required Version",
			inputConfig: RegistryData{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "",
			},
			expectError: true,
		},
		{
			testName: "Empty ProxyEndpoint is invalid",
			inputConfig: RegistryData{
				Mode:           "managed",
				ImagesBase:     "example_base",
				Version:        "1.0",
				ProxyEndpoints: []string{""},
			},
			expectError: true,
		},
		{
			testName: "Empty Host is invalid",
			inputConfig: RegistryData{
				Mode:         "managed",
				ImagesBase:   "example_base",
				Version:      "1.0",
				Hosts:        []RegistryDataHostsObject{{Host: ""}},
				PrepullHosts: []RegistryDataHostsObject{{Host: ""}},
			},
			expectError: true,
		},
		{
			testName: "Empty CA is invalid",
			inputConfig: RegistryData{
				Mode:         "managed",
				ImagesBase:   "example_base",
				Version:      "1.0",
				Hosts:        []RegistryDataHostsObject{{Host: "host", CA: []string{""}}},
				PrepullHosts: []RegistryDataHostsObject{{Host: "host", CA: []string{""}}},
			},
			expectError: true,
		},
		{
			testName: "Mirror with empty Host is invalid",
			inputConfig: RegistryData{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "host",
						Mirrors: []RegistryDataMirrorHostObject{{Host: "", Scheme: "https"}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "host",
						Mirrors: []RegistryDataMirrorHostObject{{Host: "", Scheme: "https"}},
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Mirror with empty Scheme is invalid",
			inputConfig: RegistryData{
				Mode:       "managed",
				ImagesBase: "example_base",
				Version:    "1.0",
				Hosts: []RegistryDataHostsObject{
					{
						Host:    "host",
						Mirrors: []RegistryDataMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
				PrepullHosts: []RegistryDataHostsObject{
					{
						Host:    "host",
						Mirrors: []RegistryDataMirrorHostObject{{Host: "host", Scheme: ""}},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := tt.inputConfig.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.expectError {
				assert.Error(t, err, "Expected validation errors but got none")
			} else {
				assert.NoError(t, err, "Expected no validation errors but got some")
			}
		})
	}
}
