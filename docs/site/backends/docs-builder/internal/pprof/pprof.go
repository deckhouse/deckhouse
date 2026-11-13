// Copyright 2023 Flant JSC
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

//go:build pprof

// Package pprof serves the runtime profiling endpoints of net/http/pprof in
// a development build of docs-builder: images built for a merge request or a
// branch carry the pprof build tag, release images do not and have none of
// it (pprof_off.go). See modules/810-documentation/docs/internal/PROFILING.md.
package pprof

import (
	"log/slog"
	"net/http"
	httppprof "net/http/pprof"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Address is reachable from inside the pod only: kubectl port-forward.
const Address = "127.0.0.1:6060"

// NewServer returns the HTTP server of the pprof handlers on Address.
func NewServer(logger *log.Logger) *http.Server {
	logger.Info("pprof listens", slog.String("address", Address))

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", httppprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", httppprof.Trace)

	return &http.Server{
		Addr:    Address,
		Handler: mux,
	}
}
