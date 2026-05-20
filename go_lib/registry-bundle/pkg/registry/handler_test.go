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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry/mocks"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

func positiveRepositoryPathScenarios() []string {
	return []string{
		"a/b/c/d/e/f/g/h/i/g/k/l/m/n/o/p/q/r/s/t/u/v/w/z/y/z",
		"a/a/a",
		"healthz",
		"v2",
		"_catalog",
		"v2/_catalog",
		"tags",
		"v2/tags",
		"blobs",
		"v2/blobs",
		"manifests",
		"v2/manifests",
		"referrers",
		"v2/referrers",
		"list",
		"v2/list",
		"range",
		"latest",
	}
}

func newTestHandler(reg Registry) http.Handler {
	return NewV2Handler(
		log.NewNoop(),
		reg,
	)
}

func TestHandleV2Root(t *testing.T) {
	h := newTestHandler(mocks.NopRegistry())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v2/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if v := rec.Header().Get("Docker-Distribution-API-Version"); v != "registry/2.0" {
		t.Errorf("Docker-Distribution-API-Version = %q", v)
	}
}

func TestHandleCatalog(t *testing.T) {
	newRegistry := func(repos []string) Registry {
		reg := mocks.NopRegistry()
		reg.SortedReposFunc = func() []string { return repos }
		return reg
	}

	tests := []struct {
		name       string
		registry   Registry
		request    *http.Request
		wantStatus int
		wantRepos  []string
	}{
		{
			name:       "all repos",
			registry:   newRegistry([]string{"foo", "bar"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil),
			wantStatus: http.StatusOK,
			wantRepos:  []string{"foo", "bar"},
		},
		{
			name:       "with limit",
			registry:   newRegistry([]string{"a", "b", "c"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog?n=2", nil),
			wantStatus: http.StatusOK,
			wantRepos:  []string{"a", "b"},
		},
		{
			name:       "limit larger than list",
			registry:   newRegistry([]string{"a", "b"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog?n=10", nil),
			wantStatus: http.StatusOK,
			wantRepos:  []string{"a", "b"},
		},
		{
			name:       "empty catalog",
			registry:   newRegistry(nil),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog", nil),
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid n",
			registry:   newRegistry(nil),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog?n=notanumber", nil),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative n",
			registry:   newRegistry(nil),
			request:    httptest.NewRequest(http.MethodGet, "/v2/_catalog?n=-1", nil),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.registry)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.request)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantRepos != nil {
				var got catalog
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if len(got.Repos) != len(tt.wantRepos) {
					t.Errorf("repos = %v, want %v", got.Repos, tt.wantRepos)
				}
			}
		})
	}
}

func TestHandleTags(t *testing.T) {
	newRegistry := func(inputRepo string, tags []string) Registry {
		reg := mocks.NopRegistry()
		reg.SortedTagsFunc = func(_ context.Context, reqRepo, last string) ([]string, error) {
			if inputRepo != reqRepo {
				return nil, errs.ErrUnknownRepository
			}

			slices.Sort(tags)

			if last == "" {
				return tags, nil
			}

			i, found := slices.BinarySearch(tags, last)
			if found {
				i++
			}
			return tags[i:], nil
		}
		return reg
	}

	// nolint:prealloc
	tests := []struct {
		name       string
		registry   Registry
		request    *http.Request
		wantStatus int
		wantTags   []string
	}{
		// OK
		{
			name:       "all tags",
			registry:   newRegistry("myrepo", []string{"v1.0", "v2.0"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/tags/list", nil),
			wantStatus: http.StatusOK,
			wantTags:   []string{"v1.0", "v2.0"},
		},
		{
			name:       "with limit",
			registry:   newRegistry("myrepo", []string{"v1", "v2", "v3"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/tags/list?n=2", nil),
			wantStatus: http.StatusOK,
			wantTags:   []string{"v1", "v2"},
		},
		{
			name:       "with last",
			registry:   newRegistry("myrepo", []string{"v1", "v2", "v3"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/tags/list?last=v2", nil),
			wantStatus: http.StatusOK,
			wantTags:   []string{"v3"},
		},
		// Bad
		{
			name:       "repo not found",
			registry:   newRegistry("test", []string{}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/tags/list", nil),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid n",
			registry:   newRegistry("myrepo", nil),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/tags/list?n=bad", nil),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, repo := range positiveRepositoryPathScenarios() {
		tests = append(tests, struct {
			name       string
			registry   Registry
			request    *http.Request
			wantStatus int
			wantTags   []string
		}{
			name:       "for corner cases repo name: " + repo,
			registry:   newRegistry(repo, []string{"v1.0"}),
			request:    httptest.NewRequest(http.MethodGet, "/v2/"+repo+"/tags/list", nil),
			wantStatus: http.StatusOK,
			wantTags:   []string{"v1.0"},
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.registry)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.request)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantTags != nil {
				var got listTags
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if len(got.Tags) != len(tt.wantTags) {
					t.Errorf("tags = %v, want %v", got.Tags, tt.wantTags)
				}
			}
		})
	}
}

func TestHandleManifest(t *testing.T) {
	content := `{"schemaVersion":2}`
	dgst := digest.FromString(content)
	desc := types.ShortDescriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    dgst,
		Size:      int64(len(content)),
	}

	newRegistry := func(inputRepo, inputDgst string) Registry {
		reg := mocks.NopRegistry()
		reg.ResolveFunc = func(_ context.Context, reqRepo, reqDgst string) (types.ShortDescriptor, io.ReadCloser, error) {
			if inputRepo != reqRepo {
				return types.ShortDescriptor{}, nil, errs.ErrUnknownRepository
			}
			if inputDgst != reqDgst {
				return types.ShortDescriptor{}, nil, errs.ErrManifestNotFound
			}
			return desc, io.NopCloser(strings.NewReader(content)), nil
		}
		return reg
	}

	// nolint:prealloc
	tests := []struct {
		name       string
		registry   Registry
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		// OK
		{
			name:       "Simple",
			registry:   newRegistry("myrepo", "latest"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/manifests/latest", nil),
			wantStatus: http.StatusOK,
			wantBody:   content,
		},
		{
			name:       "With digest",
			registry:   newRegistry("myrepo", dgst.String()),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/manifests/"+dgst.String(), nil),
			wantStatus: http.StatusOK,
			wantBody:   content,
		},
		{
			name:       "With method head",
			registry:   newRegistry("myrepo", dgst.String()),
			request:    httptest.NewRequest(http.MethodHead, "/v2/myrepo/manifests/"+dgst.String(), nil),
			wantStatus: http.StatusOK,
			wantBody:   "",
		},
		// Fail
		{
			name:       "Unknown tag",
			registry:   newRegistry("myrepo", "test"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/manifests/latest", nil),
			wantStatus: http.StatusNotFound,
			wantBody:   "",
		},
		{
			name:       "Unknown repo",
			registry:   newRegistry("test", "latest"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/manifests/latest", nil),
			wantStatus: http.StatusNotFound,
			wantBody:   "",
		},
	}

	for _, repo := range positiveRepositoryPathScenarios() {
		tests = append(tests, struct {
			name       string
			registry   Registry
			request    *http.Request
			wantStatus int
			wantBody   string
		}{
			name:       "for corner cases repo name: " + repo,
			registry:   newRegistry(repo, "latest"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/"+repo+"/manifests/latest", nil),
			wantStatus: http.StatusOK,
			wantBody:   content,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.registry)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.request)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantBody != "" && rec.Body.Len() != len(tt.wantBody) {
				t.Errorf("body len = %d, want %d", rec.Body.Len(), len(tt.wantBody))
			}
		})
	}
}

func TestHandleBlobHead(t *testing.T) {
	content := "1234 4321 content"
	dgst := digest.FromString(content)

	newRegistry := func(inputRepo string) Registry {
		reg := mocks.NopRegistry()
		reg.ExistsFunc = func(_ context.Context, reqRepo string, reqDgst digest.Digest) (bool, int64, error) {
			if inputRepo != reqRepo {
				return false, 0, errs.ErrUnknownRepository
			}
			if dgst != reqDgst {
				return false, 0, nil
			}
			return true, int64(len(content)), nil
		}

		reg.FetchFunc = func(_ context.Context, reqRepo string, reqDgst digest.Digest) (io.ReadCloser, error) {
			if inputRepo != reqRepo {
				return nil, errs.ErrUnknownRepository
			}
			if dgst != reqDgst {
				return nil, errs.ErrBlobNotFound
			}
			return io.NopCloser(bytes.NewReader([]byte(content))), nil
		}
		return reg
	}

	newRangeRequest := func(method, target string, body io.Reader, rangeHeader string) *http.Request {
		req := httptest.NewRequest(method, target, body)
		req.Header.Set("Range", rangeHeader)
		return req
	}

	// nolint:prealloc
	tests := []struct {
		name       string
		registry   Registry
		request    *http.Request
		wantStatus int
		wantBody   string
		wantRange  string
	}{
		// HEAD OK
		{
			name:       "blob exists",
			registry:   newRegistry("myrepo"),
			request:    httptest.NewRequest(http.MethodHead, "/v2/myrepo/blobs/"+dgst.String(), nil),
			wantStatus: http.StatusOK,
		},
		// HEAD Bad
		{
			name:       "blob not found",
			registry:   newRegistry("myrepo"),
			request:    httptest.NewRequest(http.MethodHead, "/v2/myrepo/blobs/"+digest.FromString("other content").String(), nil),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "repo not found",
			registry:   newRegistry("myrepo/test"),
			request:    httptest.NewRequest(http.MethodHead, "/v2/myrepo/blobs/"+dgst.String(), nil),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid digest",
			registry:   newRegistry("myrepo"),
			request:    httptest.NewRequest(http.MethodHead, "/v2/myrepo/blobs/notadigest", nil),
			wantStatus: http.StatusBadRequest,
		},
		// GET OK
		{
			name:       "full blob",
			registry:   newRegistry("test"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/test/blobs/"+dgst.String(), nil),
			wantStatus: http.StatusOK,
			wantBody:   content,
		},
		{
			name:       "range request",
			registry:   newRegistry("test"),
			request:    newRangeRequest(http.MethodGet, "/v2/test/blobs/"+dgst.String(), nil, "bytes=0-4"),
			wantStatus: http.StatusPartialContent,
			wantBody:   content[:5],
			wantRange:  fmt.Sprintf("bytes 0-4/%d", len(content)),
		},
		// Get Bad
		{
			name:       "range out of bounds",
			registry:   newRegistry("test"),
			request:    newRangeRequest(http.MethodGet, "/v2/test/blobs/"+dgst.String(), nil, "bytes=0-100"),
			wantStatus: http.StatusRequestedRangeNotSatisfiable,
		},
		{
			name:       "invalid range format",
			registry:   newRegistry("test"),
			request:    newRangeRequest(http.MethodGet, "/v2/test/blobs/"+dgst.String(), nil, "invalid"),
			wantStatus: http.StatusRequestedRangeNotSatisfiable,
		},
		{
			name:       "blob not found",
			registry:   newRegistry("myrepo"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/blobs/"+digest.FromString("other content").String(), nil),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "repo not found",
			registry:   newRegistry("myrepo/test"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/blobs/"+dgst.String(), nil),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid digest",
			registry:   newRegistry("myrepo"),
			request:    httptest.NewRequest(http.MethodGet, "/v2/myrepo/blobs/notadigest", nil),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, repo := range positiveRepositoryPathScenarios() {
		tests = append(tests, struct {
			name       string
			registry   Registry
			request    *http.Request
			wantStatus int
			wantBody   string
			wantRange  string
		}{
			name:       "for corner cases repo name: " + repo,
			registry:   newRegistry(repo),
			request:    httptest.NewRequest(http.MethodGet, "/v2/"+repo+"/blobs/"+dgst.String(), nil),
			wantStatus: http.StatusOK,
			wantBody:   content,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.registry)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.request)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantRange != "" && rec.Header().Get("Content-Range") != tt.wantRange {
				t.Errorf("Content-Range = %q, want %q", rec.Header().Get("Content-Range"), tt.wantRange)
			}
		})
	}
}
