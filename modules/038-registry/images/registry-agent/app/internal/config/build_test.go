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

package config

import (
	"encoding/base64"
	"fmt"
	"testing"

	"registry-agent/internal/proxy"
)

func opts() Options { return Options{AgentURL: "https://127.0.0.1:5001", ModuleCA: "MODCA"} }

func TestBuild_PrimaryCacheOn(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:     "registry.d8-system.svc:5001",
		Source:   "Primary",
		Upstream: &UpstreamSpec{Host: "up.example.com", Path: "deckhouse/ee", Scheme: "HTTPS"},
		Cache:    &CacheSpec{Enabled: true},
	}}}
	ds, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	// containerd: agent primary + cache failover
	hc := ds.Hosts["registry.d8-system.svc:5001"]
	if len(hc.Entries) != 2 {
		t.Fatalf("want 2 containerd entries (agent+cache), got %d", len(hc.Entries))
	}
	if hc.Entries[0].URL != "https://127.0.0.1:5001" || hc.Entries[1].URL != "https://registry-cache.d8-system.svc:5001" {
		t.Fatalf("entries = %+v", hc.Entries)
	}
	// route: cache mode
	if len(routes) != 1 || routes[0].Mode != proxy.ModeCache || routes[0].NS != "registry.d8-system.svc:5001" {
		t.Fatalf("routes = %+v", routes)
	}
	if routes[0].CacheURL != "https://registry-cache.d8-system.svc:5001" {
		t.Fatalf("cache url = %q", routes[0].CacheURL)
	}
}

func TestBuild_PrimaryCacheOff(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:     "registry.d8-system.svc:5001",
		Source:   "Primary",
		Upstream: &UpstreamSpec{Host: "up.example.com", Path: "deckhouse/ee", Scheme: "HTTPS", Credentials: &Credentials{Username: "u", Password: "p"}},
		Cache:    &CacheSpec{Enabled: false},
	}}}
	ds, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	hc := ds.Hosts["registry.d8-system.svc:5001"]
	if len(hc.Entries) != 1 { // agent only, no cache failover
		t.Fatalf("want 1 containerd entry (agent only), got %d", len(hc.Entries))
	}
	r := routes[0]
	if r.Mode != proxy.ModeDirect || r.Upstream == nil {
		t.Fatalf("want direct route w/ upstream, got %+v", r)
	}
	if r.Upstream.URL != "https://up.example.com" || r.Upstream.RemotePath != "deckhouse/ee" || r.Upstream.LocalPathAlias != "system/deckhouse" {
		t.Fatalf("upstream = %+v", r.Upstream)
	}
	if r.Upstream.Creds == nil || r.Upstream.Creds.Username != "u" {
		t.Fatalf("creds not mapped: %+v", r.Upstream.Creds)
	}
}

func TestBuild_AdditionalCacheOff(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:     "docker.io",
		Source:   "Additional",
		Upstream: &UpstreamSpec{Host: "registry-1.docker.io", Path: "", Scheme: "HTTPS"},
		Cache:    &CacheSpec{Enabled: false},
	}}}
	_, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	r := routes[0]
	if r.NS != "docker.io" || r.Upstream.LocalPathAlias != "" {
		t.Fatalf("additional entry should have empty local alias: %+v", r.Upstream)
	}
	if r.Upstream.RemotePath != "" {
		t.Fatalf("additional entry with empty Path should have empty RemotePath, got %q", r.Upstream.RemotePath)
	}
}

func TestBuild_AirGapCacheOnNoUpstream(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   "registry.d8-system.svc:5001",
		Source: "Primary",
		Cache:  &CacheSpec{Enabled: true}, // no upstream => air-gap
	}}}
	ds, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Hosts["registry.d8-system.svc:5001"].Entries) != 2 {
		t.Fatal("air-gap still routes containerd to agent+cache")
	}
	if routes[0].Mode != proxy.ModeCache {
		t.Fatalf("air-gap must be cache mode, got %+v", routes[0])
	}
}

func TestBuild_CacheOffNoUpstreamIsError(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   "registry.d8-system.svc:5001",
		Source: "Primary",
		Cache:  &CacheSpec{Enabled: false}, // cache off + no upstream => invalid
	}}}
	if _, _, err := Build(cfg, opts()); err == nil {
		t.Fatal("expected error: cache-off entry requires an upstream")
	}
}

func TestBuild_DockerCfgCreds(t *testing.T) {
	// Build base64("user:pass") for the auth field.
	authField := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	// Build the dockercfg JSON and base64-encode it.
	dockerCfgJSON := fmt.Sprintf(`{"auths":{"up.example.com":{"auth":%q}}}`, authField)
	dockerCfg := base64.StdEncoding.EncodeToString([]byte(dockerCfgJSON))

	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   "up.example.com",
		Source: SourceAdditional,
		Upstream: &UpstreamSpec{
			Host:        "up.example.com",
			Scheme:      "HTTPS",
			Credentials: &Credentials{DockerCfg: dockerCfg},
		},
		Cache: &CacheSpec{Enabled: false},
	}}}
	_, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Upstream == nil || routes[0].Upstream.Creds == nil {
		t.Fatalf("expected one direct route with creds, got %+v", routes)
	}
	if routes[0].Upstream.Creds.Username != "user" {
		t.Errorf("username: want %q, got %q", "user", routes[0].Upstream.Creds.Username)
	}
	if routes[0].Upstream.Creds.Password != "pass" {
		t.Errorf("password: want %q, got %q", "pass", routes[0].Upstream.Creds.Password)
	}
}

func TestBuild_DockerCfgUsernamePassword(t *testing.T) {
	// Build dockercfg JSON with explicit username/password fields (not auth).
	dockerCfgJSON := `{"auths":{"up.example.com":{"username":"alice","password":"secret"}}}`
	dockerCfg := base64.StdEncoding.EncodeToString([]byte(dockerCfgJSON))

	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   "up.example.com",
		Source: SourceAdditional,
		Upstream: &UpstreamSpec{
			Host:        "up.example.com",
			Scheme:      "HTTPS",
			Credentials: &Credentials{DockerCfg: dockerCfg},
		},
		Cache: &CacheSpec{Enabled: false},
	}}}
	_, routes, err := Build(cfg, opts())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Upstream == nil || routes[0].Upstream.Creds == nil {
		t.Fatalf("expected one direct route with creds, got %+v", routes)
	}
	if routes[0].Upstream.Creds.Username != "alice" {
		t.Errorf("username: want %q, got %q", "alice", routes[0].Upstream.Creds.Username)
	}
	if routes[0].Upstream.Creds.Password != "secret" {
		t.Errorf("password: want %q, got %q", "secret", routes[0].Upstream.Creds.Password)
	}
}

func TestBuildAppendsSeedMirrorAfterCache(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   PrimaryHost,
		Source: SourcePrimary,
		Cache:  &CacheSpec{Enabled: true},
	}}}
	opts := Options{
		AgentURL: "https://127.0.0.1:5001",
		ModuleCA: "AGENTCA",
		Seed:     &SeedMirror{URL: "https://127.0.0.1:5010", CA: "SEEDCA"},
	}
	ds, _, err := Build(cfg, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	host := ds.Hosts[PrimaryHost]
	if len(host.Entries) != 3 {
		t.Fatalf("entries = %d, want 3 (agent, cache, seed)", len(host.Entries))
	}
	if host.Entries[0].URL != "https://127.0.0.1:5001" {
		t.Fatalf("entry0 = %q, want agent", host.Entries[0].URL)
	}
	if host.Entries[1].URL != CacheURL {
		t.Fatalf("entry1 = %q, want cache", host.Entries[1].URL)
	}
	if host.Entries[2].URL != "https://127.0.0.1:5010" || host.Entries[2].CA != "SEEDCA" {
		t.Fatalf("entry2 = %+v, want seed", host.Entries[2])
	}
}

func TestBuildNoSeedWhenOptionNil(t *testing.T) {
	cfg := RegistryConfig{Registries: []RegistryEntry{{
		Host:   PrimaryHost,
		Source: SourcePrimary,
		Cache:  &CacheSpec{Enabled: true},
	}}}
	opts := Options{AgentURL: "https://127.0.0.1:5001", ModuleCA: "AGENTCA"}
	ds, _, err := Build(cfg, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds.Hosts[PrimaryHost].Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (agent, cache; no seed)", len(ds.Hosts[PrimaryHost].Entries))
	}
}
