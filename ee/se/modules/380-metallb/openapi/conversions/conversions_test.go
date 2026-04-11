/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestMetallbConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should convert from 1 to 2 version (remove layer2 pools)",
			settings: `
addressPools:
  - addresses:
    - 192.168.199.100-192.168.199.102
    name: frontend-pool-l2
    protocol: layer2
  - addresses:
    - 192.168.200.100-192.168.200.102
    name: frontend-pool-bgp
    protocol: bgp
`,
			expected: `
addressPools:
  - addresses:
    - 192.168.200.100-192.168.200.102
    name: frontend-pool-bgp
    protocol: bgp
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should convert from 2 to 3 version (remove bgp settings)",
			settings: `
bgpPeers:
  - my-asn: 65000
    peer-address: 192.168.1.1
    peer-asn: 65001
bgpCommunities:
  no-advertise: "65535:65282"
addressPools:
  - addresses:
    - 10.0.0.1/32
    name: bgp-pool
    protocol: bgp
  - addresses:
    - 10.0.0.2/32
    name: l2-pool
    protocol: layer2
`,
			expected: `
{}
`,
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name: "should jump from 1 to 3 version (remove all obsolete settings)",
			settings: `
bgpPeers:
  - my-asn: 65000
    peer-address: 192.168.1.1
    peer-asn: 65001
addressPools:
  - addresses:
    - 192.168.199.100-192.168.199.102
    name: l2-pool
    protocol: layer2
  - addresses:
    - 10.0.0.1/32
    name: bgp-pool
    protocol: bgp
speaker:
  nodeSelector:
    node: test
`,
			expected: `
speaker:
  nodeSelector:
    node: test
`,
			currentVersion:  1,
			expectedVersion: 3,
		},
		{
			name: "should convert to 3 version without addressPools",
			settings: `
speaker:
  nodeSelector:
    node: test
`,
			expected: `
speaker:
  nodeSelector:
    node: test
`,
			currentVersion:  2,
			expectedVersion: 3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := conversion.TestConvert(c.settings, c.expected, conversions, c.currentVersion, c.expectedVersion)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
