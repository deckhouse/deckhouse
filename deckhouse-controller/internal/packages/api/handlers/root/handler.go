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

// Package root composes the root endpoint tree of an API server: the
// versioned API, the process probes, pprof and the route listing.
package root

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"k8s.io/client-go/tools/leaderelection"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// apiPrefix is where the versioned tree is mounted.
const apiPrefix = "/api/v1"

// Deps holds the dependencies for root handler.
type Deps struct {
	Provider Provider
}

// Provider serves the root routes; GetElector is nil outside HA.
type Provider interface {
	GetMetrics() metricsstorage.Storage
	GetElector() *leaderelection.LeaderElector
	IsConverged() bool
}

type handler struct {
	provider Provider
}

// NewHandler mounts the versioned tree next to the routes every transport
// serves. Discovery is registered last so that it walks the complete tree.
func NewHandler(api http.Handler, deps Deps) http.Handler {
	h := &handler{provider: deps.Provider}

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	// pprof is served on every transport: profiles carry stack traces rather than
	// package values, and port-forwarding is the only practical way to point
	// `go tool pprof` at a running controller.
	router.Mount("/debug", middleware.Profiler())

	// Probes stay unversioned so a manifest can point at them directly.
	router.Get("/healthz", alive)
	router.Get("/readyz", h.ready)

	router.Get("/endpoints", listRoutes(router))
	router.Get("/metrics", h.provider.GetMetrics().Handler().ServeHTTP)

	router.Mount(apiPrefix, api)

	return router
}

// alive reports that the process is up and serving.
func alive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// ready reports the readiness of the controller.
func (h *handler) ready(w http.ResponseWriter, _ *http.Request) {
	elector := h.provider.GetElector()

	if elector == nil || elector.IsLeader() {
		h.reportStartupConvergeStatus(w)
		return
	}

	checkLeaderReadiness(w, elector)
}

// reportStartupConvergeStatus answers with this replica's startup converge state, as addon-operator did.
func (h *handler) reportStartupConvergeStatus(w http.ResponseWriter) {
	if !h.provider.IsConverged() {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Startup converge in progress\n"))

		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Startup converge done.\n"))
}

// checkLeaderReadiness queries the leader instance to determine overall readiness
func checkLeaderReadiness(w http.ResponseWriter, elector *leaderelection.LeaderElector) {
	leader := elector.GetLeader()
	if leader == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("HA mode is enabled but no leader is elected\n"))

		return
	}

	// Create context with timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// Query the leader's readiness
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("http://%s:%s/readyz", leader, envconfig.ListenPort()), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("HA mode is enabled but couldn't craft a request to the leader\n"))

		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("HA mode is enabled but couldn't send a request to the leader\n"))

		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("HA mode is enabled but the leader's status response code isn't OK\n"))

		return
	}

	// If leader is OK and we're not the leader, we're in a standby state which is OK
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("HA mode is enabled and waiting for acquiring the lock.\n"))
}

// listRoutes lists every route of the given router as plain text.
// The pprof subtree is collapsed into a single line instead of enumerated.
func listRoutes(router chi.Router) http.HandlerFunc {
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

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	}
}
