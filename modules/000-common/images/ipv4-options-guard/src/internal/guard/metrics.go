/*
Copyright 2026 Flant JSC

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

package guard

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const metricsNamespace = "d8_ipv4_options_guard"

type metrics struct {
	info     *prometheus.GaugeVec
	netns    *prometheus.GaugeVec
	matched  *prometheus.CounterVec
	repaired *prometheus.CounterVec
	bypassed prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace, Name: "node_info",
			Help: "Guard state on the node; protected=false means the filter could not be installed, " +
				"mismatch=true means it uses a different mode than iptables-wrapper selected; reason is " +
				ReasonWrapperFailed + " or " + ReasonModulesNotLoaded + ", details are in the log.",
		}, []string{"mode", "backend", "protected", "mismatch", "reason"}),
		netns: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace, Name: "netns",
			Help: "Network namespaces by reconcile result at the last resync.",
		}, []string{"state"}),
		matched: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "matched_packets_total",
			Help: "IPv4 packets with IP options matched by the filter (dropped in Enforce, counted only in Audit).",
		}, []string{"rule"}),
		repaired: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "filter_repairs_total",
			Help: "Filters found changed outside the guard (rules, verdicts, chain hook or table flags) and rewritten.",
		}, []string{"backend"}),
		bypassed: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace, Name: "filter_bypassed_netns",
			Help: "Network namespaces where a rule in raw PREROUTING ahead of the filter can accept packets before " +
				"they reach it (iptables-legacy only). The guard leaves such rules in place.",
		}),
	}
	reg.MustRegister(m.info, m.netns, m.matched, m.repaired, m.bypassed)
	return m
}

func (m *metrics) SetInfo(mode Mode, backend, reason string, protected, mismatch bool) {
	m.info.Reset()
	m.info.WithLabelValues(string(mode), backend, strconv.FormatBool(protected), strconv.FormatBool(mismatch), reason).Set(1)
}

func (m *metrics) setNetNS(ok, failed int) {
	m.netns.WithLabelValues("ok").Set(float64(ok))
	m.netns.WithLabelValues("failed").Set(float64(failed))
}
