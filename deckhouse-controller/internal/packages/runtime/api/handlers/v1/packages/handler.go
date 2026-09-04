// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Package packages serves the state of loaded packages: values, rendered
// manifests and hook snapshots. Every route here carries registry credentials
// or Secret contents, so the subtree belongs on the socket transport only.
package packages

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api"
)

// Provider provides package state to the endpoints.
type Provider interface {
	Dump() any
	// DumpByName returns nil when no package has that name.
	DumpByName(name string) any
	DumpGlobal() any
	Render(ctx context.Context, name string) (string, error)
	// Snapshots reports false when no package with that name is loaded.
	Snapshots(name string) (any, bool)
}

// NewHandler returns the package subtree, ready to mount.
func NewHandler(provider Provider) http.Handler {
	h := &handler{provider: provider}

	router := chi.NewRouter()
	router.Get("/dump", h.dump)
	router.Get("/global/dump", h.global)
	router.Get("/render/{name}", h.render)
	router.Get("/snapshots/{name}", h.snapshots)

	return router
}

// handler serves the package endpoints.
type handler struct {
	provider Provider
}

// dump serves every package, or the one named by the name query parameter.
func (h *handler) dump(w http.ResponseWriter, req *http.Request) {
	if name := req.URL.Query().Get("name"); name != "" {
		api.EncodeResponse(w, req, h.provider.DumpByName(name))

		return
	}

	api.EncodeResponse(w, req, h.provider.Dump())
}

// global serves the state of the global module.
func (h *handler) global(w http.ResponseWriter, req *http.Request) {
	api.EncodeResponse(w, req, h.provider.DumpGlobal())
}

// render serves the Helm manifests rendered for the package named in the path.
// The response is the manifest document itself, not a JSON envelope.
func (h *handler) render(w http.ResponseWriter, req *http.Request) {
	rendered, err := h.provider.Render(req.Context(), chi.URLParam(req, "name"))
	if err != nil {
		if errors.Is(err, nelm.ErrPackageNotHelm) {
			http.Error(w, "package has no Helm chart", http.StatusBadRequest)
			return
		}

		http.Error(w, fmt.Sprintf("render failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write([]byte(rendered))
}

// snapshots serves the hook snapshots of the package named in the path.
func (h *handler) snapshots(w http.ResponseWriter, req *http.Request) {
	snapshots, found := h.provider.Snapshots(chi.URLParam(req, "name"))
	if !found {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	api.EncodeResponse(w, req, snapshots)
}
