// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// nodeWithAddress answers the three commands collectHostNetworkState runs, for a node holding one
// address on eth0.
func nodeWithAddress(cidr string, prefixLen int) *fakeNode {
	addresses := `[{"ifname":"eth0","addr_info":[{"family":"inet","local":"` + cidr + `","prefixlen":` +
		jsonInt(prefixLen) + `}]}]`

	return newFakeNode().
		on("ip -j address show").prints(addresses).
		on("ip -j -4 route show table all").prints("[]").
		on("ip -j -6 route show table all").prints("[]").
		on("cat /etc/resolv.conf").prints("nameserver 8.8.8.8\n")
}

func jsonInt(v int) string {
	encoded, _ := json.Marshal(v)
	return string(encoded)
}

func internalNetworkMetaConfig(t *testing.T, cidrs ...string) *config.MetaConfig {
	t.Helper()
	meta := &config.MetaConfig{}
	if len(cidrs) > 0 {
		encoded, err := json.Marshal(cidrs)
		require.NoError(t, err)
		meta.StaticClusterConfig = map[string]json.RawMessage{"internalNetworkCIDRs": encoded}
	}
	return meta
}

// TestNodeInternalNetwork covers the failure that used to surface as "Discovered NodeIP is empty.
// Check out your StaticClusterConfiguration internalNetworkCIDRs" — after the tunnels were up and
// 01-bootstrap-prerequisites had run, and without printing a single address of the node, because
// the diagnostics that would have run under `|| true` and went to a stream nobody sees.
func TestNodeInternalNetwork(t *testing.T) {
	tests := []struct {
		name          string
		cidrs         []string
		node          *fakeNode
		notApplicable bool
		wantErr       string
		wantDetail    string
	}{
		{
			name:       "the node is on the declared network",
			cidrs:      []string{"192.168.1.0/24"},
			node:       nodeWithAddress("192.168.1.15", 24),
			wantDetail: "has 192.168.1.15, inside internalNetworkCIDRs entry 192.168.1.0/24",
		},
		{
			name:       "one of several networks matches",
			cidrs:      []string{"10.10.0.0/16", "192.168.1.0/24"},
			node:       nodeWithAddress("192.168.1.15", 24),
			wantDetail: "192.168.1.0/24",
		},
		{
			// The message prints the addresses, which is the whole point: bashible never did.
			name:    "the node is on a different network",
			cidrs:   []string{"10.10.0.0/16"},
			node:    nodeWithAddress("192.168.1.15", 24),
			wantErr: "the node has 192.168.1.15, and none of them is inside 10.10.0.0/16",
		},
		{
			// A single-interface node picks its own address, which is what no declaration means.
			name:          "nothing declared",
			node:          nodeWithAddress("192.168.1.15", 24),
			notApplicable: true,
		},
		{
			name:    "a declaration that is not a CIDR",
			cidrs:   []string{"192.168.1.0"},
			node:    nodeWithAddress("192.168.1.15", 24),
			wantErr: `"192.168.1.0" is not a CIDR`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeInternalNetworkCheck{
				MetaConfig:    internalNetworkMetaConfig(t, tt.cidrs...),
				NodeInterface: FixedNodeInterface(tt.node),
			}

			detail, err := check.Run(t.Context())

			switch {
			case tt.wantErr != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			case tt.notApplicable:
				assert.ErrorIs(t, err, preflight.ErrNotApplicable)
			default:
				require.NoError(t, err)
				assert.Contains(t, detail, tt.wantDetail)
			}
		})
	}
}
