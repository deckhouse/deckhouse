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
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

func TestData_FromRegistrySettings(t *testing.T) {
	tests := []struct {
		name   string
		input  module_config.RegistrySettings
		output Data
	}{
		{
			name: "Without username and password",
			input: module_config.RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.SchemeHTTPS,
				CA:         "ca-cert",
				Username:   "user",
				Password:   "pass",
				License:    "",
			},
			output: Data{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.SchemeHTTPS,
				CA:         "ca-cert",
				Username:   "user",
				Password:   "pass",
			},
		},
		{
			name: "With license",
			input: module_config.RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.SchemeHTTPS,
				CA:         "ca-cert",
				Username:   "user",
				Password:   "pass",
				License:    "license-key",
			},
			output: Data{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.SchemeHTTPS,
				CA:         "ca-cert",
				Username:   constant.LicenseUsername,
				Password:   "license-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Data{}
			d.FromRegistrySettings(tt.input)
			assert.Equal(t, tt.output, d)
		})
	}
}

func TestData_AuthBase64(t *testing.T) {
	tests := []struct {
		name   string
		input  Data
		output string
	}{
		{
			name: "with username and password",
			input: Data{
				Username: "user",
				Password: "pass",
			},
			output: base64.StdEncoding.EncodeToString([]byte("user:pass")),
		},
		{
			name: "empty username returns empty string",
			input: Data{
				Username: "",
				Password: "pass",
			},
			output: "",
		},
		{
			name: "empty password with username",
			input: Data{
				Username: "user",
				Password: "",
			},
			output: base64.StdEncoding.EncodeToString([]byte("user:")),
		},
		{
			name: "empty",
			input: Data{
				Username: "",
				Password: "",
			},
			output: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.AuthBase64()
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestData_DockerCfg(t *testing.T) {
	type output = struct {
		auths map[string]any
		err   bool
	}

	tests := []struct {
		name   string
		input  Data
		output output
	}{
		{
			name: "valid credentials",
			input: Data{
				ImagesRepo: "registry.example.com",
				Username:   "user",
				Password:   "pass",
			},
			output: output{
				auths: map[string]any{
					"registry.example.com": map[string]any{
						"username": "user",
						"password": "pass",
						"auth":     "dXNlcjpwYXNz", // "user:pass"
					},
				},
				err: false,
			},
		},
		{
			name: "empty credentials",
			input: Data{
				ImagesRepo: "registry.example.com",
				Username:   "",
				Password:   "",
			},
			output: output{
				auths: map[string]any{
					"registry.example.com": map[string]any{
						"auth": "Og==", // ":"
					},
				},
				err: false,
			},
		},
		{
			name: "registry with path - should use hostname only",
			input: Data{
				ImagesRepo: "registry.example.com/path",
				Username:   "user",
				Password:   "pass",
			},
			output: output{
				auths: map[string]any{
					"registry.example.com": map[string]any{
						"username": "user",
						"password": "pass",
						"auth":     "dXNlcjpwYXNz", // "user:pass"
					},
				},
				err: false,
			},
		},
		{
			name: "registry with port and path - should use hostname with port",
			input: Data{
				ImagesRepo: "registry.example.com:8080/path",
				Username:   "user",
				Password:   "pass",
			},
			output: output{
				auths: map[string]any{
					"registry.example.com:8080": map[string]any{
						"username": "user",
						"password": "pass",
						"auth":     "dXNlcjpwYXNz", // "user:pass"
					},
				},
				err: false,
			},
		},
		{
			name: "localhost registry",
			input: Data{
				ImagesRepo: "localhost:5000",
				Username:   "admin",
				Password:   "secret",
			},
			output: output{
				auths: map[string]any{
					"localhost:5000": map[string]any{
						"username": "admin",
						"password": "secret",
						"auth":     "YWRtaW46c2VjcmV0", // "admin:secret"
					},
				},
				err: false,
			},
		},
		{
			name: "empty registry - should error",
			input: Data{
				ImagesRepo: "",
				Username:   "user",
				Password:   "pass",
			},
			output: output{
				auths: nil,
				err:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// get dockerCfg
			dockerCfg, err := tt.input.DockerCfg()
			if tt.output.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// get dockerCfg base64
			dockerCfgBase64, err := tt.input.DockerCfgBase64()
			if tt.output.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Compare dockerCfg from DockerCfg and DockerCfgBase64
			decoded, err := base64.StdEncoding.DecodeString(string(dockerCfgBase64))
			assert.NoError(t, err)
			assert.Equal(t, dockerCfg, decoded)

			// Compare with test case
			var dockerCfgJson map[string]any
			err = json.Unmarshal(dockerCfg, &dockerCfgJson)
			assert.NoError(t, err)
			assert.Equal(t, tt.output.auths, dockerCfgJson["auths"])
		})
	}
}

func TestData_AddressAndPath(t *testing.T) {
	type output = struct {
		address string
		path    string
	}

	tests := []struct {
		name   string
		input  Data
		output output
	}{
		{
			name: "registry without path",
			input: Data{
				ImagesRepo: "registry.example.com",
			},
			output: output{
				address: "registry.example.com",
				path:    "",
			},
		},
		{
			name: "registry with path",
			input: Data{
				ImagesRepo: "registry.example.com/path/to/repo",
			},
			output: output{
				address: "registry.example.com",
				path:    "/path/to/repo",
			},
		},
		{
			name: "registry with trailing slash",
			input: Data{
				ImagesRepo: "registry.example.com/",
			},
			output: output{
				address: "registry.example.com",
				path:    "",
			},
		},
		{
			name: "empty images repo",
			input: Data{
				ImagesRepo: "",
			},
			output: output{
				address: "",
				path:    "",
			},
		},
		{
			name: "registry with port and path",
			input: Data{
				ImagesRepo: "registry.example.com:5000/path",
			},
			output: output{
				address: "registry.example.com:5000",
				path:    "/path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, path := tt.input.AddressAndPath()
			assert.Equal(t, tt.output.address, addr)
			assert.Equal(t, tt.output.path, path)
		})
	}
}
