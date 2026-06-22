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

import (
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

// deriveFromLegacy maps the legacy orchestrator's desired state to the new-arch
// upstream/cache config. Pure: no I/O, no clock.
//
//	Direct    -> upstream from imagesRepo, cache OFF
//	Proxy     -> upstream from imagesRepo, cache ON (ttl)
//	Local     -> NO upstream (air-gap), cache ON      (store synced by 7b-5)
//	Unmanaged -> not present (operator mc config flows through)
func deriveFromLegacy(lc LegacyConfig) DerivedConfig {
	switch registry_const.ToModeType(lc.Mode) {
	case registry_const.ModeDirect:
		return DerivedConfig{Present: true, Upstream: upstreamFromRepo(lc), Cache: DerivedCache{Enabled: false}}
	case registry_const.ModeProxy:
		return DerivedConfig{Present: true, Upstream: upstreamFromRepo(lc), Cache: DerivedCache{Enabled: true, TTL: lc.TTL}}
	case registry_const.ModeLocal:
		// Air-gap: no upstream. Images are served from the on-master cache,
		// which 7b-5 populates from the legacy store.
		return DerivedConfig{Present: true, Cache: DerivedCache{Enabled: true}}
	case registry_const.ModeUnmanaged:
		// The module does not manage a registry — nothing to translate; the
		// operator's mc/registry config flows through unchanged.
		return DerivedConfig{Present: false}
	default:
		// ToModeType maps every unrecognized string to ModeUnmanaged, so this is
		// belt-and-suspenders for any future ModeType the switch doesn't case.
		return DerivedConfig{Present: false}
	}
}

// upstreamFromRepo splits the legacy imagesRepo (host[/path]) into the new-arch
// upstream shape, carrying scheme/ca/credentials.
func upstreamFromRepo(lc LegacyConfig) *DerivedUpstream {
	host, path := registry_helpers.SplitAddressAndPath(lc.ImagesRepo)
	up := &DerivedUpstream{
		Host:   host,
		Path:   path,
		Scheme: lc.Scheme,
		CA:     lc.CA,
	}
	if lc.Username != "" || lc.Password != "" {
		up.Credentials = &DerivedCredentials{Username: lc.Username, Password: lc.Password}
	}
	return up
}
