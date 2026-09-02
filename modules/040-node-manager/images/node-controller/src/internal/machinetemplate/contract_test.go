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
	"os"
	"path/filepath"
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
			name:     "additionalFields value does not parse",
			contract: "version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    failureDomain: \"{{ .zone\"\ntemplate: |\n  kind: X\n",
			expError: "machineDeployment.additionalFields[failureDomain]",
		},
		{
			name:     "additionalFields value uses a nondeterministic function",
			contract: "version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    failureDomain: \"{{ now }}\"\ntemplate: |\n  kind: X\n",
			expError: "function \"now\" not defined",
		},
		{
			name:     "additionalFields may not touch infrastructureRef",
			contract: "version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    \"infrastructureRef.name\": hijacked\ntemplate: |\n  kind: X\n",
			expError: "belongs to node-controller",
		},
		{
			name:     "additionalFields may not touch bootstrap",
			contract: "version: v2\nrolloutFields: [a]\nmachineDeployment:\n  additionalFields:\n    \"bootstrap.dataSecretName\": mine\ntemplate: |\n  kind: X\n",
			expError: "belongs to node-controller",
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

// ApplyMachineDeploymentFields is the second half of what a provider file produces: the fields
// node-controller writes into the generic MachineDeployment it builds.
func TestApplyMachineDeploymentFields(t *testing.T) {
	contract, err := ParseContract([]byte(`version: v2
rolloutFields: [flavorName]
machineDeployment:
  additionalFields:
    failureDomain: "{{ .zone }}"
    "template.metadata.labels.zone": "zone-{{ .zone }}"
template: |
  apiVersion: v1
  kind: X
  spec: {}
`))
	require.NoError(t, err)

	spec := map[string]any{
		"template": map[string]any{"spec": map[string]any{"clusterName": "openstack"}},
	}
	require.NoError(t, ApplyMachineDeploymentFields(spec, contract, testRenderContext()))

	assert.Equal(t, map[string]any{
		"template": map[string]any{
			"spec": map[string]any{
				"clusterName":   "openstack",
				"failureDomain": "ru-1a",
				"template":      map[string]any{"metadata": map[string]any{"labels": map[string]any{"zone": "zone-ru-1a"}}},
			},
		},
	}, spec)
}

func TestApplyMachineDeploymentFieldsWithoutAny(t *testing.T) {
	contract, err := ParseContract([]byte(minimalContract))
	require.NoError(t, err)

	spec := map[string]any{"replicas": int64(1)}
	require.NoError(t, ApplyMachineDeploymentFields(spec, contract, testRenderContext()))
	assert.Equal(t, map[string]any{"replicas": int64(1)}, spec)
}

// Every provider contract shipped in the repository must parse. This is the check a new provider
// gets for free — the parity fixtures below have to be added by hand, this one does not.
func TestShippedProviderContractsParse(t *testing.T) {
	patterns := []string{
		"../../../../../../030-cloud-provider-*/capi/template.yaml",
		"../../../../../../../ee/modules/030-cloud-provider-*/capi/template.yaml",
		"../../../../../../../ee/se-plus/modules/030-cloud-provider-*/capi/template.yaml",
	}

	found := 0
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		for _, path := range matches {
			t.Run(filepath.Base(filepath.Dir(filepath.Dir(path))), func(t *testing.T) {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				contract, err := ParseContract(data)
				require.NoError(t, err, "a shipped provider contract must be valid")
				assert.NotEmpty(t, contract.RolloutFields)
			})
			found++
		}
	}
	assert.GreaterOrEqual(t, found, 7, "every in-tree CAPI provider ships a v2 contract")
}
