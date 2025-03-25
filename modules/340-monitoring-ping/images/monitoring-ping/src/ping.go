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
	"time"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-ping/ping"
)

func pingAndCollectMetrics(name, host string, isNode bool, p *PrometheusExporterMetrics) {
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

	if err := pinger.Run(); err != nil {
		log.Error("failed to ping %s: %v", host, err)
		return
	}

	if isNode {
		p.UpdateNode(name, host, rtts, pinger.PacketsSent, pinger.PacketsRecv)
	} else {
		p.UpdateExternal(name, host, rtts, pinger.PacketsSent, pinger.PacketsRecv)
	}
}

func PingAll(cluster []NodeTarget, external []ExternalTarget, p *PrometheusExporterMetrics) {
	for _, node := range cluster {
		name := GetTargetName(node.Name, node.IP)
		pingAndCollectMetrics(name, node.IP, true, p)
	}
	for _, ext := range external {
		name := GetTargetName(ext.Name, ext.Host)
		pingAndCollectMetrics(name, ext.Host, false, p)
	}
}
