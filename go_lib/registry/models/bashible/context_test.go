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

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
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
		// --- the validation added with the fuzz harnesses ------------------
		//
		// Each of these values reaches a sink with no quoting of its own: a
		// proxy endpoint becomes `server <value>;` in the node balancer's NGINX
		// configuration, and a hosts key becomes a directory name under
		// /etc/containerd/registry.d. The rules that bound them live in
		// go_lib/registry/helpers; what these cases pin is that the model
		// applies them, and to which fields.
		{
			name: "Proxy endpoint as the module generates it",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"10.0.0.1:5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint as an IPv6 endpoint",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"[fd00::1]:5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint as the bootstrap placeholder",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{helpers.NodeIPPlaceholder + ":5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint without a port",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"10.0.0.1"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as a DNS name",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"registry.example.com:5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint ending the NGINX directive",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"10.0.0.1:5001; return 200"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint carrying a command substitution",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"$(id > /tmp/pwned):5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as the bare placeholder",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{helpers.NodeIPPlaceholder}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as a placeholder near miss",
			input: func() *Context {
				cfg := validContext()
				cfg.ProxyEndpoints = []string{"${discovered_node_ip:-$(id)}:5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror host as the bootstrap placeholder",
			input: func() *Context {
				cfg := validContext()
				mirror := validContextMirrorHost()
				mirror.Host = helpers.NodeIPPlaceholder + ":5001"
				cfg.Hosts = map[string]ContextHosts{"host1": {Mirrors: []ContextMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Mirror host as a bare IPv6 address",
			input: func() *Context {
				cfg := validContext()
				mirror := validContextMirrorHost()
				mirror.Host = "fd00::1"
				cfg.Hosts = map[string]ContextHosts{"host1": {Mirrors: []ContextMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Mirror host carrying a path separator",
			input: func() *Context {
				cfg := validContext()
				mirror := validContextMirrorHost()
				mirror.Host = "mirror1.example.com/path"
				cfg.Hosts = map[string]ContextHosts{"host1": {Mirrors: []ContextMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror host traversing out of registry.d",
			input: func() *Context {
				cfg := validContext()
				mirror := validContextMirrorHost()
				mirror.Host = "../../../etc/cron.d/x"
				cfg.Hosts = map[string]ContextHosts{"host1": {Mirrors: []ContextMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror scheme in the wrong case",
			input: func() *Context {
				cfg := validContext()
				mirror := validContextMirrorHost()
				mirror.Scheme = "HTTPS"
				cfg.Hosts = map[string]ContextHosts{"host1": {Mirrors: []ContextMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			// ozzo validates map values, not keys, so the model walks the keys
			// itself. Without that loop this value would reach mkdir -p.
			name: "Hosts key traversing out of registry.d",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{"../../../etc/cron.d": validContextHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key carrying a command substitution",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{"$(id)": validContextHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key that is empty",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{"": validContextHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key as the in-cluster address",
			input: func() *Context {
				cfg := validContext()
				cfg.Hosts = map[string]ContextHosts{"registry.d8-system.svc:5001": validContextHosts()}
				return cfg
			}(),
			wantErr: false,
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
					ProxyEndpoints:       []string{"192.168.1.1:5001"},
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
						"proxyEndpoints":       []any{"192.168.1.1:5001"},
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
		{
			name: "With Bootstrap",
			input: func() Context {
				ret := Context{
					Bootstrap: &ContextBootstrap{
						Init: initsecret.Config{
							CA: initsecret.CertKey{
								Cert: "---cert---",
								Key:  "---key---",
							},
							ROUser: initsecret.User{
								Name:         "ro_name",
								Password:     "ro_password",
								PasswordHash: "ro_password_hash",
							},
							RWUser: initsecret.User{
								Name:         "rw_name",
								Password:     "rw_password",
								PasswordHash: "rw_password_hash",
							},
						},
						Proxy: &ContextBootstrapProxy{
							Host:     "example.com",
							Path:     "/path",
							Scheme:   "https",
							Username: "user",
							Password: "pass",
							CA:       "---cert---",
							TTL:      "5m",
						},
					},
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
						"bootstrap": map[string]any{
							"init": map[string]any{
								"ca": map[string]any{
									"cert": "---cert---",
									"key":  "---key---",
								},
								"ro_user": map[string]any{
									"name":          "ro_name",
									"password":      "ro_password",
									"password_hash": "ro_password_hash",
								},
								"rw_user": map[string]any{
									"name":          "rw_name",
									"password":      "rw_password",
									"password_hash": "rw_password_hash",
								},
							},
							"proxy": map[string]any{
								"host":     "example.com",
								"path":     "/path",
								"scheme":   "https",
								"username": "user",
								"password": "pass",
								"ca":       "---cert---",
								"ttl":      "5m",
							},
						},
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
			require.Equal(t, tt.result.toMap, tt.input.ToMap())
		})
	}
}
