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

// Package metrics is what the agent knows about the pulls passing through it.
//
// Never scraped directly. That is not a shortcut: the agent is a static pod because it has to
// keep working when the API server does not, and a kube-rbac-proxy beside it would
// authenticate against that same API server — turning the one component built to survive an
// outage into one that reports nothing during it. So the collectors here reach Prometheus as a
// file in the node exporter's textfile directory (Textfile), under the node exporter's job and
// with the node's name as a label, and someone on the node with a shell reads them from the
// loopback listener. The agent's state as the cluster sees it still travels through the
// RegistryNode status.
//
// What is here is what nothing else can see. Once the container runtime is pointed at the
// agent, every pull on the node passes through it — including pulls of images this module
// knows nothing about — and how those were routed exists in no resource and in no other
// process.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics are the collectors the agent updates.
type Metrics struct {
	// Requests counts pulls by how they were routed and how they ended.
	//
	// The kind label is the interesting one: it separates the images this module manages
	// from everything else the node happens to pull, and the second group is invisible
	// from anywhere but here.
	Requests *prometheus.CounterVec

	// Failovers counts a target being passed over for the next one.
	Failovers *prometheus.CounterVec

	// Tokens counts credential exchanges, cached and not.
	Tokens *prometheus.CounterVec

	// LayoutOrigin reports where the layout in use came from.
	LayoutOrigin *prometheus.GaugeVec

	// MirrorRequests counts what the node's RegistryMirror answered, by mirror and result.
	//
	// Its own counter because a mirror's answer never ends a request on its own — a miss goes on
	// to the targets behind it — so Requests cannot tell how much the mirror actually serves. The
	// share of hits is what says whether the mirror is filled with what the node pulls.
	MirrorRequests *prometheus.CounterVec
}

// New registers the collectors and returns them.
func New(registry prometheus.Registerer) *Metrics {
	m := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_agent_requests_total",
			Help: "Registry requests the agent routed, by routing kind and result.",
		}, []string{"kind", "result"}),
		Failovers: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_agent_failovers_total",
			Help: "Times a target was passed over for the next one, by target and reason.",
		}, []string{"target", "reason"}),
		Tokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_agent_token_exchanges_total",
			Help: "Credential exchanges with a registry's token service, by result.",
		}, []string{"result"}),
		LayoutOrigin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "d8_registry_agent_layout_origin",
			Help: "1 for the origin of the layout currently in use.",
		}, []string{"origin"}),
		MirrorRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "d8_registry_agent_mirror_requests_total",
			Help: "Requests the agent sent to the node's RegistryMirror, by mirror and result.",
		}, []string{"mirror", "result"}),
	}

	registry.MustRegister(m.Requests, m.Failovers, m.Tokens, m.LayoutOrigin, m.MirrorRequests)
	return m
}

// Results of a request to a RegistryMirror.
const (
	// MirrorHit is the mirror serving the content.
	MirrorHit = "hit"
	// MirrorMiss is the mirror not holding it, which sends the request on.
	MirrorMiss = "miss"
	// MirrorRefused is the mirror refusing the request: its credentials, its permissions.
	MirrorRefused = "refused"
	// MirrorUnavailable is the mirror not answering, or answering with a server error.
	MirrorUnavailable = "unavailable"
	// MirrorDigestMismatch is the mirror serving content that is not what the digest names.
	MirrorDigestMismatch = "digest_mismatch"
)

// Results a routed request can have.
const (
	ResultServed      = "served"
	ResultUnroutable  = "unroutable"
	ResultAllFailed   = "all_failed"
	ResultNotFound    = "not_found"
	ResultUnavailable = "unavailable"
)

// Reasons a target was passed over.
const (
	ReasonUnreachable = "unreachable"
	ReasonServerError = "server_error"

	// ReasonUnauthorized is a target that refused the agent's own credentials. Its own
	// reason because it is the one that points at this module rather than at the network:
	// nothing the client did can cause it, and nothing the client does can fix it.
	ReasonUnauthorized = "unauthorized"

	// ReasonRefused is a mirror answering with a client error other than a miss. A mirror's
	// refusal is never relayed, whoever's credentials it turned down.
	ReasonRefused = "refused"
)

// Results of a token exchange.
const (
	TokenCached   = "cached"
	TokenObtained = "obtained"
	TokenFailed   = "failed"
)
