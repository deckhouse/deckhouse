/*
Copyright 2025 Flant JSC

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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"node-group-exporter/pkg/collector"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	serverAddress = flag.String("server.exporter-address", ":9000", "Address to export prometheus metrics")
	kubeConfig    = flag.String("kube.config", "", "Path to kubeconfig (optional)")
	debug         = flag.Bool("server.debug", false, "Turn On debug")
)

func main() {
	flag.Parse()

	// Create Kubernetes client
	var config *rest.Config
	var err error

	log.SetDefaultLevel(log.LevelInfo)
	if *debug {
		log.SetDefaultLevel(log.LevelDebug)
	}

	logger := log.Default()

	if *kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		logger.Fatal("Failed to create kubernetes config:", log.Err(err))
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Failed to create kubernetes client:", log.Err(err))
	}

	// Create nodegroup collector
	nodegroupCollector, err := collector.NewNodeGroupCollector(clientset, config, logger)
	if err != nil {
		logger.Fatal("Failed to create nodegroup collector:", log.Err(err))
	}

	// Start collector
	ctx := context.Background()
	if err := nodegroupCollector.Start(ctx); err != nil {
		logger.Fatal("Failed to start collector:", log.Err(err))
	}

	// Register collector with Prometheus
	prometheus.MustRegister(nodegroupCollector)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    *serverAddress,
		Handler: mux,
	}

	go func() {
		logger.Info("Starting HTTP server on ", slog.String("Address", *serverAddress))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed: ", log.Err(err))
		}
		logger.Info("HTTP server stopped")
	}()

	logger.Info("Node group exporter is ready")
	logger.Info("Metrics available at ", slog.String("Address", *serverAddress))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Received shutdown signal")

	logger.Info("Shutting down server...")

	// Stop collector
	nodegroupCollector.Stop()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: ", log.Err(err))
	}

	logger.Info("Server exited")
}
