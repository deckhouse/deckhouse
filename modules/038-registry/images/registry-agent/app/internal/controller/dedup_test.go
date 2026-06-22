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

package controller

import (
	"testing"

	"registry-agent/internal/config"
	"registry-agent/internal/proxy"
)

// TestDedupRoutes_PreservesModuleSourcePrefixes guards the regression where
// dedupRoutes keyed on NS alone collapsed every module-source route (all
// NS=PrimaryHost, distinct PathPrefix) and the primary's default route into one,
// making path-prefix routing inert in the live agent. It exercises the full
// Build -> dedupRoutes -> NewRouter chain the unit tests of proxy/config bypass.
func TestDedupRoutes_PreservesModuleSourcePrefixes(t *testing.T) {
	cfg := config.RegistryConfig{Registries: []config.RegistryEntry{
		{
			Host:     config.PrimaryHost,
			Source:   config.SourcePrimary,
			Upstream: &config.UpstreamSpec{Host: "registry.deckhouse.io", Path: "deckhouse/ee", Scheme: "HTTPS"},
			Cache:    &config.CacheSpec{Enabled: false},
		},
		{
			Host:     "nexus.example.com/modules/a",
			Source:   config.SourceModuleSource,
			Upstream: &config.UpstreamSpec{Host: "nexus.example.com", Path: "modules/a", Scheme: "HTTPS"},
		},
		{
			Host:     "nexus.example.com/modules/b",
			Source:   config.SourceModuleSource,
			Upstream: &config.UpstreamSpec{Host: "nexus.example.com", Path: "modules/b", Scheme: "HTTPS"},
		},
	}}

	_, routes, err := config.Build(cfg, config.Options{AgentURL: "https://127.0.0.1:5001"})
	if err != nil {
		t.Fatal(err)
	}

	router := proxy.NewRouter(dedupRoutes(routes))

	// Primary default route must survive (system/deckhouse paths).
	if _, ok := router.Match(config.PrimaryHost, "", "/v2/system/deckhouse/img/manifests/x"); !ok {
		t.Error("primary default route was dropped by dedup")
	}
	// Both module-source prefixes must survive and resolve to their own route.
	a, ok := router.Match(config.PrimaryHost, "", "/v2/nexus.example.com/modules/a/img/manifests/x")
	if !ok || a.PathPrefix != "nexus.example.com/modules/a" {
		t.Errorf("module-source A dropped/mismatched: ok=%v prefix=%q", ok, a.PathPrefix)
	}
	b, ok := router.Match(config.PrimaryHost, "", "/v2/nexus.example.com/modules/b/img/manifests/x")
	if !ok || b.PathPrefix != "nexus.example.com/modules/b" {
		t.Errorf("module-source B dropped/mismatched: ok=%v prefix=%q", ok, b.PathPrefix)
	}
}
