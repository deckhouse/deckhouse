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
			name: "full v1 settings: move namespace, drop kubeconfigDataBase64, move zones, fill layout placeholder",
			settings: `
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: my-ns
zones:
  - zone-a
  - zone-b
`,
			expected: `
provider:
  parameters:
    namespace: my-ns
nodes:
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
    zones:
      - zone-a
      - zone-b
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "empty v1 settings: synthesize defaults for required v2 fields",
			settings: `{}
`,
			expected: `
provider:
  parameters:
    namespace: default
nodes:
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "only provider in v1: keep namespace, drop kubeconfigDataBase64, synthesize nodes",
			settings: `
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: ns1
`,
			expected: `
provider:
  parameters:
    namespace: ns1
nodes:
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "only zones in v1: synthesize provider, move zones into nodes",
			settings: `
zones:
  - z1
`,
			expected: `
provider:
  parameters:
    namespace: default
nodes:
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
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
