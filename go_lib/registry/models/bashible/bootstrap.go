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

const (
	// on-master bootstrap cache (hostNetwork pod) on the node loopback; the
	// distribution cert carries a 127.0.0.1 SAN.
	cacheBootstrapHostLocal = "127.0.0.1:5001"
	// dhctl-side bundle registry over the SSH reverse tunnel (plain HTTP) — the
	// bring-up fallback until the cache is filled.
	bundleTunnelHostLocal = "127.0.0.1:5511"
)

// BootstrapAirGapHostsLocal builds the first-master air-gap containerd drop-in:
// registry.d8-system.svc:5001 -> two ordered mirrors — the on-master cache loopback
// (https, module CA, RO creds) then the bundle tunnel (http) as the bring-up
// fallback (cache absent -> connection refused -> falls through to the bundle).
// No rewrite: both serve rooted at system/deckhouse (imagesBase carries the path).
// The cache Service DNS is not listed — no cluster DNS from the node netns at
// bring-up; the agent re-renders Service mirrors post-install.
func BootstrapAirGapHostsLocal(ca, username, password string) map[string]ContextHosts {
	cacheMirror := ContextMirrorHost{
		Host:   cacheBootstrapHostLocal,
		Scheme: "https",
		CA:     ca,
		Auth: ContextAuth{
			Username: username,
			Password: password,
		},
	}
	bundleMirror := ContextMirrorHost{
		Host:   bundleTunnelHostLocal,
		Scheme: "http",
	}
	return map[string]ContextHosts{
		"registry.d8-system.svc:5001": {Mirrors: []ContextMirrorHost{cacheMirror, bundleMirror}},
	}
}

// BootstrapUpstreamHosts builds the first-master bootstrap containerd drop-in for
// connected installs (Direct = no cache; connected+cache before the cache is up).
// containerd resolves registry.d8-system.svc:5001 to the upstream registry,
// rewriting the system/deckhouse prefix to the upstream repo path. After bring-up
// the registry-agent takes over registry.d and re-renders these mirrors.
func BootstrapUpstreamHosts(host, scheme, ca, username, password, toPath string) map[string]ContextHosts {
	mirror := ContextMirrorHost{
		Host:   host,
		Scheme: scheme,
		CA:     ca,
		Auth: ContextAuth{
			Username: username,
			Password: password,
		},
		Rewrites: []ContextRewrite{
			{From: "^system/deckhouse", To: toPath},
		},
	}
	return map[string]ContextHosts{
		"registry.d8-system.svc:5001": {Mirrors: []ContextMirrorHost{mirror}},
	}
}
