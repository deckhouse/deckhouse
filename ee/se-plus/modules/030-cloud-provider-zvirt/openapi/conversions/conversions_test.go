/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestCloudProviderZvirtConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name:     "empty v1 settings are filled with placeholders",
			settings: `{}`,
			expected: `
provider:
  parameters:
    server: PLACEHOLDER_REPLACE_ME
    clusterID: 00000000-0000-0000-0000-000000000000
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa PLACEHOLDER_REPLACE_ME
storage:
  disabled: false
  parameters: {}
ccm:
  disabled: false
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "already filled v2 settings are left untouched",
			settings: `
provider:
  parameters:
    server: https://zvirt.example.com/ovirt-engine/api
    clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
    insecure: true
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3NzaC1yc2E
storage:
  disabled: false
  parameters:
    excludedStorageClasses:
      - slow-.*
ccm:
  disabled: true
`,
			expected: `
provider:
  parameters:
    server: https://zvirt.example.com/ovirt-engine/api
    clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
    insecure: true
nodes:
  disabled: false
  parameters:
    layout: Standard
    sshPublicKey: ssh-rsa AAAAB3NzaC1yc2E
storage:
  disabled: false
  parameters:
    excludedStorageClasses:
      - slow-.*
ccm:
  disabled: true
`,
			currentVersion:  2,
			expectedVersion: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := conversion.TestConvert(c.settings, c.expected, conversions, c.currentVersion, c.expectedVersion); err != nil {
				t.Error(err)
			}
		})
	}
}
