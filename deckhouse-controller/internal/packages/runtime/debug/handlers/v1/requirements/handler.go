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

// Package requirements serves the values package version checks are evaluated
// against.
package requirements

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/respond"
)

// Source returns the current requirements values.
type Source func() map[string]any

// Handler serves the requirements endpoint.
type Handler struct {
	source Source
}

// New creates the handler on top of the requirements values.
func New(source Source) *Handler {
	return &Handler{source: source}
}

// Routes returns the subtree to mount.
func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/dump", h.dump)

	return router
}

// dump serves the requirements values.
func (h *Handler) dump(w http.ResponseWriter, _ *http.Request) {
	respond.YAMLValue(w, h.source())
}
