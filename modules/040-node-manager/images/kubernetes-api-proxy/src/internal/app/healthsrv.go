/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"encoding/json"
	"net/http"

	"kubernetes-api-proxy/internal/upstream"
)

type healthChecker interface {
	Healthy() (bool, error)
	Nodes() ([]upstream.ExportNode, error)
}

// NewHealthServer constructs an HTTP server exposing /healthz (always OK) and
// /readyz (OK only if the load balancer reports at least one healthy upstream).
func NewHealthServer(addr string, lb healthChecker) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/upstreams", func(w http.ResponseWriter, _ *http.Request) {
		isHealthy, _ := lb.Healthy()
		nodes, _ := lb.Nodes()

		var statusMap = make(map[string]interface{})
		statusMap["isHealthy"] = isHealthy
		statusMap["nodes"] = nodes

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(statusMap)
	})

	return &http.Server{Addr: addr, Handler: mux}
}
