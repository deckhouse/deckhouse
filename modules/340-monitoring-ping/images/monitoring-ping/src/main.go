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
	"os/signal"
	"sync"
	"syscall"
	"time"
	"flag"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	countPings int = 30 // Count pings on every cycle
	cleanupNodeExporterMetrics bool
)

func main() {

	flag.BoolVar(&cleanupNodeExporterMetrics, "cleanup-node-exporter-metrics", false, "Clean up node exporter metrics")
	flag.Parse()
	if cleanupNodeExporterMetrics {
		// TODO remove volumes in future, need for clean staled metrics
		log.Info("cleanup mode enabled, running CleanUpDeprecatedExporterFile")
		CleanUpDeprecatedExporterFile()
		return
	}

	reg := prometheus.NewRegistry()
	metrics := RegisterMetrics(reg)
	cfg := LoadConfig()

	//
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nodeTracker := NewNodeTracker()
	if err := nodeTracker.Start(ctx, cfg.targetsCM, cfg.Namespace); err != nil {
		log.Fatal("can't start node tracker: %v", err)
	}

	var wg sync.WaitGroup

	// metrics HTTP
	wg.Add(1)
	go func() {
		defer wg.Done()
		StartPrometheusServer(ctx, "127.0.0.1:4288", reg)
	}()

	// Ping
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Duration(countPings) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("ping loop stopped")
				return
			default:
				clusterTargets := nodeTracker.ListClusterTargets()
				externalTargets := nodeTracker.ListExternalTargets()
				metrics.CleanupMetrics(clusterTargets, externalTargets)
				PingAll(ctx, clusterTargets, externalTargets, countPings, metrics)
			}
		}
	}()

	//
	<-ctx.Done()
	log.Info("main: context canceled (SIGINT/SIGTERM), waiting for goroutines...")
	wg.Wait()
	log.Info("main: shutdown complete")
}
