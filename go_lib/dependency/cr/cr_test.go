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
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/random"
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
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
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
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
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
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
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
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
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
			WithAuth(testAuthBase64),
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
			WithAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "notfound")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Not Found")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithAuth(""))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "unauthorized")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Unauthorized")
	})

	t.Run("invalid reference", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64))

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
			WithAuth(testAuthBase64),
			WithCA(tmpCA))

		require.NoError(t, err)

		img, err := client.Image(context.Background(), "latest")

		assert.NoError(t, err)
		assert.NotNil(t, img)
	})

	t.Run("with timeout", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64),
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
			WithAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.Image(context.Background(), "timeout")

		assert.Error(t, err)
	})

	t.Run("with cancelled context", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64))

		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = client.Image(ctx, "latest")

		assert.Error(t, err)
	})

	t.Run("context should not be canceled inside function", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64),
			WithTimeout(1*time.Second))

		require.NoError(t, err)

		image, err := client.Image(context.Background(), "latest")
		assert.NoError(t, err)

		_, err = image.RawConfigFile()
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "context canceled")
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
			WithAuth(testAuthBase64),
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
			WithAuth(testAuthBase64))

		require.NoError(t, err)

		tags, err := client.ListTags(context.Background())

		assert.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("repository not found", func(t *testing.T) {
		client, err := NewClient(registryHost+"/nonexistent/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list:")
	})

	t.Run("unauthorized access", func(t *testing.T) {
		client, err := NewClient(registryHost+"/unauthorized/repo",
			WithInsecureSchema(true),
			WithAuth(""))

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
			WithAuth(testAuthBase64),
			WithCA(tmpCA))

		require.NoError(t, err)

		tags, err := client.ListTags(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, tags)
	})

	t.Run("with timeout", func(t *testing.T) {
		client, err := NewClient(registryHost+"/timeout/repo",
			WithInsecureSchema(true),
			WithAuth(testAuthBase64),
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
			WithAuth(testAuthBase64),
			WithInsecureSchema(true))

		require.NoError(t, err)

		_, err = client.ListTags(context.Background())

		assert.Error(t, err)
	})

	t.Run("with cancelled context", func(t *testing.T) {
		client, err := NewClient(registryHost+"/test/repo",
			WithAuth(testAuthBase64),
			WithInsecureSchema(true))

		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = client.ListTags(ctx)

		assert.Error(t, err)
	})
}
