/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var logger = log.New(os.Stdout, "healthcheck: ", log.LstdFlags)

func main() {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	federationEnabled := os.Getenv("FEDERATION_ENABLED") == "true"
	multiclusterEnabled := os.Getenv("MULTICLUSTER_ENABLED") == "true"

	if clusterDomain == "" {
		clusterDomain = "cluster.local"
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalf("Failed to get in-cluster config: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Failed to create dynamic client: %v", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	checker := NewChecker(dynClient, reg, CheckerConfig{
		ClusterUUID:         clusterUUID,
		ClusterDomain:       clusterDomain,
		FederationEnabled:   federationEnabled,
		MulticlusterEnabled: multiclusterEnabled,
		CheckInterval:       60 * time.Second,
		RequestTimeout:      5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Ok.")
		reqPath := ""
		if r.URL != nil {
			reqPath = r.URL.Path
		}
		if r.UserAgent() == allianceHealthcheckUserAgent {
			logger.Printf("Remote cluster '%s' healthchecking us: %s %s %s", r.Header.Get("X-alliance-healthcheck-from"), r.Method, r.UserAgent(), reqPath)
		} else {
			logger.Printf("%s %s %s %s", r.RemoteAddr, r.Method, r.UserAgent(), reqPath)
		}
	})
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	server := &http.Server{
		Addr:         "0.0.0.0:8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	wg.Add(1)
	go func() {
		defer wg.Done()
		checker.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Println("Server starting on 0.0.0.0:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("Server error: %v", err)
			select {
			case stop <- syscall.SIGTERM:
			default:
			}
		}
	}()

	<-stop
	logger.Println("Shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("Server shutdown failed: %v", err)
	}

	wg.Wait()
	logger.Println("Stopped.")
}
