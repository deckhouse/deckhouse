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

func TestBootstrapSeedHostsLocal(t *testing.T) {
	const ca = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	hosts := BootstrapSeedHostsLocal(ca, "ro", "ropass")

	h, ok := hosts["registry.d8-system.svc:5001"]
	if !ok {
		t.Fatalf("missing host key registry.d8-system.svc:5001; got %v", hosts)
	}
	// Seed only: the cache (registry-cache.d8-system.svc) must NOT be listed in the
	// bootstrap drop-in — its pod and cluster DNS do not exist during first-master
	// bring-up, so listing it makes containerd fail control-plane image pulls on
	// "no such host". The agent adds the cache after bring-up.
	if len(h.Mirrors) != 1 {
		t.Fatalf("mirrors = %d, want 1 (seed only)", len(h.Mirrors))
	}
	if h.Mirrors[0].Host != "127.0.0.1:5010" {
		t.Fatalf("mirror0 host = %q, want 127.0.0.1:5010 (seed only)", h.Mirrors[0].Host)
	}
	if h.Mirrors[0].Scheme != "https" || h.Mirrors[0].CA != ca {
		t.Fatalf("mirror0 = %+v, want https + module CA", h.Mirrors[0])
	}
	// Must carry the read-only PKI creds: the seed's docker-auth rejects anonymous
	// pull (401), so containerd has to authenticate.
	if h.Mirrors[0].Auth.Username != "ro" || h.Mirrors[0].Auth.Password != "ropass" {
		t.Fatalf("mirror0 auth = %+v, want ro/ropass", h.Mirrors[0].Auth)
	}
	// No rewrites: seed served rooted at system/deckhouse; imagesBase carries path.
	if len(h.Mirrors[0].Rewrites) != 0 {
		t.Fatalf("unexpected rewrites: %+v", h.Mirrors)
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
