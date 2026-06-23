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
	"encoding/json"
	"fmt"
	"strings"

	"registry-agent/internal/containerd"
	"registry-agent/internal/proxy"
)

// dockerCfgJSON is the shape of a base64-decoded dockercfg secret
// (~/.docker/config.json): {"auths":{"<host>":{"username":"..","password":"..","auth":".."}}}
type dockerCfgJSON struct {
	Auths map[string]dockerCfgEntry `json:"auths"`
}

type dockerCfgEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// credsFor resolves proxy credentials for an upstream: prefers explicit
// username/password; otherwise parses the base64-encoded dockerCfg
// (~/.docker/config.json) and extracts credentials for the upstream host.
// Returns nil when no usable credentials are present (anonymous).
func credsFor(u *UpstreamSpec) (*proxy.Credentials, error) {
	if u.Credentials == nil {
		return nil, nil
	}
	c := u.Credentials
	if c.Username != "" || c.Password != "" {
		return &proxy.Credentials{Username: c.Username, Password: c.Password}, nil
	}
	if c.DockerCfg == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(c.DockerCfg)
	if err != nil {
		return nil, fmt.Errorf("credsFor: base64-decode dockerCfg: %w", err)
	}
	var cfg dockerCfgJSON
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("credsFor: parse dockerCfg JSON: %w", err)
	}

	// Select the auth entry: prefer exact host match, fall back to sole entry.
	var entry dockerCfgEntry
	var found bool
	if e, ok := cfg.Auths[u.Host]; ok {
		entry, found = e, true
	} else if len(cfg.Auths) == 1 {
		for _, e := range cfg.Auths {
			entry, found = e, true
		}
	}
	if !found {
		return nil, nil
	}

	if entry.Username != "" || entry.Password != "" {
		return &proxy.Credentials{Username: entry.Username, Password: entry.Password}, nil
	}
	if entry.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return nil, fmt.Errorf("credsFor: base64-decode auth field: %w", err)
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) == 2 {
			return &proxy.Credentials{Username: parts[0], Password: parts[1]}, nil
		}
	}
	return nil, nil
}

const (
	// PrimaryHost is the fixed virtual registry host for DKP images.
	PrimaryHost = "registry.d8-system.svc:5001"
	// PrimaryLocalPathAlias is the fixed local repository namespace for the
	// primary registry (mirrors distribution's localpathalias).
	PrimaryLocalPathAlias = "system/deckhouse"
	// CacheURL is the fixed on-master cache endpoint.
	CacheURL = "https://registry-cache.d8-system.svc:5001"
)

var capabilities = []string{"pull", "resolve"}

// Options carries cluster facts the mapping needs that are not in the CR.
type Options struct {
	// AgentURL is the local agent endpoint containerd should use, e.g.
	// "https://127.0.0.1:5001".
	AgentURL string
	// ModuleCA is the PEM module CA that signs both the agent and the cache
	// serving certs (used for the containerd entries).
	ModuleCA string
	// Seed, when non-nil, is the on-node bootstrap seed appended as the lowest
	// priority containerd mirror on the primary host during the air-gap install
	// window. nil after the seed is torn down (the seed file is removed).
	Seed *SeedMirror
}

// Build maps a RegistryConfig into the containerd desired state and the proxy
// routes. cache-on entries get an [agent, cache] containerd failover and a
// cache-mode route; cache-off entries get an [agent]-only containerd entry and a
// direct-mode route (which requires an upstream).
func Build(cfg RegistryConfig, opts Options) (containerd.DesiredState, []proxy.Route, error) {
	ds := containerd.DesiredState{Hosts: map[string]containerd.HostConfig{}}
	routes := make([]proxy.Route, 0, len(cfg.Registries))

	for _, e := range cfg.Registries {
		cacheOn := e.Cache != nil && e.Cache.Enabled

		// containerd entries: agent primary, cache failover only when cache-on.
		entries := []containerd.HostEntry{{
			URL:          opts.AgentURL,
			Capabilities: capabilities,
			CA:           opts.ModuleCA,
		}}
		if cacheOn {
			entries = append(entries, containerd.HostEntry{
				URL:          CacheURL,
				Capabilities: capabilities,
				CA:           opts.ModuleCA,
			})
		}
		// Bootstrap seed fallback: lowest-priority mirror on the primary host
		// only, present only during the air-gap install window. Tried last, so
		// containerd serves from agent/cache first and falls back to the seed for
		// images not yet in the cache.
		if opts.Seed != nil && (e.Host == PrimaryHost || e.Source == SourcePrimary) {
			entries = append(entries, containerd.HostEntry{
				URL:          opts.Seed.URL,
				Capabilities: capabilities,
				CA:           opts.Seed.CA,
			})
		}
		ds.Hosts[e.Host] = containerd.HostConfig{Server: e.Host, Entries: entries}

		// proxy route.
		if cacheOn {
			routes = append(routes, proxy.Route{NS: e.Host, Mode: proxy.ModeCache, CacheURL: CacheURL})
			continue
		}
		if e.Upstream == nil {
			return containerd.DesiredState{}, nil, fmt.Errorf("registry %q: cache disabled but no upstream configured", e.Host)
		}
		up, err := upstreamFor(e)
		if err != nil {
			return containerd.DesiredState{}, nil, fmt.Errorf("registry %q: %w", e.Host, err)
		}
		routes = append(routes, proxy.Route{
			NS:       e.Host,
			Mode:     proxy.ModeDirect,
			Upstream: up,
		})
	}

	return ds, routes, nil
}

func upstreamFor(e RegistryEntry) (*proxy.Upstream, error) {
	u := e.Upstream
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		scheme = "https"
	}
	creds, err := credsFor(u)
	if err != nil {
		return nil, err
	}
	up := &proxy.Upstream{
		URL:            scheme + "://" + u.Host,
		CA:             u.CA,
		LocalPathAlias: localAliasFor(e),
		RemotePath:     strings.Trim(u.Path, "/"),
		Creds:          creds,
	}
	return up, nil
}

// localAliasFor returns the local repository-namespace prefix to strip for an
// entry: the fixed PrimaryLocalPathAlias for the primary registry, empty for
// additional / module-source entries (their repo path is used as-is).
func localAliasFor(e RegistryEntry) string {
	if e.Host == PrimaryHost || e.Source == SourcePrimary {
		return PrimaryLocalPathAlias
	}
	return ""
}
