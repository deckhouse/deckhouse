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

package cloudprovider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// registration.yaml writes some values as bare strings (type: {{ b64enc "aws" }}) and others as
// JSON (zones: {{ toJson | b64enc }}). The decoder must accept both forms for every field, which is
// why the map decoder it replaces tried JSON first and fell back to the raw string.
func TestFromSecretData_BothEncodings(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string][]byte
		expProvider Provider
	}{
		{
			name: "bare strings as helm writes them",
			data: map[string][]byte{
				"type":                    []byte(`aws`),
				"instanceClassKind":       []byte(`AWSInstanceClass`),
				"instanceClassAPIVersion": []byte(`v1`),
				"machineClassKind":        []byte(`AWSMachineClass`),
				"zones":                   []byte(`["a","b"]`),
			},
			expProvider: Provider{
				Type:                    "aws",
				InstanceClassKind:       "AWSInstanceClass",
				InstanceClassAPIVersion: "v1",
				MachineClassKind:        "AWSMachineClass",
				Zones:                   []string{"a", "b"},
			},
		},
		{
			name: "json-quoted strings as the tests write them",
			data: map[string][]byte{
				"type":            []byte(`"dvp"`),
				"capiClusterKind": []byte(`"DVPCluster"`),
			},
			expProvider: Provider{
				Type: "dvp",
				CAPI: CAPIConfig{ClusterKind: "DVPCluster"},
			},
		},
		{
			name:        "empty secret",
			data:        map[string][]byte{},
			expProvider: Provider{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromSecretData(tc.data)

			require.Equal(t, tc.expProvider.Type, got.Type)
			require.Equal(t, tc.expProvider.InstanceClassKind, got.InstanceClassKind)
			require.Equal(t, tc.expProvider.InstanceClassAPIVersion, got.InstanceClassAPIVersion)
			require.Equal(t, tc.expProvider.MachineClassKind, got.MachineClassKind)
			require.Equal(t, tc.expProvider.CAPI.ClusterKind, got.CAPI.ClusterKind)
			require.Equal(t, tc.expProvider.Zones, got.Zones)
		})
	}
}

func TestFromSecretData_CloudVariablesKeyedByType(t *testing.T) {
	data := map[string][]byte{
		"type":    []byte(`vsphere`),
		"vsphere": []byte(`{"instances":{"mainNetwork":"vlan-1"}}`),
	}

	got := FromSecretData(data)

	require.Equal(t, "vsphere", got.Type)
	instances, ok := got.CloudVariables["instances"].(map[string]any)
	require.True(t, ok, "cloud variables must be keyed by the provider type")
	require.Equal(t, "vlan-1", instances["mainNetwork"])
}
