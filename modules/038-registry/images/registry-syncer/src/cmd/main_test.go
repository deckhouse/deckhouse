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

package main

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/metrics"
)

// TestResolveLocalAddress pins where a replica looks for its own registry.
//
// The default has to move off the loopback. The pod is host-networked, so the loopback belongs to
// the node, and the registry in the pod binds only the node address: a syncer that keeps the
// default reaches nothing, and the air-gap transition is left unable to count what the store holds.
// While the store and the node agent shared a port number it was the agent that answered there, and
// the syncer was rejected at the TLS handshake instead.
func TestResolveLocalAddress(t *testing.T) {
	tests := []struct {
		name          string
		localAddress  string
		listenAddress string
		want          string
	}{
		{
			name:          "the untouched default becomes the address this replica serves on",
			localAddress:  loopbackRegistry,
			listenAddress: "10.110.0.29",
			want:          "10.110.0.29:4281",
		},
		{
			// Somebody who names the loopback means it: a deployment that is not
			// host-networked has its own registry there and nothing else.
			name:          "the loopback asked for explicitly is left alone",
			localAddress:  "127.0.0.1:5555",
			listenAddress: "10.110.0.29",
			want:          "127.0.0.1:5555",
		},
		{
			name:          "an address given explicitly wins",
			localAddress:  "registry.d8-system.svc:5001",
			listenAddress: "10.110.0.29",
			want:          "registry.d8-system.svc:5001",
		},
		{
			// Nothing to substitute. main() refuses to start in this state anyway; returning
			// the default rather than ":4281" with an empty host is the honest
			// answer here.
			name:          "no listen address to fall back on",
			localAddress:  loopbackRegistry,
			listenAddress: "",
			want:          loopbackRegistry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveLocalAddress(tt.localAddress, tt.listenAddress))
		})
	}
}

// TestWriteEndpointAddressIsNotTheServingListener keeps the two apart.
//
// Everything that modifies the store — a fill and a collection alike — goes to the listener that
// does not proxy. Pointed at the serving one, a fill uploads nothing because the cache answers
// "already have it" from the upstream, and a collection deletes nothing because the proxy store
// answers 405. Both report success over a store that never changed.
func TestWriteEndpointAddressIsNotTheServingListener(t *testing.T) {
	const node = "10.0.0.5"

	write := writeEndpointAddress(node)

	assert.Equal(t, fmt.Sprintf("%s:%d", node, distribution.WriteEndpointPort), write)
	assert.NotEqual(t, resolveLocalAddress(loopbackRegistry, node), write,
		"the address that writes must not be the address that serves")
}

// TestTheLeaderGaugeFollowsTheLease: d8_registry_storage_leader is what
// D8RegistryStorageLeaderAbsent sums across the replicas, so it has to change when the lease
// does, not when a pass next gets round to looking.
//
// It used to be set by the loop, from a reading taken during a pass, and a pass can last an hour.
// A follower that won the lease mid-pass reported 0 until the pass ended and a leader that lost it
// mid-fill reported 1; nothing here runs a pass, so a gauge that moves at all moves with the lease.
// The last case is the elector calling the two callbacks out of order, which it can: "started"
// runs in a goroutine of its own and may arrive after "stopped".
func TestTheLeaderGaugeFollowsTheLease(t *testing.T) {
	gatherer := prometheus.NewRegistry()
	tracker := &leadership{gauge: metrics.New(gatherer).Leader}
	leaderGauge := func() float64 { return scrapeLeaderGauge(t, gatherer) }

	assert.Equal(t, float64(0), leaderGauge(), "a replica that has not won the lease does not lead")

	leading, lost := context.WithCancel(context.Background())
	tracker.started(leading)
	assert.True(t, tracker.IsLeader())
	assert.Equal(t, float64(1), leaderGauge(), "winning the lease shows at once")

	lost()
	tracker.stopped()
	assert.False(t, tracker.IsLeader())
	assert.Equal(t, float64(0), leaderGauge(), "losing the lease shows at once")

	// The elector cancels the context it gave "started" before it calls "stopped", so a "started"
	// that comes last finds its context done and must not undo the "stopped" before it.
	late, cancel := context.WithCancel(context.Background())
	cancel()
	tracker.stopped()
	tracker.started(late)
	assert.False(t, tracker.IsLeader(), "a lease lost before its start was recorded is not held")
	assert.Equal(t, float64(0), leaderGauge(), "nor reported as held")
}

// TestTheElectorCallbacksMoveTheLeaderGauge: the test above calls the tracker directly, so it would
// pass with callbacks that never reach it. This one goes through the callbacks serve hands the
// elector (built by leadership.callbacks), in the order the elector calls them: "started" with the
// term's context, then, once the term is over, the context cancelled and "stopped".
func TestTheElectorCallbacksMoveTheLeaderGauge(t *testing.T) {
	gatherer := prometheus.NewRegistry()
	tracker := &leadership{gauge: metrics.New(gatherer).Leader}
	callbacks := tracker.callbacks(slog.New(slog.DiscardHandler), "master-0")

	term, over := context.WithCancel(context.Background())
	callbacks.OnStartedLeading(term)
	assert.True(t, tracker.IsLeader(), "the elector's start must reach the tracker")
	assert.Equal(t, float64(1), scrapeLeaderGauge(t, gatherer), "and the gauge")

	over()
	callbacks.OnStoppedLeading()
	assert.False(t, tracker.IsLeader(), "the elector's stop must reach the tracker")
	assert.Equal(t, float64(0), scrapeLeaderGauge(t, gatherer), "and the gauge")
}

// scrapeLeaderGauge reads d8_registry_storage_leader back through the registry, the way a scrape
// would.
func scrapeLeaderGauge(t *testing.T, gatherer prometheus.Gatherer) float64 {
	t.Helper()
	families, err := gatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() == "d8_registry_storage_leader" {
			require.Len(t, family.GetMetric(), 1)
			return family.GetMetric()[0].GetGauge().GetValue()
		}
	}
	t.Fatal("d8_registry_storage_leader is not exported")
	return 0
}

// TestTheServingAddressIsNotTheAgents keeps a replica's own address off the node agent's.
//
// Both are on every master. They used to share a port number on two addresses, which held only while
// nothing took that port on all of them — and made the loopback, the obvious default, the agent's.
func TestTheServingAddressIsNotTheAgents(t *testing.T) {
	const node = "10.0.0.5"

	assert.Equal(t, fmt.Sprintf("127.0.0.1:%d", constant.StoragePort), loopbackRegistry,
		"the sentinel is the loopback form of the store's own port")
	assert.Equal(t, fmt.Sprintf("%s:%d", node, constant.StoragePort), resolveLocalAddress(loopbackRegistry, node))
	assert.NotEqual(t, constant.ProxyHost, resolveLocalAddress(loopbackRegistry, node))
	assert.Equal(t, fmt.Sprintf("%s:%d", node, constant.StorageWritePort), writeEndpointAddress(node))
}
