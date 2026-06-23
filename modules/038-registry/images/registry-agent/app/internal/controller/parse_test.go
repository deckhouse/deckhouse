// Copyright 2026 Flant JSC
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

package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"registry-agent/internal/config"
)

// fullObject builds an *unstructured.Unstructured with a fully-specified spec.
func fullObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"registries": []interface{}{
					map[string]interface{}{
						"host":   "registry.d8-system.svc:5001",
						"source": "Primary",
						"upstream": map[string]interface{}{
							"host":   "registry.deckhouse.io",
							"path":   "deckhouse/ee",
							"scheme": "HTTPS",
							"ca":     "-----BEGIN CERTIFICATE-----...",
							"credentials": map[string]interface{}{
								"username": "user",
								"password": "pass",
							},
						},
						"cache": map[string]interface{}{"enabled": true},
					},
					map[string]interface{}{
						"host":   "cr.example.io",
						"source": "Additional",
						"upstream": map[string]interface{}{
							"host":   "cr.example.io",
							"path":   "",
							"scheme": "HTTPS",
							"credentials": map[string]interface{}{
								"dockerCfg": "test",
							},
						},
						"cache": map[string]interface{}{"enabled": false},
					},
					map[string]interface{}{
						"host":   "minimal.example.io",
						"source": "Additional",
						// no upstream, no cache
					},
				},
				"auth": map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{
							"name": "admin",
							"role": "ReadWrite",
						},
						map[string]interface{}{
							"name": "viewer",
							"role": "ReadOnly",
						},
					},
				},
			},
		},
	}
}

// TestParseRegistryConfig_Primary checks that the first (Primary) entry is parsed correctly.
func TestParseRegistryConfig_Primary(t *testing.T) {
	cfg, err := parseRegistryConfig(fullObject())
	if err != nil {
		t.Fatalf("parseRegistryConfig: %v", err)
	}
	if len(cfg.Registries) < 1 {
		t.Fatal("expected at least 1 registry entry")
	}
	e := cfg.Registries[0]
	if e.Host != "registry.d8-system.svc:5001" {
		t.Errorf("host: got %q, want %q", e.Host, "registry.d8-system.svc:5001")
	}
	if e.Source != "Primary" {
		t.Errorf("source: got %q, want %q", e.Source, "Primary")
	}
	if e.Upstream == nil {
		t.Fatal("upstream is nil, want non-nil")
	}
	if e.Upstream.Host != "registry.deckhouse.io" {
		t.Errorf("upstream.host: got %q, want %q", e.Upstream.Host, "registry.deckhouse.io")
	}
	if e.Upstream.Path != "deckhouse/ee" {
		t.Errorf("upstream.path: got %q, want %q", e.Upstream.Path, "deckhouse/ee")
	}
	if e.Upstream.Scheme != "HTTPS" {
		t.Errorf("upstream.scheme: got %q, want %q", e.Upstream.Scheme, "HTTPS")
	}
	if e.Upstream.CA != "-----BEGIN CERTIFICATE-----..." {
		t.Errorf("upstream.ca: got %q, want %q", e.Upstream.CA, "-----BEGIN CERTIFICATE-----...")
	}
	if e.Upstream.Credentials == nil {
		t.Fatal("upstream.credentials is nil, want non-nil")
	}
	if e.Upstream.Credentials.Username != "user" {
		t.Errorf("credentials.username: got %q, want %q", e.Upstream.Credentials.Username, "user")
	}
	if e.Upstream.Credentials.Password != "pass" {
		t.Errorf("credentials.password: got %q, want %q", e.Upstream.Credentials.Password, "pass")
	}
	if e.Cache == nil || !e.Cache.Enabled {
		t.Errorf("cache.enabled: got false/nil, want true")
	}
}

// TestParseRegistryConfig_Additional checks the second (Additional) entry.
func TestParseRegistryConfig_Additional(t *testing.T) {
	cfg, err := parseRegistryConfig(fullObject())
	if err != nil {
		t.Fatalf("parseRegistryConfig: %v", err)
	}
	if len(cfg.Registries) < 2 {
		t.Fatal("expected at least 2 registry entries")
	}
	e := cfg.Registries[1]
	if e.Host != "cr.example.io" {
		t.Errorf("host: got %q, want %q", e.Host, "cr.example.io")
	}
	if e.Source != "Additional" {
		t.Errorf("source: got %q, want %q", e.Source, "Additional")
	}
	if e.Upstream == nil {
		t.Fatal("upstream is nil, want non-nil")
	}
	if e.Upstream.Credentials == nil {
		t.Fatal("upstream.credentials is nil, want non-nil")
	}
	if e.Upstream.Credentials.DockerCfg != "test" {
		t.Errorf("credentials.dockerCfg: got %q, want %q", e.Upstream.Credentials.DockerCfg, "test")
	}
	if e.Cache == nil {
		t.Fatal("cache is nil, want non-nil")
	}
	if e.Cache.Enabled {
		t.Errorf("cache.enabled: got true, want false")
	}
}

// TestParseRegistryConfig_AbsentOptionals checks that a minimal entry (no upstream/cache)
// results in nil pointers rather than errors.
func TestParseRegistryConfig_AbsentOptionals(t *testing.T) {
	cfg, err := parseRegistryConfig(fullObject())
	if err != nil {
		t.Fatalf("parseRegistryConfig: %v", err)
	}
	if len(cfg.Registries) < 3 {
		t.Fatal("expected at least 3 registry entries")
	}
	e := cfg.Registries[2]
	if e.Host != "minimal.example.io" {
		t.Errorf("host: got %q, want %q", e.Host, "minimal.example.io")
	}
	if e.Source != "Additional" {
		t.Errorf("source: got %q, want %q", e.Source, "Additional")
	}
	if e.Upstream != nil {
		t.Errorf("upstream: got non-nil, want nil")
	}
	if e.Cache != nil {
		t.Errorf("cache: got non-nil, want nil")
	}
}

// TestParseRegistryConfig_AuthUsers checks auth.users slice parsing.
func TestParseRegistryConfig_AuthUsers(t *testing.T) {
	cfg, err := parseRegistryConfig(fullObject())
	if err != nil {
		t.Fatalf("parseRegistryConfig: %v", err)
	}
	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("auth.users: got %d, want 2", len(cfg.Auth.Users))
	}
	want := []config.UserSpec{
		{Name: "admin", Role: "ReadWrite"},
		{Name: "viewer", Role: "ReadOnly"},
	}
	for i, u := range cfg.Auth.Users {
		if u.Name != want[i].Name {
			t.Errorf("user[%d].name: got %q, want %q", i, u.Name, want[i].Name)
		}
		if u.Role != want[i].Role {
			t.Errorf("user[%d].role: got %q, want %q", i, u.Role, want[i].Role)
		}
	}
}

// TestParseRegistryConfig_Empty checks that an empty spec returns a zero RegistryConfig.
func TestParseRegistryConfig_Empty(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	cfg, err := parseRegistryConfig(u)
	if err != nil {
		t.Fatalf("parseRegistryConfig: unexpected error: %v", err)
	}
	if len(cfg.Registries) != 0 {
		t.Errorf("registries: got %d, want 0", len(cfg.Registries))
	}
	if len(cfg.Auth.Users) != 0 {
		t.Errorf("auth.users: got %d, want 0", len(cfg.Auth.Users))
	}
}
