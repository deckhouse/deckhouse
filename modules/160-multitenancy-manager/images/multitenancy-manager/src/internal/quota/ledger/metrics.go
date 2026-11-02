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

package ledger

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics are the series of the ledger. Their labels are bounded: results, sources (the API servers),
// reasons and outcomes; no series names a project.
type Metrics struct {
	Requests         *prometheus.CounterVec
	HandlerDuration  prometheus.Histogram
	Inflight         *prometheus.GaugeVec
	InflightRatioMax prometheus.Gauge
	Leader           *prometheus.GaugeVec
	LeaseRenewAge    prometheus.Gauge
	Handovers        *prometheus.CounterVec
	HandoverDrain    prometheus.Histogram
	Synced           prometheus.Gauge
	Pending          prometheus.Gauge
	PendingResolved  *prometheus.CounterVec
	EndpointWrites   *prometheus.CounterVec
	// InflightWaits are the writes that waited for the approvals of their project to be confirmed.
	InflightWaits *prometheus.CounterVec
	CanaryResults *prometheus.CounterVec
	Drift         prometheus.Gauge
	WakeWrites    prometheus.Counter
}

// NewMetrics registers the series in the registry.
func NewMetrics(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_requests_total",
			Help: "Calls of the project quota webhook by their outcome.",
		}, []string{"result"}),
		HandlerDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "d8_multitenancy_quota_ledger_handler_duration_seconds",
			Help:    "Time a call of the project quota webhook spends in the handler.",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05},
		}),
		Inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_inflight",
			Help: "Calls of the project quota webhook in progress, by the address they come from.",
		}, []string{"source"}),
		InflightRatioMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_inflight_ratio_max",
			Help: "The largest share of a limit of a project that approvals not confirmed by the informer take.",
		}),
		Leader: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_leader",
			Help: "1 while this replica holds the ledger lease and answers.",
		}, []string{"pod"}),
		LeaseRenewAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_lease_renew_age_seconds",
			Help: "Seconds since the holder sent its last successful renewal of the ledger lease.",
		}),
		Handovers: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_handover_total",
			Help: "Changes of the ledger leader by their reason.",
		}, []string{"reason"}),
		HandoverDrain: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "d8_multitenancy_quota_ledger_handover_drain_seconds",
			Help:    "Time a leader that steps down waits for its pending approvals to be confirmed.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 9},
		}),
		Synced: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_synced",
			Help: "1 when the informers of the leader caught up with its quorum list.",
		}),
		Pending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_pending",
			Help: "Approvals the informer has not confirmed yet.",
		}),
		PendingResolved: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_pending_resolved_total",
			Help: "Approvals settled: confirmed by the informer, or by a quorum read that found a newer or the same version.",
		}, []string{"outcome"}),
		EndpointWrites: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_endpoint_writes_total",
			Help: "Writes of the EndpointSlice of the ledger Service by who made them.",
		}, []string{"actor"}),
		CanaryResults: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_canary_result_total",
			Help: "Canary writes through each API server: ok, ours (failed on the ledger), unknown (failed elsewhere), no_nonce (the webhook was not called).",
		}, []string{"apiserver", "result"}),
		Drift: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "d8_multitenancy_quota_ledger_drift",
			Help: "Namespaces whose quota the ledger holds with a usage different from a quorum read of the same version; anything but 0 is a bug.",
		}),
		WakeWrites: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "d8_multitenancy_quota_ledger_wake_writes_total",
			Help: "Writes that wake the quota of a namespace that was refused, after the project freed some of its quota.",
		}),
	}
	m.InflightWaits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_multitenancy_quota_ledger_inflight_waits_total",
		Help: "Writes that found no room for the unconfirmed approvals of their project and waited for them to be confirmed, by how the wait ended.",
	}, []string{"outcome"})
	registry.MustRegister(m.InflightWaits)
	registry.MustRegister(m.Requests, m.HandlerDuration, m.Inflight, m.InflightRatioMax, m.Leader, m.LeaseRenewAge, m.Handovers,
		m.HandoverDrain, m.Synced, m.Pending, m.PendingResolved, m.EndpointWrites, m.CanaryResults, m.Drift, m.WakeWrites)
	return m
}
