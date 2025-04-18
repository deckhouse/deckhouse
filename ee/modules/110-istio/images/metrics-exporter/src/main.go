/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {

	reg := prometheus.NewRegistry()
	cfg := LoadConfig()

	// Kube client
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to get cluster config: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		log.Fatal("Failed to create clientset: %v", err)
	}

	metrics := RegisterMetrics(clientSet, reg)
	watcher := NewWatcher(clientSet)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		watcher.StartPodWatcher(ctx, cfg.Namespace)
	}()

	// Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		StartPrometheusServer(ctx, reg, "127.0.0.1:8080")
	}()

	// Get status every 30 sec
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				podsInfo := watcher.GetRunningIstiodPods()
				metrics.GetIstiodRemoteClustersStatus(ctx, cfg.Namespace, cfg.SA, podsInfo)
			case <-ctx.Done():
				log.Info("Shutting down Istio monitor")
				return
			}

		}
	}()

	<-ctx.Done()
	log.Info("Shutdown signal received")
	wg.Wait()
	log.Info("Shutdown complete")
}
