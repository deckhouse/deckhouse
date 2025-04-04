/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {

	reg := prometheus.NewRegistry()
	metrics := RegisterMetrics(reg)
	cfg := LoadConfig()
	watcher := NewWatcher()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()


	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		watcher.StartEndpointSliceWatcher(ctx, cfg.ServiceName, cfg.Namespace)
	}()


	// Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		StartPrometheusServer(":8080", reg, ctx)
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
				ips := watcher.GetIPsIstiodPods()
				metrics.GetIstiodRemoteClustersStatus(ctx, cfg.Namespace, cfg.SA, ips)
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
