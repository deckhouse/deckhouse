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

package initconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

func generateDockerCfg(host, username, password string) string {
	auth := fmt.Sprintf("%s:%s", username, password)
	authBase64 := base64.StdEncoding.EncodeToString([]byte(auth))
	return fmt.Sprintf(`{"auths":{"%s":{"auth":"%s"}}}`, host, authBase64)
}

// generateOldDockerCfg generates a Docker config in the legacy format.
// If username or password are empty strings, they will be omitted from the output
// due to the `omitempty` JSON tag, resulting in the auth field not being populated.
func generateOldDockerCfg(host, username, password string) string {
	type authConfig struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	}

	type dockerConfig struct {
		Auths map[string]authConfig `json:"auths"`
	}

	cfg := dockerConfig{
		Auths: make(map[string]authConfig),
	}

	cfg.Auths[host] = authConfig{
		Username: username,
		Password: password,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func TestConfig_ApplySettings(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name: "default ImagesRepo",
			input: Config{
				ImagesRepo:     "",
				RegistryScheme: "HTTPS",
			},
			expected: Config{
				ImagesRepo:     constant.DefaultImagesRepo,
				RegistryScheme: "HTTPS",
			},
		},
		{
			name: "default Scheme",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "",
			},
			expected: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: string(constant.DefaultScheme),
			},
		},
		{
			name:  "default ImagesRepo and Scheme",
			input: Config{},
			expected: Config{
				ImagesRepo:     constant.DefaultImagesRepo,
				RegistryScheme: string(constant.DefaultScheme),
			},
		},
		{
			name: "trim ImagesRepo",
			input: Config{
				ImagesRepo:     "registry.example.com/",
				RegistryScheme: "HTTPS",
			},
			expected: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
			},
		},
		{
			name: "full",
			input: Config{
				ImagesRepo:        "registry.example.com",
				RegistryScheme:    "HTTP",
				RegistryDockerCfg: "<dockerCfg>",
				RegistryCA:        "<ca>",
			},
			expected: Config{
				ImagesRepo:        "registry.example.com",
				RegistryScheme:    "HTTP",
				RegistryDockerCfg: "<dockerCfg>",
				RegistryCA:        "<ca>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			config.ApplyConfig(tt.input)

			require.Equal(t, tt.expected, config)
		})
	}
}

func TestConfig_ToRegistrySettings(t *testing.T) {
	type output struct {
		err  bool
		want module_config.RegistrySettings
	}

	tests := []struct {
		name   string
		input  Config
		output output
	}{
		{
			name: "with all fields",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "-----BEGIN CERTIFICATE-----",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.SchemeHTTPS,
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with HTTP scheme",
			input: Config{
				ImagesRepo:        "registry.example.com:8080/path",
				RegistryScheme:    "HTTP",
				RegistryCA:        "",
				RegistryDockerCfg: "",
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com:8080/path",
					Scheme:     constant.SchemeHTTP,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with empty scheme",
			input: Config{
				ImagesRepo:        "registry.example.com:8080/path",
				RegistryScheme:    "",
				RegistryCA:        "",
				RegistryDockerCfg: "",
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com:8080/path",
					Scheme:     constant.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with docker config - new format",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "-----BEGIN CERTIFICATE-----",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.SchemeHTTPS,
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with docker config - old format with username and password",
			input: Config{
				ImagesRepo:     "registry.example.com:5000/path",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com:5000", "test-user", "test-password"),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com:5000/path",
					Scheme:     constant.SchemeHTTPS,
					CA:         "",
					Username:   "test-user",
					Password:   "test-password",
				},
			},
		},
		{
			name: "with old docker config - username only",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", "user", ""),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.SchemeHTTPS,
					CA:         "",
					Username:   "user",
					Password:   "",
				},
			},
		},
		{
			name: "with old docker config - password only",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", "", "pass"),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "pass",
				},
			},
		},
		{
			name: "with old docker config - empty credentials",
			input: Config{
				ImagesRepo:     "registry.example.com",
				RegistryScheme: "HTTPS",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg("registry.example.com", "", ""),
				)),
			},
			output: output{
				err: false,
				want: module_config.RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.SchemeHTTPS,
					CA:         "",
					Username:   "",
					Password:   "",
				},
			},
		},
		{
			name: "with invalid base64 docker config",
			input: Config{
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
			input: Config{
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
			input: Config{
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
			input: Config{
				ImagesRepo:     "",
				RegistryScheme: "",
				RegistryCA:     "",
				RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("registry.example.com", "user", "pass"),
				)),
			},
			output: output{
				err: true,
				want: module_config.RegistrySettings{
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
