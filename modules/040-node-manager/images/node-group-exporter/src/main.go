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
	"node-group-exporter/pkg/logger"
)

var (
	serverAddress = flag.String("server.exporter-address", ":9000", "Address to export prometheus metrics")
	logLevel      = flag.String("server.log-level", "info", "Log level")
	kubeConfig    = flag.String("kube.config", "", "Path to kubeconfig (optional)")
)

func main() {
	flag.Parse()

	// Initialize logger
	if err := logger.Init(*logLevel); err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Create Kubernetes client
	var config *rest.Config
	var err error

	if *kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		logger.Fatalf("Failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Failed to create kubernetes client: %v", err)
	}

	// Create nodegroup collector
	nodegroupCollector, err := collector.NewNodeGroupCollector(clientset, config)
	if err != nil {
		logger.Fatalf("Failed to create nodegroup collector: %v", err)
	}

	// Start collector
	ctx := context.Background()
	if err := nodegroupCollector.Start(ctx); err != nil {
		logger.Fatalf("Failed to start collector: %v", err)
	}

	// Register collector with Prometheus
	prometheus.MustRegister(nodegroupCollector)

	// Create HTTP server
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

	// Start server in goroutine
	go func() {
		logger.Infof("Starting HTTP server on %s", *serverAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("HTTP server failed: %v", err)
		}
		logger.Info("HTTP server stopped")
	}()

	logger.Info("Node group exporter is ready")
	logger.Infof("Metrics available at %s", *serverAddress)

	// Wait for interrupt signal
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
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}
