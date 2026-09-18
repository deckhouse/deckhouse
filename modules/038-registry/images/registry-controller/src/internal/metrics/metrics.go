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

// Package metrics is what the controller reports about its own decisions.
//
// Only the decisions. The state they produce is in the resources — which upstream is in
// effect, whether the cache is complete, what each node applied — and a module hook turns
// those into metrics from one place. Exporting them here as well would give one question
// two answers, and they would differ exactly when a reconciliation was in flight.
//
// What belongs here is what leaves no trace in a resource: that a preflight probe ran and
// refused an upstream, that an air-gap transition was considered and held back. Those are
// events, and by the time an operator asks, the object looks the same as if they had never
// happened.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// UpstreamProbes counts preflight probes by what they concluded.
	//
	// The outcome label is the useful part: "unreachable" and "wrong credentials" are the
	// same refusal to the reconciler and completely different problems to an operator.
	UpstreamProbes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "d8_registry_upstream_probes_total",
		Help: "Preflight probes of a configured upstream, by outcome.",
	}, []string{"outcome"})

	// AirGapHeld is 1 while the configuration asks to drop the upstream and the cache is
	// not complete enough to allow it.
	//
	// A gauge and not a counter: the question is whether the cluster is waiting now, and
	// for how long, not how many times it has been asked. It is also the one transition in
	// the design that can cut every node off from images, so the wait itself is worth
	// alerting on.
	AirGapHeld = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_air_gap_transition_held",
		Help: "1 while the upstream cannot be dropped yet because the cache leader is not complete.",
	})

	// UpstreamsRejected is how many RegistryUpstream objects arbitration refused, by
	// reason.
	UpstreamsRejected = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_upstreams_rejected",
		Help: "Additional registries refused by arbitration, by reason.",
	}, []string{"reason"})
)

// The state of the resources, exported by the observe reconciler.
//
// From one place, because these are facts the resources already carry and a second exporter
// would reach the same conclusions a beat later. Several of them cannot be reported any
// other way: the node agent is a static pod that must work when the API server does not, so
// it cannot carry a kube-rbac-proxy and cannot be scraped — what it knows arrives here by
// way of the resource it writes.
var (
	// Managed is 1 while the module owns the pull path.
	//
	// Every other metric here is meaningless without it: a cluster that manages nothing
	// has no cache to be incomplete and no nodes to be unreconciled, and an alert firing
	// on that would be firing on a module doing exactly what it was told.
	Managed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_managed",
		Help: "1 while the module manages how the cluster pulls images.",
	})

	// ConfigValid reflects the Valid condition of RegistryConfig.
	ConfigValid = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_config_valid",
		Help: "1 while the registry configuration passed validation.",
	})

	// EffectiveUpstream names the upstream actually in effect, which is not necessarily
	// the configured one: a failed preflight probe holds the cluster on the last one that
	// worked, and nothing in the spec says so.
	EffectiveUpstream = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_effective_upstream",
		Help: "The upstream currently in effect, which may not be the configured one.",
	}, []string{"host"})

	// Nodes is how many nodes have a layout.
	Nodes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_nodes",
		Help: "Nodes with a registry layout.",
	})

	// NodeReconciled is 1 per node whose agent has applied the layout it was given.
	NodeReconciled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_node_reconciled",
		Help: "1 per node whose agent has applied its current layout.",
	}, []string{"node"})

	// NodeFromCache is 1 per node routing from a copy on disk rather than from the
	// cluster's answer.
	//
	// Not a failure: the node pulls images either way, which is the whole point of the
	// copy. It is worth reporting precisely because it looks like success from every
	// other angle.
	NodeFromCache = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_node_layout_from_disk",
		Help: "1 per node whose agent is routing from its on-disk copy because the API server was unreachable.",
	}, []string{"node"})

	// StorageReplicas is how many replicas have reported.
	StorageReplicas = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_storage_replicas",
		Help: "Storage replicas that have reported their state.",
	})

	// StorageReplicaFull is 1 per replica holding the whole expected set.
	StorageReplicaFull = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_storage_replica_full",
		Help: "1 per storage replica holding the whole expected image set.",
	}, []string{"node", "role"})

	// StorageAllReplicasFull is 1 while losing the leader would cost nothing.
	StorageAllReplicasFull = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_storage_all_replicas_full",
		Help: "1 while every storage replica holds the whole expected image set.",
	})

	// StorageSafeToDropUpstream is 1 while the cache could stand alone.
	StorageSafeToDropUpstream = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "d8_registry_storage_safe_to_drop_upstream",
		Help: "1 while the cache holds enough for the upstream to be removed.",
	})

	// StorageStaleData is cache data still on a node that no longer runs a cache.
	//
	// Reported in bytes, because the question it answers is whether the space is worth
	// reclaiming — and a boolean could not tell a few megabytes from a hundred gigabytes.
	StorageStaleData = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_registry_storage_stale_data_bytes",
		Help: "Cache data left on a node while no cache is configured.",
	}, []string{"node"})
)

// Outcomes of a preflight probe.
const (
	OutcomeAccepted    = "accepted"
	OutcomeUnreachable = "unreachable"
	OutcomeAuth        = "auth"
	OutcomeSentinel    = "sentinel"
)

func init() {
	// Registered with the controller-runtime registry, so these are served by the
	// manager's own endpoint alongside the reconcile counters rather than from a second
	// listener.
	metrics.Registry.MustRegister(
		UpstreamProbes, AirGapHeld, UpstreamsRejected,
		Managed, ConfigValid, EffectiveUpstream, Nodes, NodeReconciled, NodeFromCache,
		StorageReplicas, StorageReplicaFull, StorageAllReplicasFull,
		StorageSafeToDropUpstream, StorageStaleData,
	)
}
