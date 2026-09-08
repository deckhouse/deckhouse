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
