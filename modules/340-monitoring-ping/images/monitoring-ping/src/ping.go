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
	"errors"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	ping "github.com/prometheus-community/pro-bing"
)

// pingAndCollectMetrics performs ICMP ping to a given host,
// collects RTTs and updates Prometheus metrics accordingly.
// It respects context cancellation for graceful shutdown.
func pingAndCollectMetrics(ctx context.Context, name, host string, isNode bool, p *PrometheusExporterMetrics) {

	pinger, err := ping.NewPinger(host)
	if err != nil {
		log.Error("ping error: %s -> %v", host, err)
		return
	}

	pinger.Count = 30
	pinger.Interval = time.Second
	pinger.Timeout = 35 * time.Second
	pinger.SetPrivileged(true)

	var rtts []float64
	pinger.OnRecv = func(pkt *ping.Packet) {
		rtts = append(rtts, float64(pkt.Rtt.Microseconds())/1000)
	}

	if err := pinger.RunWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("ping error: %s -> %v", host, err)
		return
	}

	if isNode {
		p.UpdateNode(name, host, rtts, pinger.PacketsSent, pinger.PacketsRecv)
	} else {
		p.UpdateExternal(name, host, rtts, pinger.PacketsSent, pinger.PacketsRecv)
	}
}

// PingAll launches a separate goroutine for each ping target (cluster and external)
// and waits for all of them to complete. Context is respected for cancellation.
func PingAll(ctx context.Context, cluster []NodeTarget, external []ExternalTarget, p *PrometheusExporterMetrics) {
	var wg sync.WaitGroup

	// Ping all cluster nodes
	for _, node := range cluster {
		name := GetTargetName(node.Name, node.IP)
		wg.Add(1)
		go func(name, ip string) {
			defer wg.Done()
			pingAndCollectMetrics(ctx, name, ip, true, p)
		}(name, node.IP)
	}

	// Ping all external targets
	for _, ext := range external {
		name := GetTargetName(ext.Name, ext.Host)
		wg.Add(1)
		go func(name, host string) {
			defer wg.Done()
			pingAndCollectMetrics(ctx, name, host, false, p)
		}(name, ext.Host)
	}

	wg.Wait()
}
