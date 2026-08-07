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

func TestCloudProviderYandexConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "full v1 settings: move additionalExternalNetworkIDs, migrate storageClass.exclude, fill provider and node placeholders",
			settings: `
additionalExternalNetworkIDs:
  - enp1
  - enp2
storageClass:
  default: network-hdd
  exclude:
    - network-ssd-.*
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters:
    additionalExternalNetworkIDs:
      - enp1
      - enp2
storage:
  disabled: false
  parameters:
    excludedStorageClasses:
      - network-ssd-.*
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
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "only additionalExternalNetworkIDs in v1: migrate to ccm, synthesize provider/nodes/storage",
			settings: `
additionalExternalNetworkIDs:
  - enp1
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters:
    additionalExternalNetworkIDs:
      - enp1
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "only storageClass in v1: migrate exclude to storage.parameters, drop default, synthesize provider/nodes/ccm",
			settings: `
storageClass:
  default: network-hdd
  exclude:
    - network-ssd-.*
    - network-hdd-.*
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters:
    excludedStorageClasses:
      - network-ssd-.*
      - network-hdd-.*
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "storageClass with only default in v1: default is dropped, storage.parameters stays empty",
			settings: `
storageClass:
  default: network-hdd
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "storageClass with only exclude in v1: migrate excludedStorageClasses",
			settings: `
storageClass:
  exclude:
    - network-ssd-.*
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters:
    excludedStorageClasses:
      - network-ssd-.*
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "empty additionalExternalNetworkIDs in v1: migrate empty list to ccm, no storageClass",
			settings: `
additionalExternalNetworkIDs: []
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters:
    additionalExternalNetworkIDs: []
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "empty storageClass object in v1: no sub-fields to migrate, storage.parameters stays empty",
			settings: `
storageClass: {}
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "both v1 fields with empty collections: empty list and empty object",
			settings: `
additionalExternalNetworkIDs: []
storageClass: {}
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters:
    additionalExternalNetworkIDs: []
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "null storageClass in v1: skipped safely, no storageClass sub-fields to migrate",
			settings: `
storageClass: null
`,
			expected: `
provider:
  parameters:
    cloudID: PLACEHOLDER_REPLACE_ME
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "already filled provider parameters: existing cloudID and folderID are preserved",
			settings: `
provider:
  parameters:
    cloudID: b1gxxxxxxxxxxxxxxxxx
    folderID: b1fxxxxxxxxxxxxxxxxx
`,
			expected: `
provider:
  parameters:
    cloudID: b1gxxxxxxxxxxxxxxxxx
    folderID: b1fxxxxxxxxxxxxxxxxx
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "partially filled provider parameters: only the missing field gets a placeholder",
			settings: `
provider:
  parameters:
    cloudID: b1gxxxxxxxxxxxxxxxxx
`,
			expected: `
provider:
  parameters:
    cloudID: b1gxxxxxxxxxxxxxxxxx
    folderID: PLACEHOLDER_REPLACE_ME
nodes:
  disabled: false
  parameters:
    layout: Standard
    nodeNetworkCIDR: 10.0.0.0/16
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
ccm:
  disabled: false
  parameters: {}
storage:
  disabled: false
  parameters: {}
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
