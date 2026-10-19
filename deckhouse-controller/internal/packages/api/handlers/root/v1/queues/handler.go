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
// Package queues serves the hook lanes of the package engine (the queues of the hooks).
package queues

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api"
)

// Provider provides the hook lanes' state to the endpoints.
type Provider interface {
	// DumpQueues returns the lanes of one package, or of every package when name is empty.
	DumpQueues(name string) any
}

// NewHandler returns the queue subtree, ready to mount.
func NewHandler(provider Provider) http.Handler {
	h := &handler{provider: provider}

	router := chi.NewRouter()
	router.Get("/dump", h.dump)

	return router
}

// handler serves the queue endpoints.
type handler struct {
	provider Provider
}

// dump serves every lane with its state, or the lanes of the package named by the
// name query parameter.
func (h *handler) dump(w http.ResponseWriter, req *http.Request) {
	api.EncodeResponse(w, req, h.provider.DumpQueues(req.URL.Query().Get("name")))
}
