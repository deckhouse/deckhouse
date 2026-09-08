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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

// tagsServer serves /v2/ plus /v2/<repo>/tags/list through handler, recording
// every tags/list request URI so tests can assert on round trips and on the
// query parameters the client actually sent.
type tagsServer struct {
	addr     string
	requests *[]string
}

func newTagsServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, hit int)) *tagsServer {
	t.Helper()

	var (
		hits     int32
		requests []string
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))

			return
		}

		requests = append(requests, r.URL.RequestURI())
		handler(w, r, int(atomic.AddInt32(&hits, 1)))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &tagsServer{addr: strings.TrimPrefix(srv.URL, "http://"), requests: &requests}
}

func writeTagsPage(w http.ResponseWriter, tags []string) {
	_, _ = fmt.Fprintf(w, `{"name":"repo","tags":["%s"]}`, strings.Join(tags, `","`))
}

// linkTo sets the Link header the way an OCI registry announces a next page.
func linkTo(w http.ResponseWriter, last string) {
	w.Header().Set("Link", fmt.Sprintf(`</v2/repo/tags/list?last=%s>; rel="next"`, last))
}

func TestClient_StreamTags(t *testing.T) {
	t.Run("delivers one page per response as it arrives", func(t *testing.T) {
		pages := [][]string{{"v1", "v2"}, {"v3", "v4"}, {"v5"}}

		srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
			page := pages[hit-1]
			if hit < len(pages) {
				linkTo(w, page[len(page)-1])
			}

			writeTagsPage(w, page)
		})

		c := New(srv.addr, WithInsecure(true))

		var seen [][]string

		err := c.WithSegment("repo").StreamTags(context.Background(), func(tags []string) error {
			seen = append(seen, tags)

			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, pages, seen, "visit must be called once per page, in order")
	})

	t.Run("ErrStopStreaming stops the walk without an error", func(t *testing.T) {
		srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
			linkTo(w, fmt.Sprintf("p%d", hit))
			writeTagsPage(w, []string{fmt.Sprintf("t%d", hit)})
		})

		c := New(srv.addr, WithInsecure(true))

		var seen int

		err := c.WithSegment("repo").StreamTags(context.Background(), func([]string) error {
			seen++

			return registry.ErrStopStreaming
		})
		require.NoError(t, err, "ErrStopStreaming must not surface as a failure")
		assert.Equal(t, 1, seen)
		assert.Len(t, *srv.requests, 1, "the walk must stop instead of fetching further pages")
	})

	t.Run("an error from visit is propagated unchanged", func(t *testing.T) {
		srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			writeTagsPage(w, []string{"v1"})
		})

		c := New(srv.addr, WithInsecure(true))
		sentinel := fmt.Errorf("caller failed")

		err := c.WithSegment("repo").StreamTags(context.Background(), func([]string) error {
			return sentinel
		})
		assert.ErrorIs(t, err, sentinel)
	})
}

func TestClient_ListTags_WalksEveryPage(t *testing.T) {
	pages := [][]string{{"a", "b"}, {"c", "d"}, {"e"}}

	srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
		page := pages[hit-1]
		if hit < len(pages) {
			linkTo(w, page[len(page)-1])
		}

		writeTagsPage(w, page)
	})

	c := New(srv.addr, WithInsecure(true))

	tags, err := c.WithSegment("repo").ListTags(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, tags, "the accumulated result must be the complete list")
}

// Registries that do not implement the `n` query parameter answer 400 for it.
// The unpaginated listing must not send `n` at all, since the cursor is walked
// to the end regardless and a larger page size buys nothing.
func TestClient_ListTags_OmitsPageSizeParamWhenUnbounded(t *testing.T) {
	srv := newTagsServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.URL.Query().Get("n") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"code":"UNSUPPORTED","message":"query param n is not supported"}]}`))

			return
		}

		writeTagsPage(w, []string{"v1", "v2"})
	})

	c := New(srv.addr, WithInsecure(true))

	tags, err := c.WithSegment("repo").ListTags(context.Background())
	require.NoError(t, err, "a registry that rejects ?n= must still be listable")
	assert.Equal(t, []string{"v1", "v2"}, tags)

	for _, uri := range *srv.requests {
		assert.NotContains(t, uri, "n=", "unbounded listing must not ask for a page size")
	}
}

// WithTagsLimit is the one case where `n` is meaningful, because the caller
// explicitly asked for a single page.
func TestClient_ListTags_SendsPageSizeParamWhenLimited(t *testing.T) {
	srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		linkTo(w, "v2")
		writeTagsPage(w, []string{"v1", "v2"})
	})

	c := New(srv.addr, WithInsecure(true))

	tags, err := c.WithSegment("repo").ListTags(context.Background(), WithTagsLimit(2))
	require.NoError(t, err)
	assert.Equal(t, []string{"v1", "v2"}, tags)
	require.Len(t, *srv.requests, 1, "a limited listing is a single page, the cursor is the caller's business")
	assert.Contains(t, (*srv.requests)[0], "n=2")
}

// A registry that ignores `last` and echoes the same cursor forever would spin
// the walk indefinitely, re-delivering the same page.
func TestClient_ListTags_RefusesRepeatedCursor(t *testing.T) {
	srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		linkTo(w, "v2")
		writeTagsPage(w, []string{"v1", "v2"})
	})

	c := New(srv.addr, WithInsecure(true))

	done := make(chan error, 1)
	go func() {
		_, err := c.WithSegment("repo").ListTags(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "same pagination cursor")
		assert.LessOrEqual(t, len(*srv.requests), 3, "the walk must stop almost immediately")
	case <-time.After(10 * time.Second):
		t.Fatal("ListTags looped instead of refusing the repeated cursor")
	}
}

// The direct HTTP path has to resolve credentials itself. A client built with
// WithKeychain used to go out anonymous there, so a private registry answered
// 401 while the same client authenticated fine through remote.*.
//
// Both /v2/ and tags/list demand Basic auth here: go-containerregistry's
// transport negotiates the scheme from the ping's WWW-Authenticate challenge,
// so a ping that answers 200 would settle on "no auth" and never exercise the
// credentials at all.
func TestClient_ListTags_UsesKeychainForDirectRequests(t *testing.T) {
	const (
		user = "robot"
		pass = "s3cret"
	)

	authorized := func(r *http.Request) bool {
		u, p, ok := r.BasicAuth()

		return ok && u == user && p == pass
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			writeTagsPage(w, []string{"v1"})

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := New(strings.TrimPrefix(srv.URL, "http://"),
		WithInsecure(true),
		WithKeychain(staticKeychain{authn.FromConfig(authn.AuthConfig{Username: user, Password: pass})}),
	)

	tags, err := c.WithSegment("repo").ListTags(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"v1"}, tags)
}

type staticKeychain struct {
	auth authn.Authenticator
}

func (k staticKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return k.auth, nil
}

// ---- Link header parsing ----

// RFC 8288 permits several comma-separated values in one Link header, and a
// page in the middle of a listing may advertise both prev and next. Taking the
// first <...> followed prev and walked the listing backwards.
func TestClient_ListTags_PicksRelNextFromMultiValueLink(t *testing.T) {
	srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
		if hit == 1 {
			w.Header().Set("Link",
				`</v2/repo/tags/list?last=backwards>; rel="prev", </v2/repo/tags/list?last=v2>; rel=next`)
			writeTagsPage(w, []string{"v1", "v2"})

			return
		}

		writeTagsPage(w, []string{"v3"})
	})

	c := New(srv.addr, WithInsecure(true))

	tags, err := c.WithSegment("repo").ListTags(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"v1", "v2", "v3"}, tags)

	require.Len(t, *srv.requests, 2)
	assert.Contains(t, (*srv.requests)[1], "last=v2", "the walk must follow rel=next, not rel=prev")
}

// ---- oversized pages ----

// A registry that paginates only when asked returns the whole list at once.
// Past the buffering limit that is a hard failure, so the walk asks for
// pagination rather than giving up.
func TestClient_ListTags_AsksForPaginationWhenPageTooLarge(t *testing.T) {
	hugePage := func(w http.ResponseWriter) {
		var b strings.Builder

		b.WriteString(`{"name":"repo","tags":[`)

		for i := 0; b.Len() < maxTagsResponseBytes+(1<<20); i++ {
			if i > 0 {
				b.WriteByte(',')
			}

			fmt.Fprintf(&b, `"tag-%08d-padding-padding-padding"`, i)
		}

		b.WriteString(`]}`)
		_, _ = w.Write([]byte(b.String()))
	}

	t.Run("registry honours n on retry", func(t *testing.T) {
		srv := newTagsServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			if r.URL.Query().Get("n") == "" {
				hugePage(w)

				return
			}

			writeTagsPage(w, []string{"v1", "v2"})
		})

		c := New(srv.addr, WithInsecure(true))

		tags, err := c.WithSegment("repo").ListTags(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"v1", "v2"}, tags)
		require.Len(t, *srv.requests, 2, "one plain request, then one that demands pagination")
		assert.NotContains(t, (*srv.requests)[0], "n=")
		assert.Contains(t, (*srv.requests)[1], fmt.Sprintf("n=%d", forcedPageSize))
	})

	t.Run("registry ignores n and the size error is reported", func(t *testing.T) {
		srv := newTagsServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			hugePage(w)
		})

		c := New(srv.addr, WithInsecure(true))

		_, err := c.WithSegment("repo").ListTags(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, errPageTooLarge)
		assert.Contains(t, err.Error(), "exceeds", "the error must name the real problem, not 'unexpected EOF'")
	})
}

// ---- sentinels on real responses ----

func TestClient_ListTags_ReportsAccessDenied(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := New(strings.TrimPrefix(srv.URL, "http://"), WithInsecure(true))

	_, err := c.WithSegment("repo").ListTags(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, registry.ErrAccessDenied)
	assert.NotErrorIs(t, err, registry.ErrImageNotFound,
		"a denied request says nothing about whether the target exists")
}

// ---- catalog ----

func TestClient_ListRepositories(t *testing.T) {
	catalogServer := func(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, hit int)) (string, *[]string) {
		t.Helper()

		var (
			hits     int32
			requests []string
		)

		mux := http.NewServeMux()
		mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/_catalog") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{}"))

				return
			}

			requests = append(requests, r.URL.RequestURI())
			handler(w, r, int(atomic.AddInt32(&hits, 1)))
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		return strings.TrimPrefix(srv.URL, "http://"), &requests
	}

	t.Run("walks every page", func(t *testing.T) {
		addr, _ := catalogServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
			if hit == 1 {
				w.Header().Set("Link", `</v2/_catalog?last=b>; rel="next"`)
				_, _ = w.Write([]byte(`{"repositories":["a","b"]}`))

				return
			}

			_, _ = w.Write([]byte(`{"repositories":["c"]}`))
		})

		repos, err := New(addr, WithInsecure(true)).ListRepositories(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, repos)
	})

	t.Run("refuses a repeated cursor", func(t *testing.T) {
		addr, requests := catalogServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Link", `</v2/_catalog?last=b>; rel="next"`)
			_, _ = w.Write([]byte(`{"repositories":["a","b"]}`))
		})

		done := make(chan error, 1)
		go func() {
			_, err := New(addr, WithInsecure(true)).ListRepositories(context.Background())
			done <- err
		}()

		select {
		case err := <-done:
			require.Error(t, err)
			assert.Contains(t, err.Error(), "same pagination cursor")
			assert.LessOrEqual(t, len(*requests), 3)
		case <-time.After(10 * time.Second):
			t.Fatal("ListRepositories looped instead of refusing the repeated cursor")
		}
	})

	t.Run("unimplemented catalog is reported as such", func(t *testing.T) {
		addr, _ := catalogServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"code":"UNSUPPORTED","message":"catalog is not supported"}]}`))
		})

		_, err := New(addr, WithInsecure(true)).ListRepositories(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, registry.ErrCatalogNotSupported)
		assert.NotErrorIs(t, err, registry.ErrImageNotFound,
			"a registry without /v2/_catalog is not a missing image")
	})

	t.Run("streams one page per response", func(t *testing.T) {
		addr, _ := catalogServer(t, func(w http.ResponseWriter, _ *http.Request, hit int) {
			if hit == 1 {
				w.Header().Set("Link", `</v2/_catalog?last=b>; rel="next"`)
				_, _ = w.Write([]byte(`{"repositories":["a","b"]}`))

				return
			}

			_, _ = w.Write([]byte(`{"repositories":["c"]}`))
		})

		var seen [][]string

		err := New(addr, WithInsecure(true)).StreamRepositories(context.Background(), func(repos []string) error {
			seen = append(seen, repos)

			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, [][]string{{"a", "b"}, {"c"}}, seen)
	})
}

// ---- GetManifest platform resolution ----

func TestClient_GetManifest_Platform(t *testing.T) {
	// pushIndex publishes a two-platform index and returns the child digests.
	pushIndex := func(t *testing.T, c *Client, repo, tag string) map[string]string {
		t.Helper()

		idx := v1.ImageIndex(empty.Index)
		digests := map[string]string{}

		for _, arch := range []string{"amd64", "arm64"} {
			img, err := random.Image(64, 1)
			require.NoError(t, err)

			cfg, err := img.ConfigFile()
			require.NoError(t, err)

			cfg.OS, cfg.Architecture = "linux", arch

			img, err = mutate.ConfigFile(img, cfg)
			require.NoError(t, err)

			d, err := img.Digest()
			require.NoError(t, err)

			digests[arch] = d.String()

			idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
				Add:        img,
				Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: arch}},
			})
		}

		ref, err := name.ParseReference(c.WithSegment(repo).(*Client).buildReference(tag), name.Insecure)
		require.NoError(t, err)
		require.NoError(t, remote.WriteIndex(ref, idx))

		return digests
	}

	t.Run("without a platform the index is returned as served", func(t *testing.T) {
		_, c := newTestServer(t)
		pushIndex(t, c, "multi", "v1")

		res, err := c.WithSegment("multi").GetManifest(context.Background(), "v1")
		require.NoError(t, err)
		assert.True(t, res.GetMediaType().IsIndex(), "a multi-arch reference is an index")
	})

	t.Run("with a platform the child manifest is returned", func(t *testing.T) {
		_, c := newTestServer(t)
		digests := pushIndex(t, c, "multi", "v1")

		res, err := c.WithSegment("multi").GetManifest(context.Background(), "v1",
			WithPlatform{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}})
		require.NoError(t, err)

		assert.False(t, res.GetMediaType().IsIndex(), "the platform must resolve past the index")
		assert.Equal(t, digests["arm64"], res.GetDescriptor().GetDigest().String())

		manifest, err := res.GetManifest()
		require.NoError(t, err)
		assert.NotEmpty(t, manifest.GetLayers())
	})

	t.Run("a platform the index does not carry is not found", func(t *testing.T) {
		_, c := newTestServer(t)
		pushIndex(t, c, "multi", "v1")

		_, err := c.WithSegment("multi").GetManifest(context.Background(), "v1",
			WithPlatform{Platform: &v1.Platform{OS: "windows", Architecture: "amd64"}})
		require.Error(t, err)
		assert.ErrorIs(t, err, registry.ErrImageNotFound)
	})

	t.Run("a platform on a single-image reference changes nothing", func(t *testing.T) {
		_, c := newTestServer(t)
		pushRandomImage(t, c, "single", "v1")

		res, err := c.WithSegment("single").GetManifest(context.Background(), "v1",
			WithPlatform{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}})
		require.NoError(t, err)
		assert.False(t, res.GetMediaType().IsIndex())
	})
}
