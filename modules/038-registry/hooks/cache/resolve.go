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

// Package cache resolves the registry module's primary upstream and cache
// settings from mc/registry config-values into registry.internal.cache, which
// the cache Deployment's Helm templates consume.
package cache

import (
	"encoding/base64"
	"fmt"

	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

// Credentials is the upstream credential input from config-values.
type Credentials struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	DockerCfg string `json:"dockerCfg,omitempty"`
}

// UpstreamConfig mirrors registry.upstream from config-values.
type UpstreamConfig struct {
	Host        string       `json:"host,omitempty"`
	Path        string       `json:"path,omitempty"`
	Scheme      string       `json:"scheme,omitempty"`
	CA          string       `json:"ca,omitempty"`
	Credentials *Credentials `json:"credentials,omitempty"`
}

// CacheConfig mirrors registry.cache from config-values.
type CacheConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	TTL     string `json:"ttl,omitempty"`
}

// ResolvedUpstream is the cache-template-facing upstream view.
type ResolvedUpstream struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Path     string `json:"path,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	HasCA    bool   `json:"hasCA"`
	TTL      string `json:"ttl,omitempty"`
}

// CacheValues is what the hook writes to registry.internal.cache.
type CacheValues struct {
	Enabled  bool              `json:"enabled"`
	Upstream *ResolvedUpstream `json:"upstream,omitempty"`
}

// Resolve builds the internal cache values from config-values. When upstream is
// absent (air-gap), Upstream is nil and the cache is an authoritative store.
func Resolve(upstream *UpstreamConfig, cache CacheConfig) (CacheValues, error) {
	out := CacheValues{Enabled: cache.Enabled}

	if upstream == nil || upstream.Host == "" {
		return out, nil
	}

	scheme := upstream.Scheme
	if scheme == "" {
		scheme = "HTTPS"
	}

	ru := &ResolvedUpstream{
		Scheme: scheme,
		Host:   upstream.Host,
		Path:   upstream.Path,
		HasCA:  upstream.CA != "",
		TTL:    cache.TTL,
	}

	if creds := upstream.Credentials; creds != nil {
		if creds.Username != "" {
			ru.Username = creds.Username
			ru.Password = creds.Password
		} else if creds.DockerCfg != "" {
			rawDockerCfg, err := base64.StdEncoding.DecodeString(creds.DockerCfg)
			if err != nil {
				return out, fmt.Errorf("decode dockerCfg base64 for %q: %w", upstream.Host, err)
			}
			user, pass, err := registry_helpers.CredsFromDockerCfg(rawDockerCfg, upstream.Host)
			if err != nil {
				return out, fmt.Errorf("resolve dockerCfg credentials for %q: %w", upstream.Host, err)
			}
			ru.Username = user
			ru.Password = pass
		}
	}

	out.Upstream = ru
	return out, nil
}
