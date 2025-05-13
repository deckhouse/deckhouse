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
	"errors"
	"fmt"
	fastping "ping/pkg/fastping"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// PingAll sends ICMP pings to all cluster and external targets in batch mode.
// Uses a single Pinger instance and runs all pings concurrently inside the library.
func PingAll(ctx context.Context, cluster []NodeTarget, external []ExternalTarget, countPings int, p *PrometheusExporterMetrics) {
	// Prepare flat list of hosts and a map to distinguish internal/external
	startPingTime := time.Now()
	var allHosts []string
	summuryCountHosts := len(cluster) + len(external)
	hostTypes := make(map[string]string, summuryCountHosts) // host -> "internal" / "external"
	nameMap := make(map[string]string, summuryCountHosts)   // host -> name
	log.Info(fmt.Sprintf("count internal nodes: %d", len(cluster)))
	log.Info(fmt.Sprintf("count external hosts: %d", len(external)))

	for _, node := range cluster {
		allHosts = append(allHosts, node.IP)
		hostTypes[node.IP] = "internal"
		nameMap[node.IP] = GetTargetName(node.Name, node.IP)
	}

	for _, ext := range external {
		allHosts = append(allHosts, ext.Host)
		hostTypes[ext.Host] = "external"
		nameMap[ext.Host] = GetTargetName(ext.Name, ext.Host)
	}

	// Initialize fastping with list of hosts
	fp := fastping.NewPinger(allHosts, countPings, 1*time.Second, 5*time.Second)

	// Collect RTTs per host
	rttsMap := make(map[string][]float64, len(allHosts))
	for _, host := range allHosts {
		rttsMap[host] = make([]float64, 0, countPings)
	}

	// Callback for each received packet
	fp.OnRecv = func(pkt fastping.PacketResult) {
		host := pkt.Host
		rttsMap[host] = append(rttsMap[host], float64(pkt.RTT.Seconds()*1000))
	}

	// Run pinger
	if err := fp.RunWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error(fmt.Sprintf("Failed to run pinger: %v\n", err))
		return
	}

	// Update metrics
	for _, host := range allHosts {
		rtts := rttsMap[host]
		sent, recv := fp.StatsForHost(host)
		// log.Info(fmt.Sprintf("Metrics host %s, sent: %d, recv: %d", host, sent, recv))

		name := nameMap[host]

		switch hostTypes[host] {
		case "internal":
			p.UpdateNode(name, host, rtts, sent, recv)
		case "external":
			p.UpdateExternal(name, host, rtts, sent, recv)
		}
	}
	log.Info(fmt.Sprintf("Ping take a %v sec time", time.Since(startPingTime).Seconds()))
}
