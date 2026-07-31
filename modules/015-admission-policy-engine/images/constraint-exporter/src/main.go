/*
Copyright 2022 Flant JSC

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

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// flags
	listenAddress string
	metricsPath   string
	interval      time.Duration

	trackObjectsCMName string
)

func init() {
	flag.StringVar(&listenAddress, "server.telemetry-address", ":15060",
		"Address to listen on for telemetry")
	flag.StringVar(&metricsPath, "server.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	flag.DurationVar(&interval, "server.interval", 30*time.Second,
		"Kubernetes API server polling interval")
	flag.StringVar(&trackObjectsCMName, "track-objects-configmap", "constraint-exporter", "ConfigMap for export tracking resource kinds")
}

var (
	ticker *time.Ticker
	stopCh = make(chan struct{})
)

func main() {
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ns := os.Getenv("POD_NAMESPACE")
	if len(ns) == 0 {
		fatal("pod namespace is not set", nil)
	}

	exporter := NewExporter()
	err := exporter.initKindTracker(ns, trackObjectsCMName)
	if err != nil {
		fatal("init kind tracker failed", err)
	}

	clientGVR, err := exporter.createKubeClientGroupVersion()
	if err != nil {
		fatal("create kube client failed", err)
	}

	go exporter.startScheduled(clientGVR, interval)
	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.MustRegister(exporter)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("server start failed", err)
		}
	}()
	slog.Info("server started", "listen_address", listenAddress, "metrics_path", metricsPath, "interval", interval.String())

	<-signalCh
	close(stopCh)
	slog.Info("server stopping")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		fatal("server shutdown failed", err)
	}
}

func fatal(message string, err error) {
	if err != nil {
		slog.Error(message, "error", err)
	} else {
		slog.Error(message)
	}
	os.Exit(1)
}
