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

package proxy

import "testing"

func testRouter() *Router {
	return NewRouter([]Route{
		{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: "https://10.0.0.1:5001"},
		{NS: "docker.io", Mode: ModeDirect, Upstream: &Upstream{URL: "https://registry-1.docker.io"}},
	})
}

func TestRouter_MatchByNS(t *testing.T) {
	r := testRouter()
	got, ok := r.Match("docker.io", "registry.d8-system.svc:5001", "")
	if !ok {
		t.Fatal("expected match by ns")
	}
	if got.Mode != ModeDirect || got.Upstream == nil || got.Upstream.URL != "https://registry-1.docker.io" {
		t.Fatalf("ns match returned wrong route: %+v", got)
	}
}

func TestRouter_FallbackToHostWhenNSEmpty(t *testing.T) {
	r := testRouter()
	got, ok := r.Match("", "registry.d8-system.svc:5001", "")
	if !ok {
		t.Fatal("expected fallback match by host")
	}
	if got.Mode != ModeCache || got.CacheURL != "https://10.0.0.1:5001" {
		t.Fatalf("host fallback returned wrong route: %+v", got)
	}
}

func TestRouter_NoMatch(t *testing.T) {
	r := testRouter()
	if _, ok := r.Match("quay.io", "quay.io", ""); ok {
		t.Fatal("expected no match for unknown registry")
	}
}

func TestRouter_PathPrefixMatch(t *testing.T) {
	primary := Route{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: "https://cache"}
	modA := Route{NS: "registry.d8-system.svc:5001", PathPrefix: "nexus.example.com/modules/a", Mode: ModeDirect, Upstream: &Upstream{URL: "https://nexus.example.com"}}
	modB := Route{NS: "registry.d8-system.svc:5001", PathPrefix: "nexus.example.com/modules/ab", Mode: ModeDirect, Upstream: &Upstream{URL: "https://nexus.example.com"}}
	r := NewRouter([]Route{primary, modA, modB})

	cases := []struct {
		name      string
		path      string
		wantPfx   string // expected matched route's PathPrefix
		wantFound bool
	}{
		{"module-source A exact-ish", "/v2/nexus.example.com/modules/a/img/manifests/x", "nexus.example.com/modules/a", true},
		{"longest wins (ab over a)", "/v2/nexus.example.com/modules/ab/img/manifests/x", "nexus.example.com/modules/ab", true},
		{"primary default for system/deckhouse", "/v2/system/deckhouse/foo/manifests/x", "", true},
		{"non-matching path -> default", "/v2/other/thing/manifests/x", "", true},
		{"partial segment is NOT a prefix match", "/v2/nexus.example.com/modules/abc/img/manifests/x", "", true},
		{"/v2/ ping -> default", "/v2/", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := r.Match("registry.d8-system.svc:5001", "", tc.path)
			if ok != tc.wantFound {
				t.Fatalf("found = %v, want %v", ok, tc.wantFound)
			}
			if got.PathPrefix != tc.wantPfx {
				t.Fatalf("matched PathPrefix = %q, want %q", got.PathPrefix, tc.wantPfx)
			}
		})
	}
}

func TestRouter_NoDefaultNoPrefix(t *testing.T) {
	// NS with only a prefix route and no default: non-matching path -> not found.
	r := NewRouter([]Route{{NS: "h", PathPrefix: "p", Mode: ModeDirect, Upstream: &Upstream{URL: "u"}}})
	if _, ok := r.Match("h", "", "/v2/other/x"); ok {
		t.Fatal("expected no match when no default and prefix does not match")
	}
	if _, ok := r.Match("h", "", "/v2/p/x"); !ok {
		t.Fatal("expected match on the prefix route")
	}
}

func TestRouter_HostFallbackUnchanged(t *testing.T) {
	// ns empty -> Host key; default route returned regardless of path (back-compat).
	r := NewRouter([]Route{{NS: "docker.io", Mode: ModeDirect, Upstream: &Upstream{URL: "https://registry-1.docker.io"}}})
	got, ok := r.Match("", "docker.io", "/v2/library/nginx/manifests/x")
	if !ok || got.NS != "docker.io" {
		t.Fatalf("host-fallback default route not matched: %+v ok=%v", got, ok)
	}
}
