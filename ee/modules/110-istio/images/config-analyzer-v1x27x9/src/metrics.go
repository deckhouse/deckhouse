/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

//go:build deckhouse_external

package main

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"istio.io/istio/pkg/log"
)

type exporter struct {
	istioNamespace string
	revision       string
	allNamespaces  bool
	timeout        time.Duration

	issuesGauge *prometheus.GaugeVec
	lastRunGauge prometheus.Gauge
	runErrorsCounter prometheus.Counter

	mu sync.Mutex
}

func newExporter(istioNamespace, revision string, allNamespaces bool, timeout time.Duration) *exporter {
	return &exporter{
		istioNamespace: istioNamespace,
		revision:       revision,
		allNamespaces:  allNamespaces,
		timeout:        timeout,
		issuesGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "d8_istio_config_analysis_issue",
				Help: "Istio configuration analysis issue detected in the cluster (1 means present).",
			},
			[]string{"type", "namespace", "resource", "severity", "revision", "code", "message"},
		),
		lastRunGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "d8_istio_config_analysis_last_run_timestamp_seconds",
				Help: "Unix timestamp of the last successful Istio configuration analysis run.",
			},
		),
		runErrorsCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "d8_istio_config_analysis_run_errors_total",
				Help: "Total number of failed Istio configuration analysis runs.",
			},
		),
	}
}

func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	e.issuesGauge.Describe(ch)
	e.lastRunGauge.Describe(ch)
	e.runErrorsCounter.Describe(ch)
}

func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.issuesGauge.Collect(ch)
	e.lastRunGauge.Collect(ch)
	e.runErrorsCounter.Collect(ch)
}

func (e *exporter) run(ctx context.Context, interval time.Duration) {
	e.analyzeOnce(ctx)

	for {
		if err := waitForNextRun(ctx, interval); err != nil {
			return
		}
		e.analyzeOnce(ctx)
	}
}

func (e *exporter) analyzeOnce(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	messages, err := runAnalysis(runCtx, e.istioNamespace, e.revision, e.allNamespaces)

	e.mu.Lock()
	defer e.mu.Unlock()

	if err != nil {
		e.runErrorsCounter.Inc()
		log.Errorf("analysis run failed: %v", err)
		return
	}

	e.issuesGauge.Reset()
	for _, message := range messages {
		messageType, namespace, resourceName, severity, code, messageText := messageLabels(message, e.revision)
		e.issuesGauge.WithLabelValues(messageType, namespace, resourceName, severity, e.revision, code, messageText).Set(1)
	}
	e.lastRunGauge.Set(float64(time.Now().Unix()))
}
