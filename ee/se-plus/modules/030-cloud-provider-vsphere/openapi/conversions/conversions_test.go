/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestOpenVpnConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should convert from 1 to 2 version and capitalize the value of the storageClass.compatibilityFlag field",
			settings: `
storageClass:
  exclude: qwerty123
  compatibilityFlag: migration
`,
			expected: `
storageClass:
  exclude: qwerty123
  compatibilityFlag: Migration
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should convert from 1 to 2 version and do nothing if the storageClass.compatibilityFlag field does not exists",
			settings: `
storageClass:
  exclude: qwerty123
`,
			expected: `
storageClass:
  exclude: qwerty123
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
