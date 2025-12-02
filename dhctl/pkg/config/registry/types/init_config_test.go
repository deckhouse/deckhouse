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

package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func stringPtr(s string) *string {
	return &s
}

func dockerCfgAuth(username, password string) string {
	auth := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func generateDockerCfg(host, username, password string) string {
	return fmt.Sprintf(`{"auths":{"%s":{"auth":"%s"}}}`, host, dockerCfgAuth(username, password))
}

func generateOldDockerCfg(host string, username, password *string) string {
	res := map[string]interface{}{
		"auths": map[string]interface{}{
			host: make(map[string]interface{}),
		},
	}

	if username != nil {
		err := unstructured.SetNestedField(res, *username, "auths", host, "username")
		if err != nil {
			panic(err)
		}
	}

	if password != nil {
		err := unstructured.SetNestedField(res, *password, "auths", host, "password")
		if err != nil {
			panic(err)
		}
	}

	auth, err := json.Marshal(res)
	if err != nil {
		panic(err)
	}

	return string(auth)
}

func TestInitConfig_ToRegistrySettings(t *testing.T) {
	type output struct {
		err  bool
		want RegistrySettings
	}

	tests := []struct {
		name   string
		input  InitConfig
		output output
	}{
		{
			name: "with all fields",
			input: InitConfig{
				ImagesRepo:     "registry.example.com/",
				RegistryScheme: "HTTPS",
				RegistryCA:     "-----BEGIN CERTIFICATE-----",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with HTTP scheme",
			input: InitConfig{
				ImagesRepo:        "registry.example.com:8080/path",
				RegistryScheme:    "HTTP",
				RegistryCA:        "",
				RegistryDockerCfg: "",
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com:8080/path",
					Scheme:     registry_const.SchemeHTTP,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with empty scheme",
			input: InitConfig{
				ImagesRepo:        "registry.example.com:8080/path",
				RegistryScheme:    "",
				RegistryCA:        "",
				RegistryDockerCfg: "",
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com:8080/path",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with docker config - new format",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "-----BEGIN CERTIFICATE-----",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with docker config - old format with username and password",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", stringPtr("test-user"), stringPtr("test-password")),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with docker config - different registry address",
			input: InitConfig{
				ImagesRepo:     "registry.example.com:5000/path",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com:5000", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com:5000/path",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with old docker config - username only",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", stringPtr("user"), nil),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "user",
					Password:   "",
				},
			},
		},
		{
			name: "with old docker config - password only",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", nil, stringPtr("pass")),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "pass",
				},
			},
		},
		{
			name: "with old docker config - empty credentials",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", nil, nil),
				)),
			},
			output: output{
				err: false,
				want: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     registry_const.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with invalid base64 docker config",
			input: InitConfig{
				ImagesRepo:        "registry.example.com",
				RegistryScheme:    "HTTPS",
				RegistryCA:        "",
				RegistryDockerCfg: "invalid-base64!!!",
			},
			output: output{
				err: true,
			},
		},
		{
			name: "with invalid json in docker config",
			input: InitConfig{
				ImagesRepo:        "registry.example.com",
				RegistryScheme:    "HTTPS",
				RegistryCA:        "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte("invalid-json")),
			},
			output: output{
				err: true,
			},
		},
		{
			name: "with missing registry address in docker config",
			input: InitConfig{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("other-registry.com", "user", "pass"),
				)),
			},
			output: output{
				err: true,
			},
		},
		{
			name: "with empty images repo",
			input: InitConfig{
				ImagesRepo:     "",
				RegistryScheme: "",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "user", "pass"),
				)),
			},
			output: output{
				err: true,
				want: RegistrySettings{
					ImagesRepo: "",
					Scheme:     "",
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.input.ToRegistrySettings()

			if tt.output.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.output.want, result)
			}
		})
	}
}

func TestValidateRegistryDockerCfg(t *testing.T) {
	t.Run("Expect successful validation", func(t *testing.T) {
		creds := map[string]string{
			"registry.deckhouse.io":                `{"auths": { "registry.deckhouse.io": {}}}`,
			"regi-stry.deckhouse.io":               `{"auths": { "regi-stry.deckhouse.io": {}}}`,
			"registry.io":                          `{"auths": { "registry.io": {}}}`,
			"1.io":                                 `{"auths": { "1.io": {}}}`,
			"1.s.io":                               `{"auths": { "1.s.io": {}}}`,
			"regi.stry:5000":                       `{"auths": { "regi.stry:5000": {}}}`,
			"1.2.3":                                `{"auths": { "1.2.3": {}}}`,
			"1.2:5000":                             `{"auths": { "1.2:5000": {}}}`,
			"reg.dec.io1":                          `{"auths": { "reg.dec.io1": {}}}`,
			"one.two.three.four.five.six.whatever": `{"auths": { "one.two.three.four.five.six.whatever": {}}}`,
			"1.2.3.4.5.6.0":                        `{"auths": { "1.2.3.4.5.6.0": {}}}`,
		}

		for host, cred := range creds {
			dockerCfg := base64.StdEncoding.EncodeToString([]byte(cred))

			err := validateRegistryDockerCfg(dockerCfg, host)
			require.NoError(t, err)
		}
	})

	t.Run("Expect failed validation", func(t *testing.T) {
		hosts := []string{
			"some-bad-host:1434/deckhouse",
			"some-bad/deckhouse",
			".some-bad/deckhouse",
			"-some.bad",
			"somebad.",
			"some--ba",
			"some..ba",
			"14214.ba1::1554",
			"some.bad:host",
			"some-bad:host1",
		}

		for _, host := range hosts {
			creds := fmt.Sprintf("{\"auths\": { \"%s\": {}}}", host)
			dockerCfg := base64.StdEncoding.EncodeToString([]byte(creds))

			err := validateRegistryDockerCfg(dockerCfg, host)
			require.EqualErrorf(t,
				err,
				fmt.Sprintf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", host),
				err.Error())
		}
	})
}
