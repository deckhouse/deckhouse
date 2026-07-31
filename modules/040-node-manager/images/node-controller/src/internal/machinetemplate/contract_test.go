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

package machinetemplate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalContract = `
version: v2
rolloutFields:
  - flavorName
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackMachineTemplate
  spec:
    template:
      spec:
        flavor: {{ .instanceClass.flavorName }}
`

func TestParseContract(t *testing.T) {
	contract, err := ParseContract([]byte(minimalContract))
	require.NoError(t, err)
	assert.Equal(t, ContractVersionV2, contract.Version)
	assert.Equal(t, []string{"flavorName"}, contract.RolloutFields)
}

func TestParseContractRejects(t *testing.T) {
	tests := []struct {
		name     string
		contract string
		expError string
	}{
		{
			name:     "no version",
			contract: "rolloutFields: [a]\ntemplate: |\n  kind: X\n",
			expError: "unsupported machine-template contract version",
		},
		{
			name:     "future version",
			contract: "version: v3\nrolloutFields: [a]\ntemplate: |\n  kind: X\n",
			expError: "unsupported machine-template contract version \"v3\"",
		},
		{
			name:     "empty template",
			contract: "version: v2\nrolloutFields: [a]\ntemplate: \"\"\n",
			expError: "empty template",
		},
		{
			name:     "no rolloutFields",
			contract: "version: v2\ntemplate: |\n  kind: X\n",
			expError: "must state which InstanceClass fields recreate machines",
		},
		{
			name:     "duplicate rolloutField",
			contract: "version: v2\nrolloutFields: [a, a]\ntemplate: |\n  kind: X\n",
			expError: "duplicate field \"a\"",
		},
		{
			name:     "malformed field path",
			contract: "version: v2\nrolloutFields: [\"a..b\"]\ntemplate: |\n  kind: X\n",
			expError: "malformed field path",
		},
		{
			name:     "unknown additionalFields source",
			contract: "version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    failureDomain: region\ntemplate: |\n  kind: X\n",
			expError: "unknown source \"region\"",
		},
		{
			name:     "unknown top-level key",
			contract: "version: v2\nrolloutFields: [a]\nrolloutField: [b]\ntemplate: |\n  kind: X\n",
			expError: "unknown field",
		},
		{
			name:     "template does not parse",
			contract: "version: v2\nrolloutFields: [a]\ntemplate: |\n  kind: {{ .a\n",
			expError: "parse machine template",
		},
		{
			name:     "template uses a nondeterministic function",
			contract: "version: v2\nrolloutFields: [a]\ntemplate: |\n  kind: {{ now }}\n",
			expError: "function \"now\" not defined",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseContract([]byte(tc.contract))
			require.ErrorContains(t, err, tc.expError)
		})
	}
}

// The contract file is what a provider ships and what node-controller trusts, so a template that
// is only broken for some inputs must still be caught at load time — parsing happens in
// ParseContract, not at the first render in a cluster.
func TestParseContractParsesTemplateEagerly(t *testing.T) {
	_, err := ParseContract([]byte("version: v2\nrolloutFields: [a]\ntemplate: |\n  kind: {{ if }}\n"))
	require.ErrorContains(t, err, "parse machine template")
}

func TestParseContractAcceptsZoneAdditionalField(t *testing.T) {
	contract, err := ParseContract([]byte(
		"version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    failureDomain: zone\ntemplate: |\n  kind: X\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"failureDomain": "zone"}, contract.MachineDeployment.AdditionalFields)
}
