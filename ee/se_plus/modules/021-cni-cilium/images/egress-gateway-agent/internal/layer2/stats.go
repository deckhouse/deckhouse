/*
Copyright 2018 MetalLB

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

package layer2

import "github.com/prometheus/client_golang/prometheus"

var stats = metrics{
	in: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "layer2",
		Name:      "requests_received",
		Help:      "Number of layer2 requests received for owned IPs",
	}, []string{
		"ip",
	}),

	out: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "layer2",
		Name:      "responses_sent",
		Help:      "Number of layer2 responses sent for owned IPs in response to requests",
	}, []string{
		"ip",
	}),

	gratuitous: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "layer2",
		Name:      "gratuitous_sent",
		Help:      "Number of gratuitous layer2 packets sent for owned IPs as a result of failovers",
	}, []string{
		"ip",
	}),
}

type metrics struct {
	in         *prometheus.CounterVec
	out        *prometheus.CounterVec
	gratuitous *prometheus.CounterVec
}

func init() {
	prometheus.MustRegister(stats.in)
	prometheus.MustRegister(stats.out)
	prometheus.MustRegister(stats.gratuitous)
}

func (m *metrics) GotRequest(addr string) {
	m.in.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentResponse(addr string) {
	m.out.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentGratuitous(addr string) {
	m.gratuitous.WithLabelValues(addr).Add(1)
}
