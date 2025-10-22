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
	"fmt"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusExporterMetrics struct {
	extSent     *prometheus.CounterVec
	extReceived *prometheus.CounterVec
	extRtt      *prometheus.CounterVec
	extMin      *prometheus.GaugeVec
	extMax      *prometheus.GaugeVec
	extMdev     *prometheus.GaugeVec

	nodeSent     *prometheus.CounterVec
	nodeReceived *prometheus.CounterVec
	nodeRtt      *prometheus.CounterVec
	nodeMin      *prometheus.GaugeVec
	nodeMax      *prometheus.GaugeVec
	nodeMdev     *prometheus.GaugeVec

	previousClusterMap  map[string]string
	previousExternalMap map[string]string
}

func RegisterMetrics(reg prometheus.Registerer) *PrometheusExporterMetrics {

	p := &PrometheusExporterMetrics{
		nodeSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kube_node_ping_packets_sent_total",
			Help: "ICMP packets sent",
		}, []string{"destination_node", "destination_node_ip_address"}),
		nodeReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kube_node_ping_packets_received_total",
			Help: "ICMP packets received",
		}, []string{"destination_node", "destination_node_ip_address"}),
		nodeRtt: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kube_node_ping_rtt_milliseconds_total",
			Help: "Total round-trip time",
		}, []string{"destination_node", "destination_node_ip_address"}),
		nodeMin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_node_ping_rtt_min",
			Help: "Minimum round-trip time",
		}, []string{"destination_node", "destination_node_ip_address"}),
		nodeMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_node_ping_rtt_max",
			Help: "Maximum round-trip time",
		}, []string{"destination_node", "destination_node_ip_address"}),
		nodeMdev: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_node_ping_rtt_mdev",
			Help: "Standard deviation of RTT",
		}, []string{"destination_node", "destination_node_ip_address"}),
		extSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "external_ping_packets_sent_total",
			Help: "ICMP packets sent",
		}, []string{"destination_name", "destination_host"}),
		extReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "external_ping_packets_received_total",
			Help: "ICMP packets received",
		}, []string{"destination_name", "destination_host"}),
		extRtt: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "external_ping_rtt_milliseconds_total",
			Help: "Total round-trip time",
		}, []string{"destination_name", "destination_host"}),
		extMin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "external_ping_rtt_min",
			Help: "Minimum round-trip time",
		}, []string{"destination_name", "destination_host"}),
		extMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "external_ping_rtt_max",
			Help: "Maximum round-trip time",
		}, []string{"destination_name", "destination_host"}),
		extMdev: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "external_ping_rtt_mdev",
			Help: "Standard deviation of RTT",
		}, []string{"destination_name", "destination_host"}),

		previousClusterMap:  make(map[string]string),
		previousExternalMap: make(map[string]string),
	}

	reg.MustRegister(
		p.nodeSent, p.nodeReceived, p.nodeRtt, p.nodeMin, p.nodeMax, p.nodeMdev,
		p.extSent, p.extReceived, p.extRtt, p.extMin, p.extMax, p.extMdev,
	)

	return p
}

func (p *PrometheusExporterMetrics) UpdateExternal(name, host string, rtts []float64, sent, received int) {
	labels := prometheus.Labels{"destination_name": name, "destination_host": host}
	updateSet(p.extSent, p.extReceived, p.extRtt, p.extMin, p.extMax, p.extMdev, labels, rtts, sent, received)
}

func (p *PrometheusExporterMetrics) UpdateNode(name, ip string, rtts []float64, sent, received int) {
	labels := prometheus.Labels{"destination_node": name, "destination_node_ip_address": ip}
	updateSet(p.nodeSent, p.nodeReceived, p.nodeRtt, p.nodeMin, p.nodeMax, p.nodeMdev, labels, rtts, sent, received)
}

func updateSet(sentVec, recvVec *prometheus.CounterVec, rttVec *prometheus.CounterVec,
	minVec, maxVec, stdVec *prometheus.GaugeVec,
	labels prometheus.Labels, rtts []float64, sent, received int) {

	// log.Info(fmt.Sprintf("ping sent=%d, received=%d, rtts=%v", sent, received, rtts))

	sentVec.With(labels).Add(float64(sent))
	recvVec.With(labels).Add(float64(received))

	if len(rtts) == 0 {
		rttVec.With(labels).Add(0)
		minVec.With(labels).Set(0)
		maxVec.With(labels).Set(0)
		stdVec.With(labels).Set(0)
		return
	}

	min, max, _, std, sum := Summarize(rtts)

	rttVec.With(labels).Add(sum)
	minVec.With(labels).Set(min)
	maxVec.With(labels).Set(max)
	stdVec.With(labels).Set(std)
}

func StartPrometheusServer(ctx context.Context, addr string, reg *prometheus.Registry) {

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Metrics
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// pprof endpoints
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Info(fmt.Sprintf("Starting Prometheus metrics server at %s", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Prometheus server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Prometheus server shutdown initiated...")

	// Timeout shutdown 5 sec
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Prometheus server graceful shutdown failed: %v", err)
	} else {
		log.Info("Prometheus server shut down cleanly")
	}
}

// CleanupMetrics removes stale metrics for nodes and external hosts
// that are no longer in the current active target lists.
func (p *PrometheusExporterMetrics) CleanupMetrics(currentCluster []NodeTarget, currentExternal []ExternalTarget) {
	// Current hosts map
	currentClusterMap := BuildClusterMap(currentCluster)
	currentExternalMap := BuildExternalMap(currentExternal)

	diffCluster := DiffMaps(p.previousClusterMap, currentClusterMap)
	diffExternal := DiffMaps(p.previousExternalMap, currentExternalMap)

	// Remove metrics for orphans hosts
	if len(diffCluster) > 0 {
		for ip, name := range diffCluster {
			p.DeleteNodeMetrics(name, ip)
		}
	}

	if len(diffExternal) > 0 {
		for host, name := range diffExternal {
			p.DeleteExternalMetrics(name, host)
		}
	}

	if len(diffCluster) > 0 || len(diffExternal) > 0 {
		log.Info(fmt.Sprintf("Cleanup orphan metrics: %d cluster targets, %d external targets", len(diffCluster), len(diffExternal)))
	}

	// Update map
	p.previousClusterMap = currentClusterMap
	p.previousExternalMap = currentExternalMap
}

func (p *PrometheusExporterMetrics) DeleteNodeMetrics(name, ip string) {
	labels := prometheus.Labels{"destination_node": name, "destination_node_ip_address": ip}
	p.nodeSent.Delete(labels)
	p.nodeReceived.Delete(labels)
	p.nodeRtt.Delete(labels)
	p.nodeMin.Delete(labels)
	p.nodeMax.Delete(labels)
	p.nodeMdev.Delete(labels)
}

func (p *PrometheusExporterMetrics) DeleteExternalMetrics(name, host string) {
	labels := prometheus.Labels{"destination_name": name, "destination_host": host}
	p.extSent.Delete(labels)
	p.extReceived.Delete(labels)
	p.extRtt.Delete(labels)
	p.extMin.Delete(labels)
	p.extMax.Delete(labels)
	p.extMdev.Delete(labels)
}
