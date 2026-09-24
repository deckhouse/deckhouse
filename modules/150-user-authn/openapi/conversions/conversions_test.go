/*
Copyright 2024 Flant JSC

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

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestUserAuthnConversions(t *testing.T) {
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
publishAPI:
  enable: true
`,
			expected: `
publishAPI:
  enabled: true
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name:            "should convert idTokenTTL 168h from 2 to 3 version",
			settings:        "idTokenTTL: 168h\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 6h from 2 to 3 version",
			settings:        "idTokenTTL: 6h\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 24h0m0s from 2 to 3 version",
			settings:        "idTokenTTL: 24h0m0s\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 360m from 2 to 3 version",
			settings:        "idTokenTTL: 360m\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 21600s from 2 to 3 version",
			settings:        "idTokenTTL: 21600s\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 5h59m from 2 to 3 version",
			settings:        "idTokenTTL: 5h59m\n",
			expected:        "idTokenTTL: 5h59m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 5h59m59s from 2 to 3 version",
			settings:        "idTokenTTL: 5h59m59s\n",
			expected:        "idTokenTTL: 5h59m59s\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 359m from 2 to 3 version",
			settings:        "idTokenTTL: 359m\n",
			expected:        "idTokenTTL: 359m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 1h from 2 to 3 version",
			settings:        "idTokenTTL: 1h\n",
			expected:        "idTokenTTL: 1h\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should convert idTokenTTL 10m from 2 to 3 version",
			settings:        "idTokenTTL: 10m\n",
			expected:        "idTokenTTL: 10m\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should keep settings without idTokenTTL from 2 to 3 version",
			settings:        "publishAPI:\n  enabled: true\n",
			expected:        "publishAPI:\n  enabled: true\n",
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name:            "should apply both conversions from 1 to 3 version",
			settings:        "publishAPI:\n  enable: true\nidTokenTTL: 168h\n",
			expected:        "publishAPI:\n  enabled: true\nidTokenTTL: 5h59m\n",
			currentVersion:  1,
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
