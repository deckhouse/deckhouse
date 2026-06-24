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

// Package proxy is the registry-agent reverse proxy: it routes registry
// requests by the containerd "ns" parameter and forwards them to the on-master
// cache (cache mode) or directly to the upstream registry (direct mode).
package proxy

import (
	"sort"
	"strings"
)

// Mode selects how a route forwards requests.
type Mode int

const (
	// ModeCache forwards transparently to the on-master cache.
	ModeCache Mode = iota
	// ModeDirect forwards to the real upstream registry with credential injection.
	ModeDirect
)

// Credentials are registry username/password credentials.
type Credentials struct {
	Username string
	Password string
}

// Upstream describes a real upstream registry for ModeDirect routes.
type Upstream struct {
	// URL is the upstream base URL, e.g. "https://registry.deckhouse.io".
	URL string
	// CA is an optional PEM CA bundle for the upstream TLS connection.
	CA string
	// Creds are optional upstream credentials; nil means anonymous.
	Creds *Credentials
	// LocalPathAlias is the local repository-namespace prefix (after /v2/) that
	// the agent strips before forwarding; e.g. "system/deckhouse". Empty means
	// no prefix to strip.
	LocalPathAlias string
	// RemotePath is the upstream repository-namespace prefix the agent prepends;
	// e.g. "deckhouse/ee". Empty means none.
	RemotePath string
}

// Route is the forwarding rule for one managed registry, keyed by NS (the
// registry host carried in containerd's ns query param / the request Host) and,
// optionally, a repository PathPrefix (the leading repo segment after /v2/ that
// identifies a sub-route under NS — used for module-source entries that share
// the primary host). Empty PathPrefix = the default route for the NS.
type Route struct {
	NS         string
	PathPrefix string
	Mode       Mode
	CacheURL   string    // base URL of the on-master cache (ModeCache)
	CacheCA    string    // PEM CA verifying the cache's HTTPS serving cert (ModeCache)
	Upstream   *Upstream // upstream registry (ModeDirect)
}

// nsRoutes holds, for one NS, the path-prefix routes (longest-first) and an
// optional default route (PathPrefix == "").
type nsRoutes struct {
	prefixed []Route
	def      *Route
}

// Router resolves an incoming request to a Route by ns (or Host fallback) and,
// under that key, by repository path prefix.
type Router struct {
	routes map[string]nsRoutes
}

// NewRouter builds a Router from the given routes, grouping by NS and ordering
// each NS's path-prefix routes longest-first (longest prefix wins).
func NewRouter(routes []Route) *Router {
	m := make(map[string]nsRoutes, len(routes))
	for _, r := range routes {
		nr := m[r.NS]
		if r.PathPrefix == "" {
			rr := r
			nr.def = &rr
		} else {
			nr.prefixed = append(nr.prefixed, r)
		}
		m[r.NS] = nr
	}
	for ns, nr := range m {
		sort.SliceStable(nr.prefixed, func(i, j int) bool {
			return len(nr.prefixed[i].PathPrefix) > len(nr.prefixed[j].PathPrefix)
		})
		m[ns] = nr
	}
	return &Router{routes: m}
}

// Match returns the route for the request: keyed by ns (or Host when ns is
// empty), then by the longest path-prefix whose repo segment matches; falling
// back to the NS's default route. ok is false when nothing matches.
func (r *Router) Match(ns, host, path string) (Route, bool) {
	key := ns
	if key == "" {
		key = host
	}
	nr, ok := r.routes[key]
	if !ok {
		return Route{}, false
	}
	repo := repoFromPath(path)
	for _, pr := range nr.prefixed {
		if repo == pr.PathPrefix || strings.HasPrefix(repo, pr.PathPrefix+"/") {
			return pr, true
		}
	}
	if nr.def != nil {
		return *nr.def, true
	}
	return Route{}, false
}

// repoFromPath returns the repository portion of a registry path (after "/v2/"),
// or "" when the path is not a /v2/ request.
func repoFromPath(path string) string {
	const v2 = "/v2/"
	if !strings.HasPrefix(path, v2) {
		return ""
	}
	return path[len(v2):]
}
