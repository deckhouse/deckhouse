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

// Package queues serves the task queues of the package runtime.
package queues

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/respond"
)

// Provider provides task queue state to the endpoints.
type Provider interface {
	// DumpQueues returns a serialisable snapshot of the queues of one package, or
	// of every queue when name is empty.
	DumpQueues(name string) any
}

// Handler serves the queue endpoints.
type Handler struct {
	provider Provider
}

// New creates the handler on top of the runtime queues.
func New(provider Provider) *Handler {
	return &Handler{provider: provider}
}

// Routes returns the subtree to mount.
func (h *Handler) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/dump", h.dump)

	return router
}

// dump serves every queue with its tasks, or the queues of the package named by
// the name query parameter. The format query parameter selects YAML or JSON.
func (h *Handler) dump(w http.ResponseWriter, req *http.Request) {
	respond.Dump(w, req, h.provider.DumpQueues(req.URL.Query().Get("name")))
}
