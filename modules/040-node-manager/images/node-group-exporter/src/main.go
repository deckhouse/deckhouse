package main

import (
	"context"
	"flag"
	"log"
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
)

var (
	serverAddress = flag.String("server.exporter-address", ":9000", "Address to export prometheus metrics")
	logLevel      = flag.String("server.log-level", "info", "Log level")
	kubeConfig    = flag.String("kube.config", "", "Path to kubeconfig (optional)")
)

func main() {
	flag.Parse()

	// Create Kubernetes client
	var config *rest.Config
	var err error

	if *kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Fatalf("Failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	// Create nodegroup collector
	nodegroupCollector, err := collector.NewNodeGroupCollector(clientset)
	if err != nil {
		log.Fatalf("Failed to create nodegroup collector: %v", err)
	}

	// Start collector
	ctx := context.Background()
	if err := nodegroupCollector.Start(ctx); err != nil {
		log.Fatalf("Failed to start collector: %v", err)
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
		log.Printf("Starting server on %s", *serverAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop collector
	nodegroupCollector.Stop()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
