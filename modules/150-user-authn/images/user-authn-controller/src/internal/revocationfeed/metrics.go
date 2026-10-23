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

package revocationfeed

import (
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	subscribersMetricName = "d8_user_authn_revocation_feed_subscribers"
	lagMetricName         = "d8_user_authn_revocation_feed_lag_seconds"
	deliveryMetricName    = "d8_user_authn_revocation_feed_delivery_seconds"
	connectionsMetricName = "d8_user_authn_revocation_feed_connections_total"

	// resultOK counts every stream opened; resultResumed and resultSnapshot split them by how they
	// started. resultUnauthorized and resultLimited count the refusals.
	resultOK           = "ok"
	resultUnauthorized = "unauthorized"
	resultLimited      = "limited"
	resultResumed      = "resumed"
	resultSnapshot     = "snapshot"
)

var (
	subscribersDesc = prometheus.NewDesc(subscribersMetricName,
		"Streams of the revocation feed a DexAuthenticator client has open on this replica; 0 for a known client without streams.",
		[]string{"client_id"}, nil)
	lagDesc = prometheus.NewDesc(lagMetricName,
		"The longest time a stream of this replica has not written to its client, in seconds; a heartbeat is written every 10 seconds.",
		nil, nil)
)

// metrics are the metrics of one feed. The counters are in the package, the gauges are read from
// the server when scraped.
type metrics struct {
	delivery    prometheus.Histogram
	connections *prometheus.CounterVec
}

func newMetrics() *metrics {
	connections := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: connectionsMetricName,
		Help: "Connections to the revocation feed, by result: ok for every stream opened, resumed and snapshot for how " +
			"it started, unauthorized and limited for the refusals.",
	}, []string{"result"})
	for _, result := range []string{resultOK, resultUnauthorized, resultLimited, resultResumed, resultSnapshot} {
		connections.WithLabelValues(result)
	}
	return &metrics{
		delivery: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    deliveryMetricName,
			Help:    "Time from a revocation to the write of its event to a stream, in seconds.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		}),
		connections: connections,
	}
}

// collector publishes the gauges of a server and its counters.
type collector struct {
	server *Server
}

// Describe implements prometheus.Collector.
func (c collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- subscribersDesc
	ch <- lagDesc
	c.server.metrics.delivery.Describe(ch)
	c.server.metrics.connections.Describe(ch)
}

// Collect implements prometheus.Collector.
func (c collector) Collect(ch chan<- prometheus.Metric) {
	perClient, lag := c.server.hub.stats(c.server.now())
	for _, id := range c.server.clients.list() {
		if _, ok := perClient[id]; !ok {
			perClient[id] = 0
		}
	}
	for id, n := range perClient {
		ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(n), id)
	}
	ch <- prometheus.MustNewConstMetric(lagDesc, prometheus.GaugeValue, lag.Seconds())
	c.server.metrics.delivery.Collect(ch)
	c.server.metrics.connections.Collect(ch)
}

// register registers the metrics of s in registry.
func register(registry prometheus.Registerer, s *Server) error {
	if err := registry.Register(collector{server: s}); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			return nil
		}
		return fmt.Errorf("register revocation feed metrics: %w", err)
	}
	return nil
}

func (m *metrics) observeDelivery(created, now time.Time) {
	if created.IsZero() {
		return
	}
	m.delivery.Observe(max(now.Sub(created), 0).Seconds())
}
