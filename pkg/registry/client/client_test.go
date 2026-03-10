// Copyright 2025 Flant JSC
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

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	crregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientWithOptions_InsecureFlag(t *testing.T) {
	t.Run("Insecure=true sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set")
		assert.True(t, opts.Insecure, "opts.Insecure should remain true")
	})

	t.Run("Insecure=false keeps secure mode", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should not have insecure flag set")
		assert.False(t, opts.Insecure, "opts.Insecure should remain false")
	})

	t.Run("Scheme=http sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Scheme: "http",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when Scheme=http")
		assert.True(t, opts.Insecure, "opts.Insecure should be set to true when Scheme=http")
	})

	t.Run("Scheme=HTTP (uppercase) sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Scheme: "HTTP",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when Scheme=HTTP")
		assert.True(t, opts.Insecure, "opts.Insecure should be set to true when Scheme=HTTP")
	})

	t.Run("Scheme=https keeps secure mode", func(t *testing.T) {
		opts := &Options{
			Scheme: "https",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should not have insecure flag set when Scheme=https")
		assert.False(t, opts.Insecure, "opts.Insecure should remain false when Scheme=https")
	})

	t.Run("Insecure=true with Scheme=https keeps insecure", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
			Scheme:   "https",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when explicitly set")
		assert.True(t, opts.Insecure, "opts.Insecure should remain true")
	})

	t.Run("Default (no flags) uses secure mode", func(t *testing.T) {
		opts := &Options{}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should default to secure mode")
		assert.False(t, opts.Insecure, "opts.Insecure should default to false")
	})
}

func TestClient_NameOptions(t *testing.T) {
	t.Run("insecure client returns name.Insecure option", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		require.Len(t, nameOpts, 1, "should return one name option")

		// Verify the option works by parsing a reference
		ref, err := name.ParseReference("registry.example.com/repo:tag", nameOpts...)
		require.NoError(t, err)
		assert.Equal(t, "http", ref.Context().Registry.Scheme(), "should use HTTP scheme")
	})

	t.Run("secure client returns no options", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		assert.Nil(t, nameOpts, "should return nil for secure client")

		// Verify default behavior uses HTTPS
		ref, err := name.ParseReference("registry.example.com/repo:tag")
		require.NoError(t, err)
		assert.Equal(t, "https", ref.Context().Registry.Scheme(), "should use HTTPS scheme by default")
	})
}

func TestClient_WithSegment_PreservesInsecure(t *testing.T) {
	t.Run("WithSegment preserves insecure flag", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		segmentedClient := client.WithSegment("deckhouse", "ee")

		assert.True(t, segmentedClient.insecure, "WithSegment should preserve insecure flag")
		assert.Equal(t, "registry.example.com/deckhouse/ee", segmentedClient.GetRegistry())
	})

	t.Run("WithSegment preserves secure mode", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		segmentedClient := client.WithSegment("deckhouse")

		assert.False(t, segmentedClient.insecure, "WithSegment should preserve secure mode")
	})
}

func TestClient_ParseReference_UsesInsecureOption(t *testing.T) {
	t.Run("insecure client parses references with HTTP scheme", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("localhost:5000", opts)

		// Test that nameOptions returns the correct option
		nameOpts := client.nameOptions()
		ref, err := name.ParseReference("localhost:5000/repo:tag", nameOpts...)
		require.NoError(t, err)

		assert.Equal(t, "http", ref.Context().Registry.Scheme(), "should parse with HTTP scheme")
		assert.Equal(t, "localhost:5000", ref.Context().RegistryStr())
	})

	t.Run("secure client parses references with HTTPS scheme", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		ref, err := name.ParseReference("registry.example.com/repo:tag", nameOpts...)
		require.NoError(t, err)

		assert.Equal(t, "https", ref.Context().Registry.Scheme(), "should parse with HTTPS scheme")
	})
}

// ---- helpers for integration tests ----

// newTestServer starts an in-memory registry server and returns the host address
// (without scheme) and a pre-configured insecure Client pointing at it.
func newTestServer(t *testing.T) (addr string, c *Client) {
	t.Helper()
	h := crregistry.New()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	addr = strings.TrimPrefix(srv.URL, "http://")
	c = New(addr, WithInsecure())
	return addr, c
}

// pushRandomImage pushes a single-layer random image to repo:tag and returns it.
func pushRandomImage(t *testing.T, c *Client, repo, tag string) v1.Image {
	t.Helper()
	img, err := random.Image(512, 1)
	require.NoError(t, err)
	require.NoError(t, c.WithSegment(repo).PushImage(context.Background(), tag, img))
	return img
}

// ---- PushImage ----

func TestClient_PushImage(t *testing.T) {
	t.Run("pushed image is fetchable by digest", func(t *testing.T) {
		_, c := newTestServer(t)
		img := pushRandomImage(t, c, "myrepo", "v1")

		wantDigest, err := img.Digest()
		require.NoError(t, err)

		got, err := c.WithSegment("myrepo").GetDigest(context.Background(), "v1")
		require.NoError(t, err)
		assert.Equal(t, wantDigest, *got)
	})
}

// ---- GetDigest ----

func TestClient_GetDigest(t *testing.T) {
	t.Run("returns correct digest for existing tag", func(t *testing.T) {
		_, c := newTestServer(t)
		img := pushRandomImage(t, c, "repo", "latest")

		want, err := img.Digest()
		require.NoError(t, err)

		got, err := c.WithSegment("repo").GetDigest(context.Background(), "latest")
		require.NoError(t, err)
		assert.Equal(t, want, *got)
	})

	t.Run("returns error for missing tag", func(t *testing.T) {
		_, c := newTestServer(t)
		_, err := c.WithSegment("repo").GetDigest(context.Background(), "nonexistent")
		require.Error(t, err)
	})
}

// ---- GetManifest ----

func TestClient_GetManifest(t *testing.T) {
	t.Run("returns non-empty manifest for existing tag", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "v1")

		result, err := c.WithSegment("repo").GetManifest(context.Background(), "v1")
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.NotEmpty(t, result.GetMediaType())

		m, err := result.GetManifest()
		require.NoError(t, err)
		assert.NotEmpty(t, m.GetLayers())
	})

	t.Run("returns error for missing tag", func(t *testing.T) {
		_, c := newTestServer(t)
		_, err := c.WithSegment("repo").GetManifest(context.Background(), "nonexistent")
		require.Error(t, err)
	})
}

// ---- GetImage ----

func TestClient_GetImage(t *testing.T) {
	t.Run("returned image digest matches pushed image", func(t *testing.T) {
		_, c := newTestServer(t)
		pushed := pushRandomImage(t, c, "repo", "v1")

		wantDigest, err := pushed.Digest()
		require.NoError(t, err)

		img, err := c.WithSegment("repo").GetImage(context.Background(), "v1")
		require.NoError(t, err)

		gotDigest, err := img.Digest()
		require.NoError(t, err)
		assert.Equal(t, wantDigest, gotDigest)
	})

	t.Run("returns ErrImageNotFound for missing tag", func(t *testing.T) {
		_, c := newTestServer(t)
		_, err := c.WithSegment("repo").GetImage(context.Background(), "missing")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrImageNotFound))
	})
}

// ---- GetImageConfig ----

func TestClient_GetImageConfig(t *testing.T) {
	t.Run("returns config with expected label", func(t *testing.T) {
		_, c := newTestServer(t)

		base, err := random.Image(512, 1)
		require.NoError(t, err)
		cf, err := base.ConfigFile()
		require.NoError(t, err)
		cf.Config.Labels = map[string]string{"app": "test-label"}
		labeled, err := mutate.ConfigFile(base, cf)
		require.NoError(t, err)

		require.NoError(t, c.WithSegment("repo").PushImage(context.Background(), "v1", labeled))

		config, err := c.WithSegment("repo").GetImageConfig(context.Background(), "v1")
		require.NoError(t, err)
		assert.Equal(t, "test-label", config.Config.Labels["app"])
	})
}

// ---- CheckImageExists ----

func TestClient_CheckImageExists(t *testing.T) {
	t.Run("returns nil for existing image", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "v1")

		err := c.WithSegment("repo").CheckImageExists(context.Background(), "v1")
		assert.NoError(t, err)
	})

	t.Run("returns ErrImageNotFound for missing image", func(t *testing.T) {
		_, c := newTestServer(t)

		err := c.WithSegment("repo").CheckImageExists(context.Background(), "missing")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrImageNotFound))
	})
}

// ---- ListTags ----

func TestClient_ListTags(t *testing.T) {
	t.Run("returns all pushed tags", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "v1")
		pushRandomImage(t, c, "repo", "v2")
		pushRandomImage(t, c, "repo", "latest")

		tags, err := c.WithSegment("repo").ListTags(context.Background())
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"v1", "v2", "latest"}, tags)
	})

	t.Run("limit caps the number of returned tags", func(t *testing.T) {
		_, c := newTestServer(t)
		for _, tag := range []string{"a", "b", "c", "d"} {
			pushRandomImage(t, c, "paged", tag)
		}

		tags, err := c.WithSegment("paged").ListTags(context.Background(), WithTagsLimit(2))
		require.NoError(t, err)
		assert.Len(t, tags, 2)
	})
}

// ---- DeleteTag ----

func TestClient_DeleteTag(t *testing.T) {
	t.Run("tag no longer exists after deletion", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "to-delete")

		require.NoError(t, c.WithSegment("repo").DeleteTag(context.Background(), "to-delete"))

		err := c.WithSegment("repo").CheckImageExists(context.Background(), "to-delete")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrImageNotFound))
	})

	t.Run("deleting non-existent tag returns ErrImageNotFound", func(t *testing.T) {
		_, c := newTestServer(t)

		err := c.WithSegment("repo").DeleteTag(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrImageNotFound))
	})

	t.Run("other tags are unaffected by deletion", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "keep")
		pushRandomImage(t, c, "repo", "remove")

		require.NoError(t, c.WithSegment("repo").DeleteTag(context.Background(), "remove"))

		assert.NoError(t, c.WithSegment("repo").CheckImageExists(context.Background(), "keep"))
	})
}

// ---- TagImage ----

func TestClient_TagImage(t *testing.T) {
	t.Run("both tags point to the same digest after retag", func(t *testing.T) {
		_, c := newTestServer(t)
		pushed := pushRandomImage(t, c, "repo", "v1")

		wantDigest, err := pushed.Digest()
		require.NoError(t, err)

		require.NoError(t, c.WithSegment("repo").TagImage(context.Background(), "v1", "latest"))

		v1Digest, err := c.WithSegment("repo").GetDigest(context.Background(), "v1")
		require.NoError(t, err)

		latestDigest, err := c.WithSegment("repo").GetDigest(context.Background(), "latest")
		require.NoError(t, err)

		assert.Equal(t, wantDigest, *v1Digest)
		assert.Equal(t, *v1Digest, *latestDigest)
	})

	t.Run("source tag still exists after retag", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "repo", "src")

		require.NoError(t, c.WithSegment("repo").TagImage(context.Background(), "src", "dst"))

		assert.NoError(t, c.WithSegment("repo").CheckImageExists(context.Background(), "src"))
		assert.NoError(t, c.WithSegment("repo").CheckImageExists(context.Background(), "dst"))
	})

	t.Run("missing source returns ErrImageNotFound", func(t *testing.T) {
		_, c := newTestServer(t)

		err := c.WithSegment("repo").TagImage(context.Background(), "missing", "newtag")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrImageNotFound))
	})
}

// ---- isNotFound ----

func TestIsNotFound(t *testing.T) {
	t.Run("true for HTTP 404 transport error", func(t *testing.T) {
		err := &transport.Error{StatusCode: http.StatusNotFound}
		assert.True(t, isNotFound(err))
	})

	t.Run("false for HTTP 403 transport error", func(t *testing.T) {
		err := &transport.Error{StatusCode: http.StatusForbidden}
		assert.False(t, isNotFound(err))
	})

	t.Run("false for non-transport error", func(t *testing.T) {
		assert.False(t, isNotFound(errors.New("generic error")))
	})

	t.Run("false for nil", func(t *testing.T) {
		assert.False(t, isNotFound(nil))
	})

	t.Run("true for wrapped 404 transport error", func(t *testing.T) {
		inner := &transport.Error{StatusCode: http.StatusNotFound}
		wrapped := fmt.Errorf("outer: %w", inner)
		assert.True(t, isNotFound(wrapped))
	})
}
