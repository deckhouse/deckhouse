// Copyright 2025 Flant JSC
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

package main

import (
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (a *app) startMetricsServer(metricsAddr string) {
	glog.Info("Starting prometheus metrics")

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !a.isReady.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) {
		if !a.kubeAPIOK.Load() {
			http.Error(w, "kubernetes api not reachable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("alive"))
	})

	server := &http.Server{
		Addr:              metricsAddr,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           mux,
	}

	glog.Warning(server.ListenAndServe())
}
