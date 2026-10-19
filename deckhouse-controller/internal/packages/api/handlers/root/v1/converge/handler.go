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

// Package converge serves what the convergence of modules and Global waits for.
package converge

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api"
)

// Provider provides the convergence state to the endpoint.
type Provider interface {
	Converge() any
}

// NewHandler returns the converge subtree, ready to mount.
func NewHandler(provider Provider) http.Handler {
	h := &handler{provider: provider}

	router := chi.NewRouter()
	router.Get("/", h.converge)

	return router
}

type handler struct {
	provider Provider
}

func (h *handler) converge(w http.ResponseWriter, req *http.Request) {
	api.EncodeResponse(w, req, h.provider.Converge())
}
