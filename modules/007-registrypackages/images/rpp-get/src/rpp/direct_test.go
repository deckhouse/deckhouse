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

package rpp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitRepository(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		wantHost string
		wantPath string
	}{
		{name: "host and path", repo: "dev-registry.deckhouse.io/deckhouse/ce", wantHost: "dev-registry.deckhouse.io", wantPath: "deckhouse/ce"},
		{name: "single segment path", repo: "registry.local/repo", wantHost: "registry.local", wantPath: "repo"},
		{name: "trailing slash", repo: "registry.local/repo/", wantHost: "registry.local", wantPath: "repo"},
		{name: "host only", repo: "registry.local", wantHost: "", wantPath: ""},
		{name: "empty", repo: "", wantHost: "", wantPath: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, path := splitRepository(tt.repo)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestRegistrySchemeOrDefault(t *testing.T) {
	assert.Equal(t, "https", registrySchemeOrDefault(""))
	assert.Equal(t, "https", registrySchemeOrDefault("https"))
	assert.Equal(t, "https", registrySchemeOrDefault("HTTPS"))
	assert.Equal(t, "http", registrySchemeOrDefault("http"))
	assert.Equal(t, "http", registrySchemeOrDefault(" HTTP "))
}

func TestBuildManifestURL(t *testing.T) {
	got := buildManifestURL("https", "registry.local", "deckhouse/ce", "sha256:abc123")
	assert.Equal(t, "https://registry.local/v2/deckhouse/ce/manifests/sha256:abc123", got)
}

func TestBuildBlobURL(t *testing.T) {
	got := buildBlobURL("http", "registry.local", "deckhouse/ce", "sha256:layer1")
	assert.Equal(t, "http://registry.local/v2/deckhouse/ce/blobs/sha256:layer1", got)
}

func TestParseWWWAuthenticate(t *testing.T) {
	t.Run("full bearer challenge", func(t *testing.T) {
		realm, service, scope, ok := parseWWWAuthenticate(`Bearer realm="https://auth.local/token",service="registry.local",scope="repository:deckhouse/ce:pull"`)
		require.True(t, ok)
		assert.Equal(t, "https://auth.local/token", realm)
		assert.Equal(t, "registry.local", service)
		assert.Equal(t, "repository:deckhouse/ce:pull", scope)
	})
	t.Run("realm only", func(t *testing.T) {
		realm, service, scope, ok := parseWWWAuthenticate(`Bearer realm="https://auth.local/token"`)
		require.True(t, ok)
		assert.Equal(t, "https://auth.local/token", realm)
		assert.Equal(t, "", service)
		assert.Equal(t, "", scope)
	})
	t.Run("not bearer", func(t *testing.T) {
		_, _, _, ok := parseWWWAuthenticate(`Basic realm="x"`)
		assert.False(t, ok)
	})
	t.Run("bearer without realm", func(t *testing.T) {
		_, _, _, ok := parseWWWAuthenticate(`Bearer service="x"`)
		assert.False(t, ok)
	})
	t.Run("empty", func(t *testing.T) {
		_, _, _, ok := parseWWWAuthenticate("")
		assert.False(t, ok)
	})
}

func TestSelectLastLayerDigest(t *testing.T) {
	t.Run("multiple layers returns last", func(t *testing.T) {
		manifest := []byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[{"digest":"sha256:first"},{"digest":"sha256:last"}]}`)
		digest, err := selectLastLayerDigest(manifest)
		require.NoError(t, err)
		assert.Equal(t, "sha256:last", digest)
	})
	t.Run("index is rejected", func(t *testing.T) {
		manifest := []byte(`{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:a"}]}`)
		_, err := selectLastLayerDigest(manifest)
		require.Error(t, err)
	})
	t.Run("no layers", func(t *testing.T) {
		manifest := []byte(`{"layers":[]}`)
		_, err := selectLastLayerDigest(manifest)
		require.Error(t, err)
	})
	t.Run("empty layer digest", func(t *testing.T) {
		manifest := []byte(`{"layers":[{"digest":""}]}`)
		_, err := selectLastLayerDigest(manifest)
		require.Error(t, err)
	})
	t.Run("invalid json", func(t *testing.T) {
		_, err := selectLastLayerDigest([]byte(`not json`))
		require.Error(t, err)
	})
}

func TestBuildTokenURL(t *testing.T) {
	got, err := buildTokenURL("https://auth.local/token", "registry.local", "repository:deckhouse/ce:pull")
	require.NoError(t, err)
	assert.Equal(t, "https://auth.local/token?scope=repository%3Adeckhouse%2Fce%3Apull&service=registry.local", got)
}

func TestDirectClientGet(t *testing.T) {
	const (
		repoPath       = "deckhouse/ce"
		manifestDigest = "sha256:manifestdigest"
		layerDigest    = "sha256:layerdigest"
		blobContent    = "gzipped-tar-layer-bytes"
		basicAuth      = "dXNlcjpwYXNz" // base64("user:pass")
	)

	var serverURL string
	mux := http.NewServeMux()

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic "+basicAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"token":"bearer-xyz"}`))
	})

	mux.HandleFunc("/v2/"+repoPath+"/manifests/"+manifestDigest, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bearer-xyz" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+serverURL+`/token",service="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[{"digest":"` + layerDigest + `"}]}`))
	})

	mux.HandleFunc("/v2/"+repoPath+"/blobs/"+layerDigest, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bearer-xyz" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+serverURL+`/token",service="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(blobContent))
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	host := strings.TrimPrefix(server.URL, "http://")
	client := newDirectClient(Config{
		RegistryRepo:   host + "/" + repoPath,
		RegistryAuth:   basicAuth,
		RegistryScheme: "http",
	})

	body, source, err := client.Get(context.Background(), manifestDigest)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	if string(got) != blobContent {
		t.Errorf("body = %q, want %q", string(got), blobContent)
	}
	if source != host {
		t.Errorf("source = %q, want %q", source, host)
	}
}

func TestDirectClientReusesToken(t *testing.T) {
	const (
		repoPath    = "deckhouse/ce"
		layerDigest = "sha256:layer"
		blobContent = "blob"
		basicAuth   = "dXNlcjpwYXNz"
	)
	digests := []string{"sha256:first", "sha256:second"}

	var serverURL string
	var tokenHits int

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokenHits++
		_, _ = w.Write([]byte(`{"token":"bearer-xyz"}`))
	})
	challenge := func(w http.ResponseWriter) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+serverURL+`/token",service="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}
	for _, d := range digests {
		mux.HandleFunc("/v2/"+repoPath+"/manifests/"+d, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer bearer-xyz" {
				challenge(w)
				return
			}
			_, _ = w.Write([]byte(`{"layers":[{"digest":"` + layerDigest + `"}]}`))
		})
	}
	mux.HandleFunc("/v2/"+repoPath+"/blobs/"+layerDigest, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bearer-xyz" {
			challenge(w)
			return
		}
		_, _ = w.Write([]byte(blobContent))
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	serverURL = server.URL

	host := strings.TrimPrefix(server.URL, "http://")
	client := newDirectClient(Config{
		RegistryRepo:   host + "/" + repoPath,
		RegistryAuth:   basicAuth,
		RegistryScheme: "http",
	})

	for _, d := range digests {
		body, _, err := client.Get(context.Background(), d)
		require.NoError(t, err)
		_, _ = io.ReadAll(body)
		body.Close()
	}

	assert.Equal(t, 1, tokenHits, "token endpoint should be hit once across both Get calls")
}

func TestDirectClientGetNotFound(t *testing.T) {
	const repoPath = "deckhouse/ce"
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/"+repoPath+"/manifests/sha256:missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := newDirectClient(Config{
		RegistryRepo:   host + "/" + repoPath,
		RegistryScheme: "http",
	})

	_, _, err := client.Get(context.Background(), "sha256:missing")
	if err == nil {
		t.Fatal("Get() error = nil, want error")
	}
	if shouldRetryFetch(err) {
		t.Error("shouldRetryFetch(404) = true, want false")
	}
}
