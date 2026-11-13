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

package v1

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
)

// busyRetryIn is how many seconds a reader waits for a render slot when the
// queue of renders is full.
const busyRetryIn = 60

// OnDemand renders module versions on request.
type OnDemand struct {
	Store    *ondemand.Store
	Versions *ondemand.Versions
	// Site writes the links of the public site: no language in the path.
	Site bool
	// Ready, when set, holds /readyz back until the versions the pod serves
	// first are taken from S3.
	Ready func() bool
	// Live, when set, fails /healthz with the error it returns: a pass of
	// the pinned versions stuck past its deadline, which only a restart
	// gets going again (ondemand.PassWatch).
	Live func() error
	// Embedded, on the public site, lists the versions of a module embedded
	// into Deckhouse, which has no registry of its own.
	Embedded *ondemand.EmbeddedVersions
}

// EnableOnDemand serves the pages of module versions that nginx did not find
// on the cache volume, the render status, the list of module versions, the
// version menu and, until docs-builder builds them, the list of modules and
// the address of a module without a version.
func (h *DocsBuilderHandler) EnableOnDemand(o OnDemand) {
	h.onDemand = o

	h.mux.HandleFunc("GET /{lang}/modules/{$}", h.handleModulesList)
	h.mux.HandleFunc("GET /{lang}/modules/{module}/{$}", h.handleModuleRoot)
	h.mux.HandleFunc("GET /{lang}/modules/{module}/{page}", h.handleModuleRoot)
	h.mux.HandleFunc("GET /{lang}/modules/{module}/{version}/{path...}", h.handleVersionPage)
	h.mux.HandleFunc("GET /{lang}/modules/{module}/versions/{$}", h.handleVersionsPage)
	h.mux.HandleFunc("GET /api/v2/render/{module}/{version}", h.handleRenderStatus)
	h.mux.HandleFunc("GET /api/v2/modules/{module}/versions", h.handleVersions)
	h.mux.HandleFunc("GET /api/v2/modules/{module}/menu", h.handleMenu)
}

// handleVersionPage gets a page of a module version that is not on the cache
// volume. It starts the render and answers with the placeholder at once; the
// placeholder reloads the page, which nginx then serves from the volume.
func (h *DocsBuilderHandler) handleVersionPage(w http.ResponseWriter, r *http.Request) {
	lang, module, version := r.PathValue("lang"), r.PathValue("module"), r.PathValue("version")
	if !slices.Contains(docs.Languages, lang) {
		http.NotFound(w, r)
		return
	}

	if err := docs.ValidateModuleName(module); err != nil {
		http.NotFound(w, r)
		return
	}
	// Not a version: a page of the module, at the address that serves its
	// target.
	if docs.ValidateModuleVersion(module, version) != nil {
		page := modulePage(version, r.PathValue("path"))
		// A section of the module keeps its slash: its pages link relative to it.
		if page != "" && !strings.HasSuffix(page, "/") && strings.HasSuffix(r.URL.Path, "/") {
			page += "/"
		}
		h.handleModulePage(w, r, lang, module, page)
		return
	}

	st, err := h.start(r, module, version)
	if errors.Is(err, ondemand.ErrBusy) {
		w.Header().Set("Retry-After", strconv.Itoa(busyRetryIn))
		h.writePlaceholder(w, http.StatusServiceUnavailable, lang, module, version, ondemand.Status{Stage: ondemand.StageFailed, Error: ondemand.ReasonBusy, RetryIn: busyRetryIn})
		return
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Published while the request went from nginx to here: the placeholder
	// sees the version done and reloads the page, which nginx then serves.
	if st.Stage == ondemand.StageDone && h.onDemand.Store.HasPage(module, version, lang, r.PathValue("path")) {
		st = ondemand.Status{Stage: ondemand.StageRender}
	}

	code := http.StatusAccepted
	switch {
	case st.Stage == ondemand.StageDone:
		// The version is on the volume, so nginx did not find this page in it.
		code = http.StatusNotFound
	case st.Stage == ondemand.StageFailed && st.NotFound:
		code = http.StatusNotFound
	case st.Stage == ondemand.StageFailed:
		code = http.StatusBadGateway
		w.Header().Set("Retry-After", strconv.Itoa(st.RetryIn))
	}

	h.writePlaceholder(w, code, lang, module, version, st)
}

// start renders a version that is published in the registry. A version that
// is not in the list is not rendered: a crawler walking made-up versions would
// otherwise queue renders and send requests to the registry for each. When
// the list can not be read, the render itself finds out.
func (h *DocsBuilderHandler) start(r *http.Request, module, version string) (ondemand.Status, error) {
	// A rendered version or one being rendered needs no check; a failed one
	// may be started again, so it is checked like a new one.
	if st := h.onDemand.Store.Status(module, version); st.Stage != "" && st.Stage != ondemand.StageFailed {
		return st, nil
	}

	published, err := h.onDemand.Versions.Has(r.Context(), module, version)
	if ctxErr := r.Context().Err(); ctxErr != nil {
		// The reader is gone; nothing has been checked, so nothing starts.
		return ondemand.Status{}, ctxErr
	}
	if ondemand.Unpublished(err) || err == nil && !published {
		return ondemand.Status{Stage: ondemand.StageFailed, NotFound: true}, nil
	}

	return h.onDemand.Store.Start(module, version)
}

func (h *DocsBuilderHandler) writePlaceholder(w http.ResponseWriter, code int, lang, module, version string, st ondemand.Status) {
	h.writeHTML(w, code, "write placeholder", func(w io.Writer) error {
		return writePlaceholder(w, urls{site: h.onDemand.Site}, lang, module, version, st)
	})
}

// writeHTML answers with a page docs-builder writes itself; it is never
// cached, its state changes between requests. A failed write is logged as
// what.
func (h *DocsBuilderHandler) writeHTML(w http.ResponseWriter, code int, what string, write func(io.Writer) error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)

	if err := write(w); err != nil {
		h.logger.Warn(what, log.Err(err))
	}
}

func (h *DocsBuilderHandler) handleRenderStatus(w http.ResponseWriter, r *http.Request) {
	module, version := r.PathValue("module"), r.PathValue("version")
	if err := docs.ValidateModuleVersion(module, version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	h.writeJSON(w, h.onDemand.Store.Status(module, version))
}

func (h *DocsBuilderHandler) handleVersions(w http.ResponseWriter, r *http.Request) {
	module := r.PathValue("module")
	if err := docs.ValidateModuleName(module); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	versions, err := h.onDemand.Versions.List(r.Context(), module)
	if ondemand.Unpublished(err) {
		// On the public site: the minor versions of Deckhouse that kept the
		// documentation of an embedded module.
		var embedded []string
		if h.onDemand.Embedded != nil {
			var embeddedErr error
			if embedded, embeddedErr = h.onDemand.Embedded.List(module); embeddedErr != nil {
				h.logger.Error("read the history of the embedded modules", slog.String("module", module), log.Err(embeddedErr))
				http.Error(w, "cannot read the history of the embedded modules", http.StatusBadGateway)
				return
			}
		}
		if len(embedded) == 0 {
			http.Error(w, "module has no registry source", http.StatusNotFound)
			return
		}
		h.writeJSON(w, moduleVersions{Module: module, Versions: embedded})
		return
	}
	if err != nil {
		h.logger.Warn("list module versions", slog.String("module", module), log.Err(err))
		http.Error(w, "cannot list module versions in the registry", http.StatusBadGateway)
		return
	}

	h.writeJSON(w, moduleVersions{Module: module, Versions: versions, Incomplete: h.onDemand.Versions.Incomplete(module)})
}

// moduleVersions is the answer of the versions endpoint.
type moduleVersions struct {
	Module   string   `json:"module"`
	Versions []string `json:"versions"`
	// Incomplete marks a list that may miss published versions: the registry
	// could not be listed and the last list is served, or a source of the
	// module did not answer.
	Incomplete bool `json:"incomplete,omitempty"`
}

func (h *DocsBuilderHandler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Warn("write json", log.Err(err))
	}
}
