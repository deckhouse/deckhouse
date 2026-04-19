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

type testCase struct {
	name            string
	settings        string
	expected        string
	currentVersion  int
	expectedVersion int
}

func TestIstioConversions(t *testing.T) {
	conversionPath := "."

	tests := []testCase{
		{
			name: "Move enableHTTP10 to dataPlane",
			settings: `
field1: 123
enableHTTP10: True
`,
			expected: `
field1: 123
dataPlane:
  enableHTTP10: True
`,
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name: "Move proxyConfig to dataPlane",
			settings: `
field1: 123
proxyConfig:
  holdApplicationUntilProxyStarts: True
  idleTimeout: 10s
`,
			expected: `
field1: 123
dataPlane:
  proxyConfig:
    holdApplicationUntilProxyStarts: True
    idleTimeout: 10s
`,
			currentVersion:  2,
			expectedVersion: 3,
		},
		{
			name: "Should convert from 1 to 2 version",
			settings: `
auth:
  password: password
  allowedUserGroups:
    - group
    - group2
  whitelistSourceRange:
    - source1
`,
			expected: `
auth:
  allowedUserGroups:
    - group
    - group2
  whitelistSourceRange:
    - source1
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := conversion.TestConvert(tc.settings, tc.expected, conversionPath, tc.currentVersion, tc.expectedVersion)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
