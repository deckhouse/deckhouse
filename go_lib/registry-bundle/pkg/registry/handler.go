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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/opencontainers/go-digest"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
)

var (
	paramRef    = "ref"
	paramRepo   = "repo"
	patternRef  = `(?P<` + paramRef + `>[^/]+)`
	patternRepo = `(?P<` + paramRepo + `>[^/]+(/[^/]+)*)`
)

// NewV2Handler returns an HTTP handler for the registry v2 API
func NewV2Handler(logger log.Logger, registry Registry) http.Handler {
	rh := &registryHandlers{logger: logger, registry: registry}

	h := NewRegexpHandler()
	h.Add(
		regexp.MustCompile(`^/v2/?$`),
		rh.handleV2Root,
	)
	h.Add(
		regexp.MustCompile(`^/v2/_catalog$`),
		rh.handleCatalog,
	)
	h.Add(
		regexp.MustCompile(`^/v2/`+patternRepo+`/tags/list$`),
		rh.handleTags,
	)
	h.Add(
		regexp.MustCompile(`^/v2/`+patternRepo+`/blobs/`+patternRef+`$`),
		rh.handleBlob,
	)
	h.Add(
		regexp.MustCompile(`^/v2/`+patternRepo+`/manifests/`+patternRef+`$`),
		rh.handleManifest,
	)

	h.SetDefault(rh.defaultHandler)

	return h
}

func (rh *registryHandlers) defaultHandler(w http.ResponseWriter, _ *http.Request) {
	_ = errs.
		ErrStatusMethodUnknown.
		WithStatus(http.StatusNotFound).
		Write(w)
}

func (rh *registryHandlers) handleV2Root(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (rh *registryHandlers) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		_ = errs.
			ErrStatusMethodUnknown.
			Write(w)
		return
	}

	ctx := r.Context()
	repo := RegexpParam(r, paramRepo)
	last := r.URL.Query().Get("last")

	limit := 0
	if n := r.URL.Query().Get("n"); n != "" {
		if _, err := fmt.Sscanf(n, "%d", &limit); err != nil {
			_ = errs.
				ErrStatusBadRequest.
				WithMessage(fmt.Sprintf("parsing n: %v", err)).
				Write(w)
			return
		}
		if limit < 0 {
			_ = errs.
				ErrStatusBadRequest.
				WithMessage("negative n").
				Write(w)
			return
		}
	}

	tags, err := rh.registry.SortedTags(ctx, repo, last)
	if err != nil {
		_ = errs.
			MapStatusError(err).
			Write(w)
		return
	}

	if limit > 0 && limit < len(tags) {
		tags = tags[:limit]
	}

	msg, err := json.Marshal(listTags{Name: repo, Tags: tags})
	if err != nil {
		_ = errs.
			ErrStatusInternalServerError.
			WithMessage(err.Error()).
			Write(w)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprint(len(msg)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(msg)
}

func (rh *registryHandlers) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		_ = errs.
			ErrStatusMethodUnknown.
			Write(w)
		return
	}

	limit := 0
	if n := r.URL.Query().Get("n"); n != "" {
		if _, err := fmt.Sscanf(n, "%d", &limit); err != nil {
			_ = errs.
				ErrStatusBadRequest.
				WithMessage(fmt.Sprintf("parsing n: %v", err)).
				Write(w)
			return
		}
		if limit < 0 {
			_ = errs.
				ErrStatusBadRequest.
				WithMessage("negative n").
				Write(w)
			return
		}
	}

	repos := rh.registry.SortedRepos()
	if limit > 0 && limit < len(repos) {
		repos = repos[:limit]
	}

	msg, err := json.Marshal(catalog{Repos: repos})
	if err != nil {
		_ = errs.
			ErrStatusInternalServerError.
			WithMessage(err.Error()).
			Write(w)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprint(len(msg)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(msg)
}

func (rh *registryHandlers) handleManifest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		rh.handleManifestGetHead(w, r)
		return
	case http.MethodPut, http.MethodDelete:
		_ = errs.
			ErrStatusUnsupported.
			Write(w)
		return
	}
	_ = errs.
		ErrStatusMethodUnknown.
		Write(w)
}

func (rh *registryHandlers) handleManifestGetHead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repo := RegexpParam(r, paramRepo)
	reference := RegexpParam(r, paramRef)

	desc, rc, err := rh.
		registry.
		Resolve(ctx, repo, reference)
	if err != nil {
		_ = errs.
			MapStatusError(err).
			Write(w)
		return
	}
	defer rc.Close()

	w.Header().Set("Docker-Content-Digest", desc.Digest.String())
	w.Header().Set("Content-Type", desc.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(desc.Size))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		if _, err := io.Copy(w, rc); err != nil {
			rh.logger.Errorf("manifest write error: %s", err.Error())
		}
	}
}

func (rh *registryHandlers) handleBlob(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		rh.handleBlobHead(w, r)
		return

	case http.MethodGet:
		rh.handleBlobGet(w, r)
		return

	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		_ = errs.
			ErrStatusUnsupported.
			Write(w)
		return
	}
	_ = errs.
		ErrStatusMethodUnknown.
		Write(w)
}

func (rh *registryHandlers) handleBlobHead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repo := RegexpParam(r, paramRepo)

	dgst, err := digest.Parse(RegexpParam(r, paramRef))
	if err != nil {
		_ = errs.
			ErrStatusDigestInvalid.
			Write(w)
		return
	}

	ok, size, err := rh.registry.Exists(ctx, repo, dgst)
	if err != nil {
		_ = errs.
			MapStatusError(err).
			Write(w)
		return
	}
	if !ok {
		_ = errs.
			ErrStatusBlobUnknown.
			Write(w)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.WriteHeader(http.StatusOK)
}

func (rh *registryHandlers) handleBlobGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repo := RegexpParam(r, paramRepo)

	dgst, err := digest.Parse(RegexpParam(r, paramRef))
	if err != nil {
		_ = errs.
			ErrStatusDigestInvalid.
			Write(w)
		return
	}

	ok, size, err := rh.registry.Exists(ctx, repo, dgst)
	if err != nil {
		_ = errs.
			MapStatusError(err).
			Write(w)
		return
	}
	if !ok {
		_ = errs.
			ErrStatusBlobUnknown.
			Write(w)
		return
	}

	rc, err := rh.registry.Fetch(ctx, repo, dgst)
	if err != nil {
		_ = errs.
			MapStatusError(err).
			Write(w)
		return
	}
	defer rc.Close()

	// If no range
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		w.Header().Set("Docker-Content-Digest", dgst.String())
		w.Header().Set("Content-Length", fmt.Sprint(size))
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, rc); err != nil {
			rh.logger.Errorf("blob write error: %s", err.Error())
		}
		return
	}

	// If range
	start, end, err := parseRange(rangeHeader, size)
	if err != nil {
		_ = errs.
			ErrStatusBlobUnknownRange.
			WithMessage(err.Error()).
			Write(w)
		return
	}

	contentLength := end - start + 1

	var rd io.Reader
	if ra, ok := rc.(io.ReaderAt); ok {
		rd = io.NewSectionReader(ra, start, contentLength)
	} else {
		if _, err := io.CopyN(io.Discard, rc, start); err != nil {
			_ = errs.
				ErrStatusBlobUnknownRange.
				WithMessage(fmt.Sprintf("failed to seek to %d", start)).
				Write(w)
			return
		}
		rd = io.LimitReader(rc, contentLength)
	}

	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", fmt.Sprint(contentLength))
	w.WriteHeader(http.StatusPartialContent)
	if _, err := io.Copy(w, rd); err != nil {
		rh.logger.Errorf("blob partial write error: %s", err.Error())
	}
}

type catalog struct {
	Repos []string `json:"repositories"`
}

type listTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type registryHandlers struct {
	registry Registry
	logger   log.Logger
}

func parseRange(rangeHeader string, size int64) (int64, int64, error) {
	var start, end int64
	var n int

	// bytes=100-200
	if n, _ = fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); n == 2 {
		if start >= 0 && end < size && start <= end {
			return start, end, nil
		}
		return 0, 0, fmt.Errorf("invalid range: %d-%d", start, end)
	}

	// bytes=100-
	if n, _ = fmt.Sscanf(rangeHeader, "bytes=%d-", &start); n == 1 {
		if start >= 0 && start < size {
			return start, size - 1, nil
		}
		return 0, 0, fmt.Errorf("invalid start: %d", start)
	}

	// bytes=-100
	if n, _ = fmt.Sscanf(rangeHeader, "bytes=-%d", &end); n == 1 {
		if end > 0 {
			start = max(0, size-end)
			return start, size - 1, nil
		}
		return 0, 0, fmt.Errorf("invalid suffix: %d", end)
	}

	return 0, 0, fmt.Errorf("invalid range format: %s", rangeHeader)
}
