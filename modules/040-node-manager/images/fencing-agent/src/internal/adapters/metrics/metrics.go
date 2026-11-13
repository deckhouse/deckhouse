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

// Package metrics keeps the Prometheus metrics of the agent. The health server
// serves them on its local address, and kube-rbac-proxy exposes them to Prometheus.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"fencing-agent/internal/domain"
)

// Metrics implements domain.Metrics. Every metric has the label node.
type Metrics struct {
	registry      *prometheus.Registry
	condition     *prometheus.GaugeVec
	cpSource      *prometheus.GaugeVec
	decisions     *prometheus.CounterVec
	polls         prometheus.Counter
	pollAnswers   *prometheus.CounterVec
	configInvalid *prometheus.GaugeVec
}

var _ domain.Metrics = (*Metrics)(nil)

func New(node string) *Metrics {
	labels := prometheus.Labels{"node": node}
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		condition: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "d8_fencing_agent_condition",
			Help:        "Reason of the Fencing condition of the node. One series.",
			ConstLabels: labels,
		}, []string{"reason"}),
		cpSource: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "d8_fencing_agent_cp_source",
			Help:        "Source of the control plane state: proxy or self. One series.",
			ConstLabels: labels,
		}, []string{"source"}),
		decisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "d8_fencing_agent_decisions_total",
			Help:        "Decisions about the watchdog feed: feed or stop.",
			ConstLabels: labels,
		}, []string{"decision"}),
		polls: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "d8_fencing_agent_polls_total",
			Help:        "Polls of the peers.",
			ConstLabels: labels,
		}),
		pollAnswers: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "d8_fencing_agent_poll_answers_total",
			Help:        "Poll answers by network and class: witness, support or silent.",
			ConstLabels: labels,
		}, []string{"network", "class"}),
		configInvalid: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "d8_fencing_agent_config_invalid",
			Help:        "1 when the value of the parameter is invalid and not applied.",
			ConstLabels: labels,
		}, []string{"parameter"}),
	}

	m.registry.MustRegister(m.condition, m.cpSource, m.decisions, m.polls, m.pollAnswers, m.configInvalid)

	return m
}

// Condition replaces the series of the condition reason.
func (m *Metrics) Condition(reason domain.Reason) {
	m.condition.Reset()
	m.condition.WithLabelValues(string(reason)).Set(1)
}

// CPSource replaces the series of the CP source.
func (m *Metrics) CPSource(source domain.CPSource) {
	m.cpSource.Reset()
	m.cpSource.WithLabelValues(string(source)).Set(1)
}

func (m *Metrics) Decision(decision domain.FeedDecision) {
	m.decisions.WithLabelValues(string(decision)).Inc()
}

func (m *Metrics) Poll() { m.polls.Inc() }

func (m *Metrics) PollAnswer(network domain.Network, class domain.AnswerClass) {
	m.pollAnswers.WithLabelValues(string(network), string(class)).Inc()
}

func (m *Metrics) ConfigInvalid(parameter string, invalid bool) {
	if invalid {
		m.configInvalid.WithLabelValues(parameter).Set(1)

		return
	}

	m.configInvalid.DeleteLabelValues(parameter)
}

// Handler serves the metrics on /metrics.
func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	return mux
}
