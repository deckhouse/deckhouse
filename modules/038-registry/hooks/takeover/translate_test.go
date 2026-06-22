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

package takeover

import "testing"

func TestDeriveFromLegacy(t *testing.T) {
	t.Run("Direct: upstream from imagesRepo, cache off", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{
			Mode:       "Direct",
			ImagesRepo: "registry.example.com/deckhouse/ee",
			Scheme:     "HTTPS",
			CA:         "CA-PEM",
			Username:   "u",
			Password:   "p",
		})
		if !d.Present {
			t.Fatal("expected Present=true")
		}
		if d.Upstream == nil {
			t.Fatal("expected upstream")
		}
		if d.Upstream.Host != "registry.example.com" || d.Upstream.Path != "/deckhouse/ee" {
			t.Errorf("upstream host/path: %q %q", d.Upstream.Host, d.Upstream.Path)
		}
		if d.Upstream.Scheme != "HTTPS" || d.Upstream.CA != "CA-PEM" {
			t.Errorf("scheme/ca: %q %q", d.Upstream.Scheme, d.Upstream.CA)
		}
		if d.Upstream.Credentials == nil || d.Upstream.Credentials.Username != "u" || d.Upstream.Credentials.Password != "p" {
			t.Errorf("credentials: %+v", d.Upstream.Credentials)
		}
		if d.Cache.Enabled {
			t.Error("Direct must have cache disabled")
		}
	})

	t.Run("Proxy: upstream + cache on with ttl", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{
			Mode:       "Proxy",
			ImagesRepo: "registry.example.com/deckhouse/ee",
			Scheme:     "HTTPS",
			TTL:        "24h",
		})
		if !d.Present || d.Upstream == nil {
			t.Fatal("expected present upstream")
		}
		if !d.Cache.Enabled || d.Cache.TTL != "24h" {
			t.Errorf("cache: %+v", d.Cache)
		}
	})

	t.Run("Local: air-gap, no upstream, cache on", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{Mode: "Local"})
		if !d.Present {
			t.Fatal("expected Present=true")
		}
		if d.Upstream != nil {
			t.Errorf("Local must have no upstream, got %+v", d.Upstream)
		}
		if !d.Cache.Enabled {
			t.Error("Local must have cache enabled")
		}
	})

	t.Run("Unmanaged: nothing to translate", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{Mode: "Unmanaged", ImagesRepo: "x/y"})
		if d.Present {
			t.Errorf("Unmanaged must not derive, got %+v", d)
		}
	})

	t.Run("unknown mode: nothing to translate", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{Mode: "Bogus"})
		if d.Present {
			t.Error("unknown mode must not derive")
		}
	})

	t.Run("Direct without credentials: nil credentials", func(t *testing.T) {
		d := deriveFromLegacy(LegacyConfig{Mode: "Direct", ImagesRepo: "r.io/p", Scheme: "HTTP"})
		if d.Upstream == nil || d.Upstream.Credentials != nil {
			t.Errorf("expected upstream with nil credentials, got %+v", d.Upstream)
		}
	})
}
