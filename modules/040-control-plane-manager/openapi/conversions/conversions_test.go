package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

func TestConversions(t *testing.T) {
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
apiserver:
  auditPolicyEnabled: true
  basicAuditPolicyEnabled: true
etcd:
  maxDbSize: 536870912
  externalMembersNames:
    - main-master-1
    - my-external-member
`,
			expected: `
apiserver:
  auditPolicyEnabled: true
  basicAuditPolicyEnabled: true
etcd:
  maxDbSize: 536870912
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
		{
			name: "should convert from 1 to 2 version: etcd {}",
			settings: `
apiserver:
  auditPolicyEnabled: true
  basicAuditPolicyEnabled: true
etcd:
  externalMembersNames:
    - main-master-1
    - my-external-member
`,
			expected: `
apiserver:
  auditPolicyEnabled: true
  basicAuditPolicyEnabled: true
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
