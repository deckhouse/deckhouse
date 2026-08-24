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

// Package handlers composes the root endpoint tree of a debug server: the
// versioned API, the process probes, pprof and route discovery.
package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/respond"
	v1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1"
)

// apiPrefix is where the versioned tree is mounted.
const apiPrefix = "/api/v1"

// NewPublicRouter composes the tree safe for a transport others can reach.
func NewPublicRouter(deps v1.Deps) chi.Router {
	return newRouter(v1.NewPublicRouter(deps))
}

// NewPrivateRouter composes the full tree, including the endpoints that expose
// package values, rendered manifests and hook snapshots.
func NewPrivateRouter(deps v1.Deps) chi.Router {
	return newRouter(v1.NewPrivateRouter(deps))
}

// newRouter mounts the versioned tree next to the routes every transport serves.
// Discovery is registered last so that it walks the complete tree.
func newRouter(api chi.Router) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	// pprof is served on every transport: profiles carry stack traces rather than
	// package values, and port-forwarding is the only practical way to point
	// `go tool pprof` at a running controller.
	router.Mount("/debug", middleware.Profiler())
	router.Mount(apiPrefix, api)

	// Probes stay unversioned so a manifest can point at them directly.
	router.Get("/healthz", alive)
	router.Get("/readyz", alive)

	router.Get("/discovery", discovery(router))

	return router
}

// alive reports that the process is up and serving.
func alive(w http.ResponseWriter, _ *http.Request) {
	respond.Text(w, "ok")
}

// discovery lists every route of the given router as plain text.
// The pprof subtree is collapsed into a single line instead of enumerated.
func discovery(router chi.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		buf := bytes.NewBuffer(nil)

		walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if strings.HasPrefix(route, "/debug/") {
				return nil
			}

			_, _ = fmt.Fprintf(buf, "%s %s\n", method, route)

			return nil
		}

		if err := chi.Walk(router, walkFn); err != nil {
			http.Error(w, fmt.Sprintf("walk routes: %v", err), http.StatusInternalServerError)
			return
		}

		buf.WriteString("GET /debug/pprof/*\n")

		respond.Text(w, buf.String())
	}
}
