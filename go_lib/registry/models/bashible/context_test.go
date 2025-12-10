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

package bashible

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validContext() *Context {
	return &Context{
		RegistryModuleEnable: false,
		Mode:                 "managed",
		ImagesBase:           "example.com/base",
		Version:              "1.0",
		Hosts: map[string]ContextHosts{
			"host1": validContextHosts(),
		},
	}
}

func validContextHosts() ContextHosts {
	return ContextHosts{
		Mirrors: []ContextMirrorHost{
			validContextMirrorHost(),
		},
	}
}

func validContextMirrorHost() ContextMirrorHost {
	return ContextMirrorHost{
		Host:     "mirror1.example.com",
		Scheme:   "https",
		Auth:     ContextAuth{},
		Rewrites: []ContextRewrite{},
	}
}

func TestContextValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *Context
		wantErr bool
	}{
		{
			name:    "Valid config",
			input:   validContext(),
			wantErr: false,
		},
		{
			name: "Missing required hosts",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required mirror hosts",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{"host1": {}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Mode",
			input: func() *Context {
				cfg := validContext()
				cfg.Mode = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required ImagesBase",
			input: func() *Context {
				cfg := validContext()
				cfg.ImagesBase = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Version",
			input: func() *Context {
				cfg := validContext()
				cfg.Version = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty ProxyEndpoint is invalid",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{""}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Host is invalid",
			input: func() *Context {
				cfg := validContext()
				host := validContextHosts()
				mirror := validContextMirrorHost()
				mirror.Host = ""
				host.Mirrors = []ContextMirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Scheme is invalid",
			input: func() *Context {
				cfg := validContext()
				host := validContextHosts()
				mirror := validContextMirrorHost()
				mirror.Scheme = ""
				host.Mirrors = []ContextMirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Duplicate Mirrors",
			input: func() *Context {
				cfg := validContext()
				host := validContextHosts()
				mirror := validContextMirrorHost()
				host.Mirrors = []ContextMirrorHost{mirror, mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
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

func TestContextToMap(t *testing.T) {
	type result struct {
		toMap map[string]any
		err   bool
	}

	tests := []struct {
		name   string
		input  Context
		result result
	}{
		{
			name: "Valid registry data: with all fields",
			input: func() Context {
				ret := Context{
					RegistryModuleEnable: true,
					Mode:                 "unmanaged",
					Version:              "unknown",
					ImagesBase:           "registry.d8-system.svc/deckhouse/system",
					ProxyEndpoints:       []string{"192.168.1.1"},
					Hosts: map[string]ContextHosts{
						"registry.d8-system.svc": {
							Mirrors: []ContextMirrorHost{{
								Host:   "r.example.com",
								Scheme: "https",
								CA:     "==exampleCA==",
								Auth: ContextAuth{
									Username: "user",
									Password: "password",
									Auth:     "auth"},
								Rewrites: []ContextRewrite{{
									From: "^deckhouse/system",
									To:   "deckhouse/ce"}}},
							},
						},
					},
				}
				return ret
			}(),
			result: result{
				toMap: func() map[string]any {

					ret := map[string]any{
						"registryModuleEnable": true,
						"mode":                 "unmanaged",
						"version":              "unknown",
						"imagesBase":           "registry.d8-system.svc/deckhouse/system",
						"proxyEndpoints":       []any{"192.168.1.1"},
						"hosts": map[string]any{
							"registry.d8-system.svc": map[string]any{
								"mirrors": []any{
									map[string]any{
										"host":   "r.example.com",
										"scheme": "https",
										"ca":     "==exampleCA==",
										"auth": map[string]any{
											"username": "user",
											"password": "password",
											"auth":     "auth",
										},
										"rewrites": []any{
											map[string]any{
												"from": "^deckhouse/system",
												"to":   "deckhouse/ce",
											},
										},
									},
								},
							},
						},
					}
					return ret
				}(),
				err: false,
			},
		},

		{
			name: "Valid registry data: without optional fields",
			input: func() Context {
				ret := Context{
					RegistryModuleEnable: true,
					Mode:                 "unmanaged",
					Version:              "unknown",
					ImagesBase:           "registry.d8-system.svc/deckhouse/system",
					ProxyEndpoints:       []string{},
					Hosts: map[string]ContextHosts{
						"registry.d8-system.svc": {
							Mirrors: []ContextMirrorHost{{
								Host:     "r.example.com",
								Scheme:   "http",
								Auth:     ContextAuth{},
								Rewrites: []ContextRewrite{}},
							},
						},
					},
				}
				return ret
			}(),
			result: result{
				toMap: func() map[string]any {

					ret := map[string]any{
						"registryModuleEnable": true,
						"mode":                 "unmanaged",
						"version":              "unknown",
						"imagesBase":           "registry.d8-system.svc/deckhouse/system",
						"proxyEndpoints":       []any{},
						"hosts": map[string]any{
							"registry.d8-system.svc": map[string]any{
								"mirrors": []any{
									map[string]any{
										"host":   "r.example.com",
										"scheme": "http",
										"ca":     "",
										"auth": map[string]any{
											"username": "",
											"password": "",
											"auth":     "",
										},
										"rewrites": []any{},
									},
								},
							},
						},
					}
					return ret
				}(),
				err: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toMap, err := tt.input.ToMap()
			if tt.result.err {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
				require.Equal(t, tt.result.toMap, toMap)
			}
		})
	}
}
