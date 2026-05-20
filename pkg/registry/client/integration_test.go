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

// Hermetic integration tests against an in-memory OCI registry
// (httptest.NewServer + go-containerregistry's pkg/registry.New()) wrapped
// in small middlewares that simulate the flows the plain handler does not
// cover on its own: bearer-auth challenges, blob-host redirects,
// cross-repository blob mounts and multi-page tag/catalog walks driven by
// Link headers.
//
// The fixtures live in one file so that "what does our client actually do
// against a registry-shaped server" stays answerable by reading one screen
// of code.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	crregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

// recordingMiddleware logs every request URL it sees and forwards to next.
// The returned slice is owned by the caller; access it under mu.
type recordingMiddleware struct {
	mu  sync.Mutex
	log []string
}

func (r *recordingMiddleware) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, req.Method+" "+req.URL.RequestURI())
}

func (r *recordingMiddleware) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.log))
	copy(out, r.log)
	return out
}

// =====================================================================
// Bearer auth challenge
// =====================================================================

// TestIntegration_BearerAuthChallenge wires our client at the in-memory
// registry through a 401-Bearer challenge: every /v2/* hit without a valid
// bearer token is rejected with WWW-Authenticate pointing at a separate
// token issuer. The issuer accepts the client's Basic credentials and mints
// the expected token.
//
// This exercises the upstream ping → realm-fetch → retry-with-bearer flow
// end-to-end and proves WithLoginPassword wires its credentials all the way
// to a real challenge.
func TestIntegration_BearerAuthChallenge(t *testing.T) {
	const (
		user  = "alice"
		pass  = "s3cret"
		token = "fake-bearer-token-xyz"
	)

	// Token issuer: GET realm with Basic auth, returns JSON {token: ...}.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":        token,
			"access_token": token,
		})
	}))
	t.Cleanup(tokenSrv.Close)

	// Registry: 401-Bearer challenge until the right token shows up.
	backend := crregistry.New()
	var anonHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			atomic.AddInt32(&anonHits, 1)
			w.Header().Set("WWW-Authenticate",
				fmt.Sprintf(`Bearer realm="%s",service="registry"`, tokenSrv.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true), WithLoginPassword(user, pass))

	t.Run("push survives the challenge", func(t *testing.T) {
		img, err := random.Image(512, 1)
		require.NoError(t, err)
		_, err = c.WithSegment("alice/repo").Push(context.Background(), "v1", img)
		require.NoError(t, err)
	})

	t.Run("subsequent reads still authenticate", func(t *testing.T) {
		tags, err := c.WithSegment("alice/repo").ListTags(context.Background())
		require.NoError(t, err)
		assert.Contains(t, tags, "v1")

		exists, err := c.WithSegment("alice/repo").ImageExists(context.Background(), "v1")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	// The middleware MUST have rejected at least one unauthenticated request
	// (the initial ping) for the challenge flow to have run at all.
	assert.GreaterOrEqual(t, int(atomic.LoadInt32(&anonHits)), 1,
		"expected at least one 401 challenge")
}

// TestIntegration_BearerAuthChallenge_BadCreds documents the negative case:
// wrong password ⇒ token issuer 401s ⇒ original request stays unauthorized
// ⇒ surface an error (not a silent empty result).
func TestIntegration_BearerAuthChallenge_BadCreds(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(tokenSrv.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer realm="%s",service="registry"`, tokenSrv.URL))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true), WithLoginPassword("bob", "wrong"))

	_, err := c.WithSegment("repo").ListTags(context.Background())
	require.Error(t, err, "bad creds must surface as an error")
}

// =====================================================================
// Redirect for blob fetches
// =====================================================================

// TestIntegration_BlobRedirect simulates an S3-style storage backend: blob
// GETs from /v2/.../blobs/sha256:... return a 307 pointing the client at
// itself with ?redirected=1, where the request is then served by the
// backing registry. This proves our client follows redirects without losing
// auth or context.
func TestIntegration_BlobRedirect(t *testing.T) {
	backend := crregistry.New()
	var redirects atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// On GET .../blobs/sha256:... with no redirect marker, bounce once.
		if r.Method == http.MethodGet &&
			strings.Contains(r.URL.Path, "/blobs/") &&
			r.URL.Query().Get("redirected") == "" {
			redirects.Add(1)
			loc := r.URL.Path + "?redirected=1"
			http.Redirect(w, r, loc, http.StatusTemporaryRedirect)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true))

	pushed := pushRandomImage(t, c, "repo", "v1")

	// Fetching the config file forces a blob GET, which is exactly the
	// branch we redirect.
	img, err := c.WithSegment("repo").GetImage(context.Background(), "v1")
	require.NoError(t, err)
	cfg, err := img.RawConfigFile()
	require.NoError(t, err)
	require.NotEmpty(t, cfg, "config blob must come back non-empty after redirect")

	// Sanity: the redirect actually fired.
	assert.GreaterOrEqual(t, redirects.Load(), int32(1),
		"expected at least one blob redirect")

	// And the digest still matches the source, so the redirect chain
	// did not silently corrupt anything.
	gotDigest, err := img.Digest()
	require.NoError(t, err)
	wantDigest, err := pushed.Digest()
	require.NoError(t, err)
	assert.Equal(t, wantDigest, gotDigest)
}

// =====================================================================
// Cross-repository blob mount
// =====================================================================

// TestIntegration_CopyImage_AttemptsCrossRepoMount asserts that CopyImage
// across two repos on the same host issues the cross-repository mount POST
// per blob:
//
//	POST /v2/<dst>/blobs/uploads/?mount=<digest>&from=<src-repo>
//
// The in-memory crregistry stores blobs in a content-addressed map shared
// across repos, so HEAD on /v2/dst/repo/blobs/<digest> returns 200 even
// before the dst repo has ever been touched, which lets the upstream
// uploader skip both upload AND mount. To exercise the mount branch we
// force HEADs for the dst repo's blobs to 404 - mimicking a registry that
// keeps blob storage per-repository.
func TestIntegration_CopyImage_AttemptsCrossRepoMount(t *testing.T) {
	rec := &recordingMiddleware{}
	backend := crregistry.New()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)

		// Force a "blob unknown" answer when the client probes whether a
		// blob already exists in the destination repo. Without this the
		// uploader sees a 200 from the shared content store and skips
		// the mount POST entirely.
		if r.Method == http.MethodHead &&
			strings.HasPrefix(r.URL.Path, "/v2/dst/repo/blobs/sha256:") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true))

	img, err := random.Image(512, 2) // 2 layers => 2 mount candidates
	require.NoError(t, err)
	_, err = c.WithSegment("src/repo").Push(context.Background(), "v1", img)
	require.NoError(t, err)

	require.NoError(t, c.WithSegment("src/repo").CopyImage(
		context.Background(), "v1",
		c.WithSegment("dst/repo"), "v1",
	))

	// Look for at least one cross-repo mount POST whose ?from= references
	// the source repo. Query-escaping turns "src/repo" into "src%2Frepo".
	requests := rec.snapshot()
	var found bool
	for _, req := range requests {
		if strings.Contains(req, "POST /v2/dst/repo/blobs/uploads/") &&
			strings.Contains(req, "mount=sha256") &&
			(strings.Contains(req, "from=src/repo") || strings.Contains(req, "from=src%2Frepo")) {
			found = true
			break
		}
	}
	if !found {
		// Surface the actual request log so future regressions are easy
		// to diagnose without rerunning under -v.
		t.Fatalf("no cross-repo mount POST observed; client must use ?mount=&from=. requests:\n  %s",
			strings.Join(requests, "\n  "))
	}
}

// =====================================================================
// Server-side pagination via Link header
// =====================================================================

// TestIntegration_WalkTags_FollowsLinkHeader stands up a custom handler
// that paginates /v2/<repo>/tags/list across multiple pages, emitting
// `Link: <next>; rel="next"` until the last one. The upstream puller is
// the one that has to follow Link; our WalkTags/ListTags must walk every
// page transparently.
func TestIntegration_WalkTags_FollowsLinkHeader(t *testing.T) {
	allTags := []string{"a", "b", "c", "d", "e", "f", "g"}
	const pageSize = 3

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/tags/list"):
			hits.Add(1)
			start := 0
			if last := r.URL.Query().Get("last"); last != "" {
				start = len(allTags)
				for i, t := range allTags {
					if t > last {
						start = i
						break
					}
				}
			}
			end := start + pageSize
			if end > len(allTags) {
				end = len(allTags)
			}
			page := allTags[start:end]

			// Emit Link to the next page when there is more after this slice.
			if end < len(allTags) {
				w.Header().Set("Link",
					fmt.Sprintf(`<%s?n=%d&last=%s>; rel="next"`, r.URL.Path, pageSize, page[len(page)-1]))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "repo",
				"tags": page,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true))

	t.Run("WalkTags visits every page", func(t *testing.T) {
		var (
			got        []string
			visitCalls int
		)
		err := c.WithSegment("repo").WalkTags(context.Background(),
			func(page []string) error {
				visitCalls++
				got = append(got, page...)
				return nil
			})
		require.NoError(t, err)
		assert.Equal(t, allTags, got, "must walk all pages and produce sorted tags")
		assert.GreaterOrEqual(t, visitCalls, 2, "Link header must have produced >=2 pages")
	})

	t.Run("ListTags accumulates the same set", func(t *testing.T) {
		got, err := c.WithSegment("repo").ListTags(context.Background())
		require.NoError(t, err)
		assert.Equal(t, allTags, got)
	})

	// At least 2 hits per ListTags/WalkTags (== 4 minimum across the two
	// subtests). Tighter than "> 0", looser than an exact count so the
	// test stays robust against the puller batching reads.
	assert.GreaterOrEqual(t, hits.Load(), int32(4), "expected multi-page traversal")
}

// =====================================================================
// Server-side pagination of /_catalog
// =====================================================================

// TestIntegration_WalkRepositories_FollowsLinkHeader mirrors WalkTags for
// the catalog endpoint: a custom handler emits multi-page repos with
// Link headers and our WalkRepositories must walk every page.
func TestIntegration_WalkRepositories_FollowsLinkHeader(t *testing.T) {
	allRepos := []string{"org/a", "org/b", "org/c", "org/d", "org/e"}
	const pageSize = 2

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/v2/_catalog":
			start := 0
			if last := r.URL.Query().Get("last"); last != "" {
				start = len(allRepos)
				for i, repo := range allRepos {
					if repo > last {
						start = i
						break
					}
				}
			}
			end := start + pageSize
			if end > len(allRepos) {
				end = len(allRepos)
			}
			page := allRepos[start:end]

			if end < len(allRepos) {
				w.Header().Set("Link",
					fmt.Sprintf(`<%s?n=%d&last=%s>; rel="next"`, r.URL.Path, pageSize, page[len(page)-1]))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"repositories": page,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	addr := strings.TrimPrefix(srv.URL, "http://")
	c := New(addr, WithInsecure(true))

	repos, err := c.ListRepositories(context.Background())
	require.NoError(t, err)
	assert.Equal(t, allRepos, repos, "must walk all _catalog pages")
}
