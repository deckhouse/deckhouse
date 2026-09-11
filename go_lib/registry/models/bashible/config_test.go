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
)

func validConfig() *Config {
	return &Config{
		Mode:       "managed",
		ImagesBase: "example.com/base",
		Version:    "1.0",
		Hosts: map[string]ConfigHosts{
			"host1": validConfigHosts(),
		},
	}
}

func validConfigHosts() ConfigHosts {
	return ConfigHosts{
		Mirrors: []ConfigMirrorHost{
			validConfigMirrorHost(),
		},
	}
}

func validConfigMirrorHost() ConfigMirrorHost {
	return ConfigMirrorHost{
		Host:     "mirror1.example.com",
		Scheme:   "https",
		Auth:     ConfigAuth{},
		Rewrites: []ConfigRewrite{},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *Config
		wantErr bool
	}{
		{
			name:    "Valid config",
			input:   validConfig(),
			wantErr: false,
		},
		{
			name: "Missing required hosts",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required mirror hosts",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{"host1": {}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Mode",
			input: func() *Config {
				cfg := validConfig()
				cfg.Mode = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required ImagesBase",
			input: func() *Config {
				cfg := validConfig()
				cfg.ImagesBase = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Missing required Version",
			input: func() *Config {
				cfg := validConfig()
				cfg.Version = ""
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Empty ProxyEndpoint is invalid",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{""}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Host is invalid",
			input: func() *Config {
				cfg := validConfig()
				host := validConfigHosts()
				mirror := validConfigMirrorHost()
				mirror.Host = ""
				host.Mirrors = []ConfigMirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror with empty Scheme is invalid",
			input: func() *Config {
				cfg := validConfig()
				host := validConfigHosts()
				mirror := validConfigMirrorHost()
				mirror.Scheme = ""
				host.Mirrors = []ConfigMirrorHost{mirror}
				cfg.Hosts["host1"] = host
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Duplicate Mirrors",
			input: func() *Config {
				cfg := validConfig()
				host := validConfigHosts()
				mirror := validConfigMirrorHost()
				host.Mirrors = []ConfigMirrorHost{mirror, mirror}
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
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"10.0.0.1:5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint as an IPv6 endpoint",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"[fd00::1]:5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint as the bootstrap placeholder",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{helpers.NodeIPPlaceholder + ":5001"}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Proxy endpoint without a port",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"10.0.0.1"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as a DNS name",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"registry.example.com:5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint ending the NGINX directive",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"10.0.0.1:5001; return 200"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint carrying a command substitution",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"$(id > /tmp/pwned):5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as the bare placeholder",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{helpers.NodeIPPlaceholder}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Proxy endpoint as a placeholder near miss",
			input: func() *Config {
				cfg := validConfig()
				cfg.ProxyEndpoints = []string{"${discovered_node_ip:-$(id)}:5001"}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror host as the bootstrap placeholder",
			input: func() *Config {
				cfg := validConfig()
				mirror := validConfigMirrorHost()
				mirror.Host = helpers.NodeIPPlaceholder + ":5001"
				cfg.Hosts = map[string]ConfigHosts{"host1": {Mirrors: []ConfigMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Mirror host as a bare IPv6 address",
			input: func() *Config {
				cfg := validConfig()
				mirror := validConfigMirrorHost()
				mirror.Host = "fd00::1"
				cfg.Hosts = map[string]ConfigHosts{"host1": {Mirrors: []ConfigMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "Mirror host carrying a path separator",
			input: func() *Config {
				cfg := validConfig()
				mirror := validConfigMirrorHost()
				mirror.Host = "mirror1.example.com/path"
				cfg.Hosts = map[string]ConfigHosts{"host1": {Mirrors: []ConfigMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror host traversing out of registry.d",
			input: func() *Config {
				cfg := validConfig()
				mirror := validConfigMirrorHost()
				mirror.Host = "../../../etc/cron.d/x"
				cfg.Hosts = map[string]ConfigHosts{"host1": {Mirrors: []ConfigMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Mirror scheme in the wrong case",
			input: func() *Config {
				cfg := validConfig()
				mirror := validConfigMirrorHost()
				mirror.Scheme = "HTTPS"
				cfg.Hosts = map[string]ConfigHosts{"host1": {Mirrors: []ConfigMirrorHost{mirror}}}
				return cfg
			}(),
			wantErr: true,
		},
		{
			// ozzo validates map values, not keys, so the model walks the keys
			// itself. Without that loop this value would reach mkdir -p.
			name: "Hosts key traversing out of registry.d",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{"../../../etc/cron.d": validConfigHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key carrying a command substitution",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{"$(id)": validConfigHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key that is empty",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{"": validConfigHosts()}
				return cfg
			}(),
			wantErr: true,
		},
		{
			name: "Hosts key as the in-cluster address",
			input: func() *Config {
				cfg := validConfig()
				cfg.Hosts = map[string]ConfigHosts{"registry.d8-system.svc:5001": validConfigHosts()}
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

func TestConfigToContext(t *testing.T) {
	type result struct {
		toMap map[string]any
		err   bool
	}

	tests := []struct {
		name   string
		input  Config
		result Context
	}{
		{
			name: "with all fields",
			input: Config{
				Mode:           "unmanaged",
				Version:        "unknown",
				ImagesBase:     "registry.d8-system.svc/deckhouse/system",
				ProxyEndpoints: []string{"192.168.1.1:5001"},
				Hosts: map[string]ConfigHosts{
					"registry.d8-system.svc": {
						Mirrors: []ConfigMirrorHost{
							{
								Host:   "r.example.com",
								Scheme: "https",
								CA:     "==exampleCA==",
								Auth: ConfigAuth{
									Username: "user",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []ConfigRewrite{{
									From: "^deckhouse/system",
									To:   "deckhouse/ce",
								}},
							},
						},
					},
				},
			},
			result: Context{
				RegistryModuleEnable: false,
				Mode:                 "unmanaged",
				Version:              "unknown",
				ImagesBase:           "registry.d8-system.svc/deckhouse/system",
				ProxyEndpoints:       []string{"192.168.1.1:5001"},
				Hosts: map[string]ContextHosts{
					"registry.d8-system.svc": {
						Mirrors: []ContextMirrorHost{
							{
								Host:   "r.example.com",
								Scheme: "https",
								CA:     "==exampleCA==",
								Auth: ContextAuth{
									Username: "user",
									Password: "password",
									Auth:     "auth",
								},
								Rewrites: []ContextRewrite{{
									From: "^deckhouse/system",
									To:   "deckhouse/ce",
								}},
							},
						},
					},
				},
			},
		},
		{
			name: "without optional fields",
			input: Config{
				Mode:           "unmanaged",
				Version:        "unknown",
				ImagesBase:     "registry.d8-system.svc/deckhouse/system",
				ProxyEndpoints: nil,
				Hosts: map[string]ConfigHosts{
					"registry.d8-system.svc": {
						Mirrors: []ConfigMirrorHost{
							{
								Host:     "r.example.com",
								Scheme:   "http",
								Auth:     ConfigAuth{},
								Rewrites: nil,
							},
						},
					},
				},
			},
			result: Context{
				RegistryModuleEnable: false,
				Mode:                 "unmanaged",
				Version:              "unknown",
				ImagesBase:           "registry.d8-system.svc/deckhouse/system",
				ProxyEndpoints:       nil,
				Hosts: map[string]ContextHosts{
					"registry.d8-system.svc": {
						Mirrors: []ContextMirrorHost{
							{
								Host:     "r.example.com",
								Scheme:   "http",
								Auth:     ContextAuth{},
								Rewrites: nil,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			require.NoError(t, err)

			ctx := tt.input.ToContext()
			require.Equal(t, tt.result, ctx)

			err = ctx.Validate()
			require.NoError(t, err)
		})
	}
}
