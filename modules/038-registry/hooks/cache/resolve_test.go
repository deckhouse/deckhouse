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

package cache

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_ExplicitCreds(t *testing.T) {
	out, err := Resolve(&UpstreamConfig{
		Host:        "registry.example.com",
		Path:        "/deckhouse/ee",
		Scheme:      "HTTPS",
		CA:          "PEM",
		Credentials: &Credentials{Username: "u", Password: "p"},
	}, CacheConfig{Enabled: true, TTL: "168h"})
	require.NoError(t, err)

	require.NotNil(t, out.Upstream)
	assert.True(t, out.Enabled)
	assert.Equal(t, "HTTPS", out.Upstream.Scheme)
	assert.Equal(t, "registry.example.com", out.Upstream.Host)
	assert.Equal(t, "/deckhouse/ee", out.Upstream.Path)
	assert.Equal(t, "u", out.Upstream.Username)
	assert.Equal(t, "p", out.Upstream.Password)
	assert.True(t, out.Upstream.HasCA)
	assert.Equal(t, "168h", out.Upstream.TTL)
}

func TestResolve_DockerCfg(t *testing.T) {
	cfg := map[string]any{
		"auths": map[string]any{
			"registry.example.com": map[string]any{
				"username": "du",
				"password": "dp",
			},
		},
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	dockerCfg := base64.StdEncoding.EncodeToString(raw)

	out, err := Resolve(&UpstreamConfig{
		Host:        "registry.example.com",
		Scheme:      "HTTPS",
		Credentials: &Credentials{DockerCfg: dockerCfg},
	}, CacheConfig{Enabled: true})
	require.NoError(t, err)

	require.NotNil(t, out.Upstream)
	assert.Equal(t, "du", out.Upstream.Username)
	assert.Equal(t, "dp", out.Upstream.Password)
}

func TestResolve_AirGapNoUpstream(t *testing.T) {
	out, err := Resolve(nil, CacheConfig{Enabled: true})
	require.NoError(t, err)
	assert.True(t, out.Enabled)
	assert.Nil(t, out.Upstream, "air-gap: no upstream → authoritative store")
}

func TestResolve_DefaultScheme(t *testing.T) {
	out, err := Resolve(&UpstreamConfig{Host: "r.example.com"}, CacheConfig{Enabled: true})
	require.NoError(t, err)
	require.NotNil(t, out.Upstream)
	assert.Equal(t, "HTTPS", out.Upstream.Scheme)
	assert.False(t, out.Upstream.HasCA)
}
