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

// Package scheduler serves the scheduler nodes that decide package enablement.
package scheduler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/respond"
)

// Provider provides scheduler state to the endpoints.
type Provider interface {
	Dump() []byte
	// DumpByName returns nil when no scheduler node has that name.
	DumpByName(name string) []byte
}

// Handler serves the scheduler endpoints.
type Handler struct {
	provider Provider
}

// New creates the handler on top of the scheduler.
func New(provider Provider) *Handler {
	return &Handler{provider: provider}
}

// Routes returns the subtree to mount.
func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/dump", h.dump)

	return router
}

// dump serves every scheduler node, or the one named by the name query parameter.
func (h *Handler) dump(w http.ResponseWriter, req *http.Request) {
	if name := req.URL.Query().Get("name"); name != "" {
		respond.YAML(w, h.provider.DumpByName(name))
		return
	}

	respond.YAML(w, h.provider.Dump())
}
