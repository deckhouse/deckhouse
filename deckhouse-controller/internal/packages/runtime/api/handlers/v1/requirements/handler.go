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
// Package requirements serves the values package version checks are evaluated
// against.
package requirements

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/api"
)

// Source returns the current requirements values.
type Source func() map[string]any

// NewHandler returns the requirements subtree, ready to mount.
func NewHandler(source Source) http.Handler {
	h := &handler{source: source}

	router := chi.NewRouter()
	router.Get("/dump", h.dump)

	return router
}

// handler serves the requirements endpoint.
type handler struct {
	source Source
}

// dump serves the requirements values.
func (h *handler) dump(w http.ResponseWriter, req *http.Request) {
	api.EncodeResponse(w, req, h.source())
}
