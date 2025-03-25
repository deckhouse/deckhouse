// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	externalTargets []ExternalTarget
)


func main() {

	reg := prometheus.NewRegistry()
	metrics := RegisterMetrics(reg)

	if raw := os.Getenv("EXTERNAL_TARGETS_JSON"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &externalTargets); err != nil {
			log.Warn("error parsing EXTERNAL_TARGETS_JSON: %w", err)
		}
	} else {
		log.Warn("warning: EXTERNAL_TARGETS_JSON not set")
	}

	//
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nodeTracker := NewNodeTracker()
	if err := nodeTracker.Start(ctx); err != nil {
		log.Fatal("can't start node tracker: %v", err)
	}

	var wg sync.WaitGroup

	// metrics HTTP
	wg.Add(1)
	go func() {
		defer wg.Done()
		StartPrometheusServer(":8080", reg, ctx)
	}()

	// Ping
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				clusterTargets := nodeTracker.List()
				PingAll(clusterTargets, externalTargets, metrics)
			case <-ctx.Done():
				log.Info("ping loop stopped")
				return
			}
		}
	}()

	//
	<-ctx.Done()
	log.Info("main: context canceled (SIGINT/SIGTERM), waiting for goroutines...")
	wg.Wait()
	log.Info("main: shutdown complete")
}
