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

package config

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
)

func validRegistryData() RegistryData {
	return RegistryData{
		Address:   "r.example.com",
		Path:      "/deckhouse/ce",
		Scheme:    "https",
		CA:        "==exampleCA==",
		DockerCfg: "eyJhdXRocyI6eyJyLmV4YW1wbGUuY29tIjp7ImF1dGgiOiJZVHBpIn19fQ==",
	}
}

func TestRegistryDataProcess(t *testing.T) {
	type result struct {
		rData RegistryData
		err   bool
	}

	tests := []struct {
		name   string
		input  DeckhouseClusterConfig
		result result
	}{
		{
			name: "Valid registry data: with auth",
			input: func() DeckhouseClusterConfig {
				ret := DeckhouseClusterConfig{
					ImagesRepo: "r.example.com/deckhouse/ce",
					RegistryDockerCfg: base64.StdEncoding.EncodeToString([]byte(
						generateDockerCfg("r.example.com", "username", "password"),
					)),
					RegistryCA:     "==exampleCA==",
					RegistryScheme: "HTTPS",
				}
				return ret
			}(),
			result: result{
				rData: RegistryData{
					Address: "r.example.com",
					Path:    "/deckhouse/ce",
					Scheme:  "https",
					CA:      "==exampleCA==",
					DockerCfg: base64.StdEncoding.EncodeToString([]byte(
						generateDockerCfg("r.example.com", "username", "password"),
					)),
				},
				err: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rData := RegistryData{}
			err := rData.Process(tt.input)
			if tt.result.err {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
				require.Equal(t, tt.result.rData, rData)
			}
		})
	}
}

func TestRegistryDataAuth(t *testing.T) {
	type result struct {
		auth string
		err  bool
	}

	tests := []struct {
		name   string
		input  RegistryData
		result result
	}{
		{
			name: "Valid registry data: username + password",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg(ret.Address, "username", "password"),
				))
				return ret
			}(),
			result: result{
				auth: dockerCfgAuth("username", "password"),
				err:  false,
			},
		},
		{
			name: "Valid registry data: username + password",
			input: func() RegistryData {
				ret := validRegistryData()
				username := "username"
				password := "password"
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg(ret.Address, &username, &password),
				))
				return ret
			}(),
			result: result{
				auth: dockerCfgAuth("username", "password"),
				err:  false,
			},
		},
		{
			name: "Valid registry data: username + empty password",
			input: func() RegistryData {
				ret := validRegistryData()
				username := "username"
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg(ret.Address, &username, nil),
				))
				return ret
			}(),
			result: result{
				auth: "",
				err:  false,
			},
		},
		{
			name: "Valid registry data: empty username + password",
			input: func() RegistryData {
				ret := validRegistryData()
				password := "password"
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg(ret.Address, nil, &password),
				))
				return ret
			}(),
			result: result{
				auth: "",
				err:  false,
			},
		},
		{
			name: "Valid registry data: empty username + empty password",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateOldDockerCfg(ret.Address, nil, nil),
				))
				return ret
			}(),
			result: result{
				auth: "",
				err:  false,
			},
		},
		{
			name: "Invalid registry data: invalid dockerCfg",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.DockerCfg = "123"
				return ret
			}(),
			result: result{
				auth: "",
				err:  true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := tt.input.Auth()
			if tt.result.err {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
				require.Equal(t, tt.result.auth, auth)
			}
		})
	}
}

func TestRegistryDataToMap(t *testing.T) {
	type result struct {
		toMap map[string]interface{}
		err   bool
	}

	tests := []struct {
		name   string
		input  RegistryData
		result result
	}{
		{
			name: "Valid registry data: with auth",
			input: func() RegistryData {
				ret := RegistryData{
					Address: "r.example.com",
					Path:    "/deckhouse/ce",
					Scheme:  "https",
					CA:      "==exampleCA==",
				}
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("r.example.com", "username", "password"),
				))
				return ret
			}(),
			result: result{
				toMap: func() map[string]interface{} {
					ret := map[string]interface{}{
						"address": "r.example.com",
						"path":    "/deckhouse/ce",
						"scheme":  "https",
						"ca":      "==exampleCA==",
					}
					ret["dockerCfg"] = base64.StdEncoding.EncodeToString([]byte(
						generateDockerCfg("r.example.com", "username", "password"),
					))
					ret["auth"] = dockerCfgAuth("username", "password")
					return ret
				}(),
				err: false,
			},
		},
		{
			name: "Valid registry data: empty auth",
			input: func() RegistryData {
				ret := RegistryData{
					Address:   "r.example.com",
					Path:      "/deckhouse/ce",
					Scheme:    "https",
					CA:        "==exampleCA==",
					DockerCfg: "",
				}
				return ret
			}(),
			result: result{
				toMap: func() map[string]interface{} {
					ret := map[string]interface{}{
						"address":   "r.example.com",
						"path":      "/deckhouse/ce",
						"scheme":    "https",
						"ca":        "==exampleCA==",
						"dockerCfg": "",
					}
					return ret
				}(),
				err: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toMap, err := tt.input.toMap()
			if tt.result.err {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
				require.Equal(t, tt.result.toMap, toMap)
			}
		})
	}
}

func TestRegistryDataToBashibleCtx(t *testing.T) {
	type result struct {
		bashibleCtx *bashible.Context
		err         bool
	}

	tests := []struct {
		name   string
		input  RegistryData
		result result
	}{
		{
			name: "Valid registry data: with auth",
			input: func() RegistryData {
				ret := RegistryData{
					Address: "r.example.com",
					Path:    "/deckhouse/ce",
					Scheme:  "https",
					CA:      "==exampleCA==",
				}
				ret.DockerCfg = base64.StdEncoding.EncodeToString([]byte(
					generateDockerCfg("r.example.com", "username", "password"),
				))
				return ret
			}(),
			result: result{
				bashibleCtx: func() *bashible.Context {
					ret := bashible.Context{
						RegistryModuleEnable: false,
						Mode:                 "unmanaged",
						Version:              "unknown",
						ImagesBase:           "r.example.com/deckhouse/ce",
						ProxyEndpoints:       []string{},
						Hosts: map[string]bashible.ContextHosts{
							"r.example.com": {
								Mirrors: []bashible.ContextMirrorHost{{
									Host:   "r.example.com",
									Scheme: "https",
									CA:     "==exampleCA==",
									Auth: bashible.ContextAuth{
										Auth: dockerCfgAuth("username", "password")}},
								},
							},
						},
					}
					return &ret
				}(),
				err: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bashibleCtx, err := tt.input.toBashibleCtx()
			if tt.result.err {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
				require.Equal(t, tt.result.bashibleCtx, bashibleCtx)
			}
		})
	}
}

func TestValidateHTTPRegistryScheme(t *testing.T) {
	tests := []struct {
		name    string
		input   RegistryData
		wantErr string
	}{
		{
			name: "Valid registry data",
			input: func() RegistryData {
				ret := validRegistryData()
				return ret
			}(),
			wantErr: "",
		},
		{
			name: "Valid registry data: https + CA",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.Scheme = "https"
				ret.CA = "==exampleCA=="
				return ret
			}(),
			wantErr: "",
		},
		{
			name: "Valid registry data: https + empty CA",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.Scheme = "https"
				ret.CA = ""
				return ret
			}(),
			wantErr: "",
		},
		{
			name: "Valid registry data: http + empty CA",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.Scheme = "http"
				ret.CA = ""
				return ret
			}(),
			wantErr: "",
		},
		{
			name: "Invalid registry data: http + CA",
			input: func() RegistryData {
				ret := validRegistryData()
				ret.Scheme = "http"
				ret.CA = "==exampleCA=="
				return ret
			}(),
			wantErr: "registry CA is not allowed for HTTP scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHTTPRegistryScheme(tt.input.Scheme, tt.input.CA)
			if tt.wantErr != "" {
				assert.Error(t, err, "Expected errors but got none")
				require.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
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
