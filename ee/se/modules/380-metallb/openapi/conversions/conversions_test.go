/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestPrometheusConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should convert from 1 to 2 version",
			settings: `
addressPools:
  - addresses:
    - 192.168.199.100-192.168.199.102
    name: frontend-pool1
    protocol: layer2
  - addresses:
    - 192.168.200.100-192.168.200.102
    name: frontend-pool2
    protocol: bgp
`,
			expected: `
addressPools:
  - addresses:
    - 192.168.200.100-192.168.200.102
    name: frontend-pool2
    protocol: bgp
`,
			currentVersion:  1,
			expectedVersion: 2,
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
