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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/registry-syncer/internal/distribution"
)

// TestResolveLocalAddress pins where a replica looks for its own registry.
//
// The default has to move off the loopback. The pod is host-networked, so the loopback belongs to
// the node, and on a Managed cluster the registry AGENT is already listening there on the same
// port. A syncer that keeps the default therefore talks to the agent instead of the registry in
// its own pod and is rejected at the TLS handshake — measured on a cluster, where it left the
// air-gap transition unable to count what the store held.
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
			want:          "10.110.0.29:5001",
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
			// the default rather than "10.110.0.29:5001" with an empty host is the honest
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
