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

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestLogShipperConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should move flat static cpu/memory into static.requests",
			settings: `
resourcesRequests:
  mode: Static
  static:
    cpu: 0.2
    memory: 1.5Gi
`,
			expected: `
resourcesRequests:
  mode: Static
  static:
    requests:
      cpu: 0.2
      memory: 1.5Gi
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should not touch VPA mode settings",
			settings: `
resourcesRequests:
  mode: VPA
  vpa:
    cpu:
      min: 50m
      max: 2
    memory:
      min: 256Mi
      max: 2Gi
`,
			expected: `
resourcesRequests:
  mode: VPA
  vpa:
    cpu:
      min: 50m
      max: 2
    memory:
      min: 256Mi
      max: 2Gi
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
