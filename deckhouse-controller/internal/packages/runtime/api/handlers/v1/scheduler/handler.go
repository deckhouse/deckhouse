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
// Package scheduler serves the scheduler nodes that decide package enablement.
package scheduler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api"
)

// Provider provides scheduler state to the endpoints.
type Provider interface {
	Dump() any
	// DumpByName returns nil when no scheduler node has that name.
	DumpByName(name string) any
}

// NewHandler returns the scheduler subtree, ready to mount.
func NewHandler(provider Provider) http.Handler {
	h := &handler{provider: provider}

	router := chi.NewRouter()
	router.Get("/dump", h.dump)

	return router
}

// handler serves the scheduler endpoints.
type handler struct {
	provider Provider
}

// dump serves every scheduler node, or the one named by the name query parameter.
func (h *handler) dump(w http.ResponseWriter, req *http.Request) {
	if name := req.URL.Query().Get("name"); name != "" {
		api.EncodeResponse(w, req, h.provider.DumpByName(name))

		return
	}

	api.EncodeResponse(w, req, h.provider.Dump())
}
