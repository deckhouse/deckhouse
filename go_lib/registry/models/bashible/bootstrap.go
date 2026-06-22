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

package bashible

// BootstrapSeedHostsLocal builds the first-master bootstrap containerd drop-in
// for the air-gap (Local mode) install. containerd resolves
// registry.d8-system.svc:5001 to two mirrors, in failover order:
//
//  1. registry-cache.d8-system.svc:5001 (the in-cluster cache, preferred once up)
//  2. 127.0.0.1:5010 (the on-node raw-process seed, local disk — serves the
//     whole bring-up before the cache is ready)
//
// Both are https with the module CA. No path rewrite: both the seed store and the
// cache serve rooted at system/deckhouse, and imagesBase already carries the path
// (constant.HostWithPath = "registry.d8-system.svc:5001/system/deckhouse"), so
// containerd sends the correct /v2/system/deckhouse/... request directly.
//
// The seed is filled once over the SSH reverse tunnel (bashible registry-syncer
// 127.0.0.1:5511 -> 127.0.0.1:5010), then the tunnel is never in the bring-up
// pull path. After bring-up the registry-agent takes over registry.d and
// re-renders these mirrors (and appends the seed as its own lowest-priority
// fallback until the cache is filled and the seed is torn down).
func BootstrapSeedHostsLocal(ca string) map[string]ContextHosts {
	cacheMirror := ContextMirrorHost{
		Host:   "registry-cache.d8-system.svc:5001",
		Scheme: "https",
		CA:     ca,
	}
	seedMirror := ContextMirrorHost{
		Host:   "127.0.0.1:5010",
		Scheme: "https",
		CA:     ca,
	}
	return map[string]ContextHosts{
		"registry.d8-system.svc:5001": {Mirrors: []ContextMirrorHost{cacheMirror, seedMirror}},
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
