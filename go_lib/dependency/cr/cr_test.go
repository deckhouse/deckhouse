/*
Copyright 2023 Flant JSC

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

package cr

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Parse(t *testing.T) {
	tests := []string{
		"registry.deckhouse.io/deckhouse/fe",
		"registry.deckhouse.io:5123/deckhouse/fe",
		"192.168.1.1/deckhouse/fe",
		"192.168.1.1:8080/deckhouse/fe",
		"2001:db8:3333:4444:5555:6666:7777:8888/deckhouse/fe",
		"[2001:db8::1]:8080/deckhouse/fe",
		"192.168.1.1:5123/deckhouse/fe",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			u, err := parse(tt)
			if err != nil {
				t.Errorf("got error: %s", err)
			}
			if u.String() != "//"+tt {
				t.Errorf("got: %s, wanted: %s", u, tt)
			}
		})
	}
}

func Test_ReadAuthConfig(t *testing.T) {
	t.Run("host match", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8032/modules": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg, "", "")
		assert.NoError(t, err)
	})

	t.Run("path mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8032/foo/bar": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg, "", "")
		assert.NoError(t, err)
	})

	t.Run("host mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.invalid.com:8032/modules": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg, "", "")
		assert.Error(t, err)
	})

	t.Run("port mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8033/foobar": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg, "", "")
		assert.Error(t, err)
	})

	t.Run("use login/password auth", func(t *testing.T) {
		// when login is provided it should take precedence over dockerCfg
		auth, err := readAuthConfig("registry.example.com:8032/modules", "", "user", "pass")
		assert.NoError(t, err)
		if assert.NotNil(t, auth) {
			assert.Equal(t, auth.Username, "user")
			assert.Equal(t, auth.Password, "pass")
		}
	})

	t.Run("login/password override dockerCfg", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8032/modules": {
			"username": "foo",
			"password": "bar"
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		// even though dockerCfg contains credentials, login/password should be used
		auth, err := readAuthConfig("registry.example.com:8032/modules", cfg, "user", "pass")
		assert.NoError(t, err)
		if assert.NotNil(t, auth) {
			assert.Equal(t, auth.Username, "user")
			assert.Equal(t, auth.Password, "pass")
		}
	})
}

func TestClient_Image(t *testing.T) {
	// Create a test HTTP server that acts as a registry
	testImage, err := random.Image(1024, 1)
	assert.NoError(t, err)

	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic handler to simulate registry responses
		switch r.URL.Path {
		case "/v2/":
			// Registry API check
			w.WriteHeader(http.StatusOK)
		case "/v2/test/repo/manifests/latest":
			// Serving image manifest
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.WriteHeader(http.StatusOK)
			manifest, err := testImage.Manifest()
			require.NoError(t, err)
			json, err := json.Marshal(manifest)
			require.NoError(t, err)
			_, err = w.Write(json)
			require.NoError(t, err)
		case "/v2/test/repo/manifests/notfound":
			// Image not found
			w.WriteHeader(http.StatusNotFound)
		case "/v2/test/repo/manifests/unauthorized":
			// Unauthorized access
			w.WriteHeader(http.StatusUnauthorized)
		case "/v2/test/repo/manifests/timeout":
			// Simulate timeout
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			// Serving blobs - simplified for testing
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("test-blob-content"))
			require.NoError(t, err)
		}
	}))
	defer registryServer.Close()

	// Extract registry host from test server
	registryHost := registryServer.URL[7:] // remove "http://"

	// Create test auth config
	testAuth := `{"auths":{"` + registryHost + `":{"username":"testuser","password":"testpass"}}}`
	testAuthBase64 := base64.StdEncoding.EncodeToString([]byte(testAuth))

	t.Run("successful image fetch", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithUserAgent("test-agent"))

		require.NoError(t, err)

		img, err := client.Image(context.Background(), "latest")

		assert.NoError(t, err)
		assert.NotNil(t, img)

		// Verify we can get digest
		digest, err := img.Digest()
		assert.NoError(t, err)
		assert.NotEmpty(t, digest.String())
	})

	t.Run("image not found", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "notfound")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Not Found")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "unauthorized")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Unauthorized")
	})

	t.Run("invalid reference", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "in:valid:tag")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse reference")
	})

	t.Run("with custom CA", func(t *testing.T) {
		// Create a temporary CA file for testing
		tmpCA := `-----BEGIN CERTIFICATE-----
	MIIDTTCCAjWgAwIBAgIJAMVr9PAPZ0B8MA0GCSqGSIb3DQEBCwUAMD0xCzAJBgNV
	BAYTAlVTMQswCQYDVQQIDAJDQTEhMB8GA1UECgwYSW50ZXJuZXQgV2lkZ2l0cyBQ
	dHkgTHRkMB4XDTE4MDEyNTEwNDgwN1oXDTI4MDEyMzEwNDgwN1owPTELMAkGA1UE
	BhMCVVMxCzAJBgNVBAgMAkNBMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0
	eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDVvoiRRwuiRhHu
	tTGnEARy+yqgTF9XDtX6uPWk6oYk9hIp5S/uVm5H1SdS8hZvWhFWCQvNO41xBXSF
	zWVkCIUIFOUIrjbHrL53UObq7fh4kdNHZuGQHBKtUl6OGTaR1Yz1xj+ajgn663B+
	ZUOz3goT31PU6Gp7lgHdMpLDk0Mqsx/nw8RipTtzp81TLiYQAR5EG3/9+EFP0nOI
	ocW4QyxK+4/sP/COUjWFWFm3UgZCIGf9Hpn5nHCiDzQJ58JP2oOhLtBGEjpwSYcX
	7kkKQgzKnxE7Rhw1uGUKKKRyJGhCpVMXZPLXrxbseFrVijDOmKDYVQOgeu1h17k+
	TNuRzG8lAgMBAAGjUDBOMB0GA1UdDgQWBBQ49T0K4IoM0yyKz9pUPlOxl0ypkTAf
	BgNVHSMEGDAWgBQ49T0K4IoM0yyKz9pUPlOxl0ypkTAMBgNVHRMEBTADAQH/MA0G
	CSqGSIb3DQEBCwUAA4IBAQBM+JyqynpTI4pLfVsz3Iezk3FWWpawUP7l/YTv0GgV
	tTdnQDGtWlBxY3TrCfwHnH7eZ6dxJCgT4jY8B0HTzkY1YYwKTHKeYKRVR9MKd5bL
	FiGQcpS2b5pFl3fHlw/9JyTGVmvhC3WFwGK/LLxK/nYyj5yBDjUyZUFrYZSzYGSx
	lTWIANJ0GlRPn7zYc6qz67pxf48vqSYzRvSFQZBcHh2G1IYNrVvpEcIwQrEj4hDO
	XnVJCWZ55JgR1MSVCVTcB1h6AwKwLZz5ih/OEgP8IEUbUVL7kRfFJYVKCLHbJDMF
	M/XWbYyHPEEhBR6l1lqRYLNQbGQDJph8aK4AZcxz
	-----END CERTIFICATE-----`

		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithCA(tmpCA))

		require.NoError(t, err)

		img, err := client.Image(context.Background(), "latest")

		assert.NoError(t, err)
		assert.NotNil(t, img)
	})

	t.Run("with timeout", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithTimeout(1*time.Second))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "timeout")

		assert.Error(t, err)
	})

	t.Run("with overridden registry timeout from env", func(t *testing.T) {
		// Save original env
		originalTimeout := os.Getenv("REGISTRY_TIMEOUT")
		defer os.Setenv("REGISTRY_TIMEOUT", originalTimeout)

		// Set timeout through environment variable
		os.Setenv("REGISTRY_TIMEOUT", "500ms")

		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "timeout")

		assert.Error(t, err)
	})

	t.Run("with cancelled context", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = client.Image(ctx, "latest")

		assert.Error(t, err)
	})

	t.Run("context should not be canceled inside function", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithTimeout(1*time.Second))

		require.NoError(t, err)

		image, err := client.Image(context.Background(), "latest")
		assert.NoError(t, err)

		_, err = image.RawConfigFile()
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "context canceled")
	})
}

func TestClient_Digest(t *testing.T) {
	testImage, err := random.Image(1024, 1)
	require.NoError(t, err)

	rawManifest, err := testImage.RawManifest()
	require.NoError(t, err)
	wantDigest, err := testImage.Digest()
	require.NoError(t, err)
	mediaType, err := testImage.MediaType()
	require.NoError(t, err)

	// a valid but distinct digest reported by HEAD for the "multiarch" index tag
	indexDigest := "sha256:" + string(bytes.Repeat([]byte("b"), 64))

	var headCount, getCount atomic.Int64

	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)

		// "headless" simulates a registry that does not answer HEAD on a manifest,
		// forcing the GET fallback in Digest.
		case "/v2/test/repo/manifests/headless":
			if r.Method == http.MethodHead {
				headCount.Add(1)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			getCount.Add(1)
			w.Header().Set("Content-Type", string(mediaType))
			w.Header().Set("Docker-Content-Digest", wantDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rawManifest)

		case "/v2/test/repo/manifests/notfound":
			w.WriteHeader(http.StatusNotFound)

		// "multiarch" answers HEAD with an image index; GET (remote.Image) resolves it to a
		// platform child with a different digest, so Digest must not trust the HEAD result.
		case "/v2/test/repo/manifests/multiarch":
			if r.Method == http.MethodHead {
				headCount.Add(1)
				w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
				w.Header().Set("Docker-Content-Digest", indexDigest)
				w.WriteHeader(http.StatusOK)
				return
			}
			getCount.Add(1)
			w.Header().Set("Content-Type", string(mediaType))
			w.Header().Set("Docker-Content-Digest", wantDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rawManifest)

		case "/v2/test/repo/manifests/latest":
			w.Header().Set("Content-Type", string(mediaType))
			w.Header().Set("Docker-Content-Digest", wantDigest.String())
			w.Header().Set("Content-Length", fmt.Sprint(len(rawManifest)))
			if r.Method == http.MethodHead {
				headCount.Add(1)
				w.WriteHeader(http.StatusOK)
				return
			}
			getCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rawManifest)

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test-blob-content"))
		}
	}))
	defer registryServer.Close()

	registryHost := registryServer.URL[7:] // remove "http://"

	newClient := func(t *testing.T) Client {
		t.Helper()
		client, err := NewClient(registryHost+"/test/repo", WithInsecureSchema(true))
		require.NoError(t, err)
		return client
	}

	t.Run("digest is fetched with a HEAD request, without a GET", func(t *testing.T) {
		headCount.Store(0)
		getCount.Store(0)

		digest, err := newClient(t).Digest(context.Background(), "latest")
		require.NoError(t, err)

		assert.Equal(t, wantDigest.String(), digest)
		assert.Positive(t, headCount.Load(), "HEAD should be used to fetch the digest")
		assert.Zero(t, getCount.Load(), "no GET should be issued when HEAD succeeds")
	})

	t.Run("falls back to GET when the registry refuses HEAD", func(t *testing.T) {
		headCount.Store(0)
		getCount.Store(0)

		digest, err := newClient(t).Digest(context.Background(), "headless")
		require.NoError(t, err)

		assert.Equal(t, wantDigest.String(), digest)
		assert.Positive(t, headCount.Load(), "HEAD should be attempted first")
		assert.Positive(t, getCount.Load(), "GET fallback should run after HEAD is refused")
	})

	t.Run("falls back to GET when HEAD reports an index", func(t *testing.T) {
		headCount.Store(0)
		getCount.Store(0)

		digest, err := newClient(t).Digest(context.Background(), "multiarch")
		require.NoError(t, err)

		// GET resolves the index to a platform child, so Digest must return the GET
		// digest, not the index digest that HEAD advertised.
		assert.Equal(t, wantDigest.String(), digest)
		assert.NotEqual(t, indexDigest, digest)
		assert.Positive(t, headCount.Load(), "HEAD should be attempted first")
		assert.Positive(t, getCount.Load(), "GET fallback should run when HEAD reports an index")
	})

	t.Run("returns an error for a missing image", func(t *testing.T) {
		_, err := newClient(t).Digest(context.Background(), "notfound")
		require.Error(t, err)
	})
}

func TestClient_ListTags(t *testing.T) {
	// Create a test HTTP server that acts as a registry
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic handler to simulate registry responses
		switch r.URL.Path {
		case "/v2/":
			// Registry API check
			w.WriteHeader(http.StatusOK)
		case "/v2/test/repo/tags/list":
			switch {
			case r.URL.Query().Get("n") == "1":
				// Tag list with pagination
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Link", `</v2/test/repo/tags/list?n=1&last=tag1>; rel="next"`)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"name":"test/repo","tags":["tag1"]}`))
				require.NoError(t, err)
			case r.URL.Query().Get("last") == "tag1":
				// Next page of results
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"name":"test/repo","tags":["tag2","tag3"]}`))
				require.NoError(t, err)
			default:
				// Default tag list
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"name":"test/repo","tags":["latest","v1.0.0","v1.1.0"]}`))
				require.NoError(t, err)
			}
		case "/v2/empty/repo/tags/list":
			// Empty repository
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"name":"empty/repo","tags":[]}`))
			require.NoError(t, err)
		case "/v2/nonexistent/repo/tags/list":
			// Repository not found
			w.WriteHeader(http.StatusNotFound)
		case "/v2/unauthorized/repo/tags/list":
			// Unauthorized access
			w.WriteHeader(http.StatusUnauthorized)
		case "/v2/timeout/repo/tags/list":
			// Simulate timeout
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		case "/v2/malformed/repo/tags/list":
			// Malformed response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"name":"malformed/repo","tags":malformed}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer registryServer.Close()

	// Extract registry host from test server
	registryHost := registryServer.URL[7:] // remove "http://"

	// Create test auth config
	testAuth := `{"auths":{"` + registryHost + `":{"username":"testuser","password":"testpass"}}}`
	testAuthBase64 := base64.StdEncoding.EncodeToString([]byte(testAuth))

	t.Run("successful tag listing", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithUserAgent("test-agent"))

		require.NoError(t, err)

		tags, err := client.ListTags(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, tags)
		assert.ElementsMatch(t, []string{"latest", "v1.0.0", "v1.1.0"}, tags)
	})

	t.Run("empty repository", func(t *testing.T) {
		client, err := NewClient(registryHost+"/empty/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		tags, err := client.ListTags(context.Background())

		assert.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("repository not found", func(t *testing.T) {
		client, err := NewClient(registryHost+"/nonexistent/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list:")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		client, err := NewClient(registryHost+"/unauthorized/repo",
			WithInsecureSchema(true))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
	})

	t.Run("invalid repository name", func(t *testing.T) {
		_, err := NewClient("in:valid:repo",
			WithInsecureSchema(true))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse repo")
	})

	t.Run("with custom CA", func(t *testing.T) {
		// Create a temporary CA file for testing
		tmpCA := `-----BEGIN CERTIFICATE-----
MIIDTTCCAjWgAwIBAgIJAMVr9PAPZ0B8MA0GCSqGSIb3DQEBCwUAMD0xCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJDQTEhMB8GA1UECgwYSW50ZXJuZXQgV2lkZ2l0cyBQ
dHkgTHRkMB4XDTE4MDEyNTEwNDgwN1oXDTI4MDEyMzEwNDgwN1owPTELMAkGA1UE
BhMCVVMxCzAJBgNVBAgMAkNBMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0
eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDVvoiRRwuiRhHu
tTGnEARy+yqgTF9XDtX6uPWk6oYk9hIp5S/uVm5H1SdS8hZvWhFWCQvNO41xBXSF
zWVkCIUIFOUIrjbHrL53UObq7fh4kdNHZuGQHBKtUl6OGTaR1Yz1xj+ajgn663B+
ZUOz3goT31PU6Gp7lgHdMpLDk0Mqsx/nw8RipTtzp81TLiYQAR5EG3/9+EFP0nOI
ocW4QyxK+4/sP/COUjWFWFm3UgZCIGf9Hpn5nHCiDzQJ58JP2oOhLtBGEjpwSYcX
7kkKQgzKnxE7Rhw1uGUKKKRyJGhCpVMXZPLXrxbseFrVijDOmKDYVQOgeu1h17k+
TNuRzG8lAgMBAAGjUDBOMB0GA1UdDgQWBBQ49T0K4IoM0yyKz9pUPlOxl0ypkTAf
BgNVHSMEGDAWgBQ49T0K4IoM0yyKz9pUPlOxl0ypkTAMBgNVHRMEBTADAQH/MA0G
CSqGSIb3DQEBCwUAA4IBAQBM+JyqynpTI4pLfVsz3Iezk3FWWpawUP7l/YTv0GgV
tTdnQDGtWlBxY3TrCfwHnH7eZ6dxJCgT4jY8B0HTzkY1YYwKTHKeYKRVR9MKd5bL
FiGQcpS2b5pFl3fHlw/9JyTGVmvhC3WFwGK/LLxK/nYyj5yBDjUyZUFrYZSzYGSx
lTWIANJ0GlRPn7zYc6qz67pxf48vqSYzRvSFQZBcHh2G1IYNrVvpEcIwQrEj4hDO
XnVJCWZ55JgR1MSVCVTcB1h6AwKwLZz5ih/OEgP8IEUbUVL7kRfFJYVKCLHbJDMF
M/XWbYyHPEEhBR6l1lqRYLNQbGQDJph8aK4AZcxz
-----END CERTIFICATE-----`

		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithCA(tmpCA))

		require.NoError(t, err)

		tags, err := client.ListTags(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, tags)
	})

	t.Run("with timeout", func(t *testing.T) {
		client, err := NewClient(registryHost+"/timeout/repo",
			WithInsecureSchema(true),
			WithDockerCfgAuth(testAuthBase64),
			WithTimeout(1*time.Second))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
	})

	t.Run("with overridden registry timeout from env", func(t *testing.T) {
		// Save original env
		originalTimeout := os.Getenv("REGISTRY_TIMEOUT")
		defer os.Setenv("REGISTRY_TIMEOUT", originalTimeout)

		// Set timeout through environment variable
		os.Setenv("REGISTRY_TIMEOUT", "500ms")

		client, err := NewClient(registryHost+"/timeout/repo",
			WithDockerCfgAuth(testAuthBase64),
			WithInsecureSchema(true))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
	})

	t.Run("with cancelled context", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithDockerCfgAuth(testAuthBase64),
			WithInsecureSchema(true))

		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = client.ListTags(ctx)

		assert.Error(t, err)
	})
}

// errLayerRead stands in for a transient failure while reading a layer blob:
// a reset connection, an expired token, a 5xx from the registry.
var errLayerRead = errors.New("read layer blob")

// flakyReader fails every read once armed, which lets a test break a layer read
// at an exact point of the extraction.
type flakyReader struct {
	r    io.Reader
	fail *atomic.Bool
	err  error
}

func (f *flakyReader) Read(p []byte) (int, error) {
	if f.fail.Load() {
		return 0, f.err
	}

	return f.r.Read(p)
}

// testLayer builds a layer whose blob, once fail is armed, breaks with failErr:
// openErr true fails opening the blob (how net/http reports a registry that
// closed the connection before responding), false fails the read mid-stream.
func testLayer(t *testing.T, files map[string]string, fail *atomic.Bool, failErr error, openErr bool) crv1.Layer {
	t.Helper()

	data := tarArchive(t, files)
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		switch {
		case fail == nil:
			return io.NopCloser(bytes.NewReader(data)), nil
		case openErr && fail.Load():
			return nil, failErr
		default:
			return io.NopCloser(&flakyReader{r: bytes.NewReader(data), fail: fail, err: failErr}), nil
		}
	})
	require.NoError(t, err)

	return layer
}

// tarArchive builds a tar archive of the files.
func tarArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for file, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: file, Mode: 0o644, Size: int64(len(body))}))
		_, err := io.WriteString(tw, body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	return buf.Bytes()
}

// TestExtract_LayerReadFailure pins the failure mode behind silently truncated
// modules: a layer blob that fails to download must not yield an archive.
// Extract copies the image before flattening it, so the failure surfaces from
// Extract itself, and the partial copy is removed.
//
// The wrapped-io.EOF case is the trap: net/http reports a registry that closed
// the connection before responding as url.Error{Err: io.EOF} out of the blob
// open, so a guard that matches the terminal error with errors.Is takes the
// most common transient network failure for a clean end of stream.
func TestExtract_LayerReadFailure(t *testing.T) {
	tests := []struct {
		name    string
		failErr error
		openErr bool
	}{
		{name: "read fails with a plain error", failErr: errLayerRead},
		{name: "open fails with a wrapped io.EOF", failErr: fmt.Errorf("get blob: %w: %w", errLayerRead, io.EOF), openErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)

			var fail atomic.Bool

			upper := testLayer(t, map[string]string{"module.yaml": "m", "version.json": "v"}, nil, nil, false)
			lower := testLayer(t, map[string]string{"Chart.yaml": "c", "templates/deploy.yaml": "d"}, &fail, tt.failErr, tt.openErr)

			img, err := mutate.AppendLayers(empty.Image, lower, upper)
			require.NoError(t, err)

			fail.Store(true)

			rc, err := Extract(img)
			assert.ErrorIs(t, err, errLayerRead, "a broken layer must not yield an archive")
			assert.ErrorIs(t, err, ErrImageDownload)
			assert.Nil(t, rc)
			assertEmptyDir(t, tmp)
		})
	}
}

// TestCompleteReader pins how the flattened stream ends: the tar trailer is held back
// until the stream ends with the io.EOF sentinel itself, so a stream that breaks right
// after a complete-looking archive surfaces its error to a tar reader instead of io.EOF.
// A wrapped io.EOF is a failure too.
func TestCompleteReader(t *testing.T) {
	archive := tarArchive(t, map[string]string{"module.yaml": "m"})

	tests := []struct {
		name    string
		end     error
		wantErr error
	}{
		{name: "clean end", end: io.EOF, wantErr: io.EOF},
		{name: "read fails after the trailer", end: errLayerRead, wantErr: errLayerRead},
		{name: "wrapped io.EOF after the trailer", end: fmt.Errorf("get blob: %w: %w", errLayerRead, io.EOF), wantErr: errLayerRead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &completeReader{rc: io.NopCloser(io.MultiReader(bytes.NewReader(archive), iotest.ErrReader(tt.end)))}

			var err error
			tr := tar.NewReader(rc)
			for err == nil {
				_, err = tr.Next()
			}

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestExtract_DownloadsLayersOnce guards against flattening that reads every layer
// three times, twice to compute the digest and the diff ID and once more for the
// content. For a remote image each read is a download.
func TestExtract_DownloadsLayersOnce(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	var opens atomic.Int32
	data := tarArchive(t, map[string]string{"module.yaml": "m"})
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		opens.Add(1)
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	// building the layer read it to compute its digests
	opens.Store(0)

	rc, err := Extract(img)
	require.NoError(t, err)

	_, err = io.Copy(io.Discard, rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())

	assert.EqualValues(t, 1, opens.Load())
}

// TestExtract_ContentOutlivesRegistryDeadline guards against the deadline of the registry
// client cutting the content off. The deadline bounds every read of a remote image, while
// the content is read at the pace of its consumer, e.g. mkfs.erofs, after the download.
func TestExtract_ContentOutlivesRegistryDeadline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	srv := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(srv.Close)

	repo := strings.TrimPrefix(srv.URL, "http://") + "/modules/test"

	img, err := random.Image(512<<10, 2)
	require.NoError(t, err)
	ref, err := name.ParseReference(repo+":main", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, img))

	client, err := NewClient(repo, WithInsecureSchema(true), WithTimeout(2*time.Second))
	require.NoError(t, err)

	remoteImg, err := client.Image(context.Background(), "main")
	require.NoError(t, err)

	rc, err := Extract(remoteImg)
	require.NoError(t, err)

	// the consumer gets to the content after the deadline of the client
	time.Sleep(2500 * time.Millisecond)

	_, err = io.Copy(io.Discard, rc)
	require.NoError(t, err)

	require.NoError(t, rc.Close())
	assertEmptyDir(t, tmp)
}

// assertEmptyDir checks that nothing is left in dir.
func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
