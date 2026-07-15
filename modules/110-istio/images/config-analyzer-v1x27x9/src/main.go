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

// Built inside the Istio source tree during image build; excluded from the
// Deckhouse root module typecheck/lint.
//go:build deckhouse_external

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"istio.io/istio/pkg/log"
)

var (
	listenAddress  string
	metricsPath    string
	interval       time.Duration
	analysisTimeout time.Duration
	istioNamespace string
	revision       string
	allNamespaces  bool
)

func init() {
	flag.StringVar(&listenAddress, "server.telemetry-address", ":8080", "Address to listen on for Prometheus metrics")
	flag.StringVar(&metricsPath, "server.telemetry-path", "/metrics", "Path under which to expose metrics")
	flag.DurationVar(&interval, "server.interval", 5*time.Minute, "Interval between Istio configuration analysis runs")
	flag.DurationVar(&analysisTimeout, "analysis.timeout", 2*time.Minute, "Timeout for a single analysis run")
	flag.StringVar(&istioNamespace, "istio-namespace", "d8-istio", "Istio control plane namespace")
	flag.StringVar(&revision, "revision", "", "Istio revision to analyze (required)")
	flag.BoolVar(&allNamespaces, "all-namespaces", true, "Analyze all namespaces in the cluster")
}

func main() {
	flag.Parse()

	if revision == "" {
		fmt.Fprintln(os.Stderr, "revision is required")
		os.Exit(1)
	}

	log.EnableLogWithDefaultScope()

	exporter := newExporter(istioNamespace, revision, allNamespaces, analysisTimeout)
	prometheus.MustRegister(exporter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go exporter.run(ctx, interval)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Infof("istio-config-analyzer listening on %s%s", listenAddress, metricsPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("server shutdown failed: %v", err)
	}
}
