/*
Copyright 2023 Flant JSC

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
	"k8s.io/klog"
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
)

func init() {
	flag.StringVar(&listenAddress, "server.telemetry-address", ":15061",
		"Address to listen on for telemetry")
	flag.StringVar(&metricsPath, "server.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	flag.DurationVar(&interval, "server.interval", 30*time.Second,
		"Kubernetes API server polling interval")
}

var (
	ticker *time.Ticker
	done   = make(chan bool)
)

func main() {
	flag.Parse()

	ns := os.Getenv("POD_NAMESPACE")
	if len(ns) == 0 {
		klog.Fatal("POD_NAMESPACE environment variable not set")
	}

	exporter := NewExporter()

	go exporter.startScheduled(interval)

	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.MustRegister(exporter)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("listen: %s\n", err)
		}
	}()
	klog.Info("Server Started")

	<-done
	klog.Info("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		klog.Fatalf("Server Shutdown Failed:%+v", err)
	}
}
