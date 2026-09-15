/*
Copyright 2026 Flant JSC

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

package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func newConfig(spec registryv1alpha1.RegistryConfigSpec) *registryv1alpha1.RegistryConfig {
	return &registryv1alpha1.RegistryConfig{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
		Spec:       spec,
	}
}

func upstream() *registryv1alpha1.Upstream {
	return &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   "registry.deckhouse.io",
			Path:   "/deckhouse/ee",
		},
	}
}

func TestValidateAcceptsEveryValidLayout(t *testing.T) {
	source := &registryv1alpha1.StorageSource{BundleRef: "d8-mirror-bundle", ExpectedDigests: 459}

	tests := []struct {
		name string
		spec registryv1alpha1.RegistryConfigSpec
	}{
		{
			name: "cache off with upstream",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
			},
		},
		{
			name: "pass-through cache",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
				Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi"},
			},
		},
		{
			name: "air-gap with an expected set",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50Gi", Source: source},
			},
		},
		{
			name: "Unmanaged owns nothing, so it needs nothing",
			spec: registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeUnmanaged},
		},
		{
			name: "an insecure upstream inside a trusted perimeter",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "registry.local:5000"},
				}},
			},
		},
		{
			name: "an IPv6 upstream with a port",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "[2001:db8::1]:5000"},
				}},
			},
		},
		{
			name: "distinct HA mirrors with their own credentials",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
					Mirrors: []registryv1alpha1.Endpoint{
						{Host: "mirror-1.example.com", Path: "/deckhouse/ee", Auth: &registryv1alpha1.Auth{Username: "u", Password: "p"}},
						{Host: "mirror-2.example.com", Path: "/deckhouse/ee"},
					},
				}},
				Storage: registryv1alpha1.StorageConfig{Cache: true},
			},
		},
		{
			name: "pre-encoded credentials",
			spec: registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{
						Host: "registry.deckhouse.io",
						Auth: &registryv1alpha1.Auth{Auth: "dXNlcjpwYXNz"},
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, Validate(newConfig(tt.spec)).ToAggregate())
		})
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *registryv1alpha1.RegistryConfig
		wantErr string
	}{
		{
			name: "a non-singleton object, rather than ignoring it silently",
			cfg: &registryv1alpha1.RegistryConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "my-registry"},
				Spec: registryv1alpha1.RegistryConfigSpec{
					Mode:    registryv1alpha1.ModeManaged,
					Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
				},
			},
			wantErr: "only the singleton object",
		},
		{
			name:    "no upstream and no cache leaves nothing to pull from",
			cfg:     newConfig(registryv1alpha1.RegistryConfigSpec{Mode: registryv1alpha1.ModeManaged}),
			wantErr: "needs a source of images",
		},
		{
			name: "air-gap without an expected set",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Storage: registryv1alpha1.StorageConfig{Cache: true},
			}),
			wantErr: "spec.storage.source",
		},
		{
			name: "a URL pasted into the host field",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "https://registry.deckhouse.io"},
				}},
			}),
			wantErr: "not a URL",
		},
		{
			name: "a full image reference pasted into the host field",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io/deckhouse/ee"},
				}},
			}),
			wantErr: "must not contain a path",
		},
		{
			name: "an out-of-range port",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io:99999"},
				}},
			}),
			wantErr: "port must be a number",
		},
		{
			name: "a non-numeric port",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io:https"},
				}},
			}),
			wantErr: "port must be a number",
		},
		{
			name: "an empty domain label",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry..deckhouse.io"},
				}},
			}),
			wantErr: "empty domain label",
		},
		{
			name: "a URL pasted into the path field",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "https://x/y"},
				}},
			}),
			wantErr: "not a URL",
		},
		{
			name: "a mirror duplicating the primary endpoint",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/ee"},
					Mirrors: []registryv1alpha1.Endpoint{
						{Host: "registry.deckhouse.io", Path: "/ee"},
					},
				}},
			}),
			wantErr: "same endpoint as the primary upstream",
		},
		{
			name: "two mirrors differing only by credentials point at the same content",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"},
					Mirrors: []registryv1alpha1.Endpoint{
						{Host: "mirror.example.com", Auth: &registryv1alpha1.Auth{Username: "one", Password: "p"}},
						{Host: "mirror.example.com", Auth: &registryv1alpha1.Auth{Username: "two", Password: "p"}},
					},
				}},
			}),
			wantErr: "same endpoint as mirrors[0]",
		},
		{
			name: "a password without a username fails at pull time with an opaque 401",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{
						Host: "registry.deckhouse.io",
						Auth: &registryv1alpha1.Auth{Password: "p"},
					},
				}},
			}),
			wantErr: "spec.primary.upstream.auth.username",
		},
		{
			name: "credentials that are not base64",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{
						Host: "registry.deckhouse.io",
						Auth: &registryv1alpha1.Auth{Auth: "not base64!"},
					},
				}},
			}),
			wantErr: "must be base64-encoded",
		},
		{
			name: "base64 credentials without a separator",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{
						Host: "registry.deckhouse.io",
						// Valid base64 that decodes to a value carrying no ":", which is
						// exactly what this case has to be rejected for. Encoded rather
						// than spelled out, so the literal cannot read as a credential.
						Auth: &registryv1alpha1.Auth{
							Auth: base64.StdEncoding.EncodeToString([]byte("username")),
						},
					},
				}},
			}),
			wantErr: "without a \":\" separator",
		},
		{
			name: "a certificate authority together with plain HTTP",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode: registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
					Endpoint: registryv1alpha1.Endpoint{
						Scheme: registryv1alpha1.SchemeHTTP,
						Host:   "registry.local:5000",
						CA:     "-----BEGIN CERTIFICATE-----",
					},
				}},
			}),
			wantErr: "meaningless with scheme HTTP",
		},
		{
			name: "an unparseable cache size",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
				Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "50 gigabytes"},
			}),
			wantErr: "is not a valid quantity",
		},
		{
			name: "a zero cache size",
			cfg: newConfig(registryv1alpha1.RegistryConfigSpec{
				Mode:    registryv1alpha1.ModeManaged,
				Primary: registryv1alpha1.PrimarySource{Upstream: upstream()},
				Storage: registryv1alpha1.StorageConfig{Cache: true, Size: "0"},
			}),
			wantErr: "must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.cfg)
			require.NotEmpty(t, errs, "spec should be rejected")
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}

// TestValidateDoesNotLeakCredentials guards against the validator putting a
// secret into a status message, which is world-readable on the object.
func TestValidateDoesNotLeakCredentials(t *testing.T) {
	const password = "super-secret-license-token"

	cfg := newConfig(registryv1alpha1.RegistryConfigSpec{
		Mode: registryv1alpha1.ModeManaged,
		Primary: registryv1alpha1.PrimarySource{Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP,
				Host:   "https://bad-host",
				CA:     "-----BEGIN CERTIFICATE-----",
				Auth:   &registryv1alpha1.Auth{Auth: password},
			},
		}},
	})

	errs := Validate(cfg)
	require.NotEmpty(t, errs)
	assert.NotContains(t, errs.ToAggregate().Error(), password)
}
