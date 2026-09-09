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

// Package metrics exports what a consumer of the rules knows about its own freshness, so that the
// lag between user-authz-controller and the consumers, a stuck informer on one master, or a rule
// nobody can compile become alerts instead of surprises.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

// Metrics implements source.Observer with Prometheus collectors. The component label tells the
// webhook of one master from permission-browser in the same series.
type Metrics struct {
	rulesObserved      prometheus.Gauge
	maxResourceVersion prometheus.Gauge
	subjects           prometheus.Gauge
	quarantined        prometheus.Gauge
	updatedTimestamp   prometheus.Gauge
	synced             prometheus.Gauge
	rebuildDuration    prometheus.Histogram
	rebuilds           prometheus.Counter
	watchErrors        prometheus.Counter
}

// New builds the collectors. namespace is the metric prefix (user_authz_webhook,
// user_authz_permission_browser).
func New(namespace string) *Metrics {
	return &Metrics{
		rulesObserved: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_observed",
			Help: "Number of ClusterAuthorizationRules the current directory was built from.",
		}),
		maxResourceVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_max_resource_version",
			Help: "Highest resourceVersion among the rules of the current directory.",
		}),
		subjects: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_subjects",
			Help: "Number of distinct subjects in the current directory.",
		}),
		quarantined: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_quarantined",
			Help: "Number of rules whose limitNamespaces pattern or namespaceSelector does not compile; their subjects are limited to what the rest of the rule allows.",
		}),
		updatedTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_directory_updated_timestamp_seconds",
			Help: "Unix time of the last directory rebuild.",
		}),
		synced: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "rules_informer_synced",
			Help: "1 once the rules informer has listed the cluster at least once.",
		}),
		rebuildDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "rules_directory_rebuild_duration_seconds",
			Help:    "Time spent building the directory from the informer store.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
		}),
		rebuilds: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Name: "rules_directory_rebuilds_total",
			Help: "Number of directory rebuilds.",
		}),
		watchErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Name: "rules_watch_errors_total",
			Help: "Number of list/watch errors of the rules informer.",
		}),
	}
}

var _ prometheus.Collector = (*Metrics)(nil)

func (m *Metrics) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.rulesObserved, m.maxResourceVersion, m.subjects, m.quarantined, m.updatedTimestamp,
		m.synced, m.rebuildDuration, m.rebuilds, m.watchErrors,
	}
}

// Register registers the collectors with a Prometheus registry.
func (m *Metrics) Register(reg prometheus.Registerer) error {
	return reg.Register(m)
}

// Describe implements prometheus.Collector, so the metrics can also be handed to a registry that
// takes raw collectors, such as the one of a Kubernetes-style apiserver.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	for _, c := range m.collectors() {
		c.Describe(ch)
	}
}

// Collect implements prometheus.Collector.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	for _, c := range m.collectors() {
		c.Collect(ch)
	}
}

// DirectoryRebuilt implements source.Observer.
func (m *Metrics) DirectoryRebuilt(stats rules.Stats, took time.Duration) {
	m.rulesObserved.Set(float64(stats.Rules))
	m.maxResourceVersion.Set(float64(stats.MaxResourceVersion))
	m.subjects.Set(float64(stats.Subjects))
	m.quarantined.Set(float64(len(stats.Quarantined)))
	m.updatedTimestamp.SetToCurrentTime()
	m.rebuildDuration.Observe(took.Seconds())
	m.rebuilds.Inc()
}

// SyncedChanged implements source.Observer.
func (m *Metrics) SyncedChanged(synced bool) {
	if synced {
		m.synced.Set(1)
	} else {
		m.synced.Set(0)
	}
}

// WatchError implements source.Observer.
func (m *Metrics) WatchError(error) {
	m.watchErrors.Inc()
}
