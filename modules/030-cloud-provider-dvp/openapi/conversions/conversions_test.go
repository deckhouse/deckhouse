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

func TestCloudProviderDvpConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should convert provider.namespace, drop kubeconfigDataBase64, move zones",
			settings: `
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: default
zones:
  - zone-a
  - zone-b
`,
			expected: `
provider:
  parameters:
    namespace: default
nodes:
  parameters:
    zones:
      - zone-a
      - zone-b
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should drop kubeconfigDataBase64 only",
			settings: `
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: ns1
`,
			expected: `
provider:
  parameters:
    namespace: ns1
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should convert zones only when provider absent",
			settings: `
zones:
  - z1
`,
			expected: `
nodes:
  parameters:
    zones:
      - z1
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
