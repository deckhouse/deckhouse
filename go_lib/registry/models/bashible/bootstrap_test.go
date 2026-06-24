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

package bashible

import "testing"

func TestBootstrapAirGapHostsLocal(t *testing.T) {
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	hosts := BootstrapAirGapHostsLocal(ca, "ro", "ropass")

	h, ok := hosts["registry.d8-system.svc:5001"]
	if !ok {
		t.Fatalf("missing host key registry.d8-system.svc:5001; got %v", hosts)
	}
	// Two ordered mirrors: on-master cache loopback first, then the bundle tunnel
	// fallback. The cache Service DNS is not listed (no cluster DNS at bring-up).
	if len(h.Mirrors) != 2 {
		t.Fatalf("mirrors = %d, want 2 (cache + bundle tunnel)", len(h.Mirrors))
	}

	cache := h.Mirrors[0]
	if cache.Host != "127.0.0.1:5001" {
		t.Fatalf("mirror0 host = %q, want 127.0.0.1:5001 (cache)", cache.Host)
	}
	if cache.Scheme != "https" || cache.CA != ca {
		t.Fatalf("mirror0 = %+v, want https + module CA", cache)
	}
	if cache.Auth.Username != "ro" || cache.Auth.Password != "ropass" {
		t.Fatalf("mirror0 auth = %+v, want ro/ropass", cache.Auth)
	}
	if len(cache.Rewrites) != 0 {
		t.Fatalf("unexpected rewrites on cache mirror: %+v", cache.Rewrites)
	}

	bundle := h.Mirrors[1]
	if bundle.Host != "127.0.0.1:5511" {
		t.Fatalf("mirror1 host = %q, want 127.0.0.1:5511 (bundle tunnel)", bundle.Host)
	}
	// The bundle registry is plain HTTP with no auth (loopback over the SSH tunnel).
	if bundle.Scheme != "http" {
		t.Fatalf("mirror1 scheme = %q, want http (bundle tunnel)", bundle.Scheme)
	}
	if bundle.CA != "" || bundle.Auth.Username != "" || bundle.Auth.Password != "" {
		t.Fatalf("mirror1 = %+v, want no CA / no auth", bundle)
	}
	if len(bundle.Rewrites) != 0 {
		t.Fatalf("unexpected rewrites on bundle mirror: %+v", bundle.Rewrites)
	}
}

func TestBootstrapUpstreamHosts(t *testing.T) {
	hosts := BootstrapUpstreamHosts("registry.example.com", "https", "CA", "u", "p", "deckhouse/ee")
	h, ok := hosts["registry.d8-system.svc:5001"]
	if !ok || len(h.Mirrors) != 1 {
		t.Fatalf("want one mirror under primary host, got %+v", hosts)
	}
	m := h.Mirrors[0]
	if m.Host != "registry.example.com" || m.Scheme != "https" || m.CA != "CA" {
		t.Fatalf("mirror target wrong: %+v", m)
	}
	if m.Auth.Username != "u" || m.Auth.Password != "p" {
		t.Fatalf("mirror auth wrong: %+v", m.Auth)
	}
	if len(m.Rewrites) != 1 || m.Rewrites[0].From != "^system/deckhouse" || m.Rewrites[0].To != "deckhouse/ee" {
		t.Fatalf("mirror rewrite wrong: %+v", m.Rewrites)
	}
}
