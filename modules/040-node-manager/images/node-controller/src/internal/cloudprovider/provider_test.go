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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registration.yaml writes some values as bare strings (type: {{ b64enc "aws" }}) and others as
// JSON (zones: {{ toJson | b64enc }}), so every field tries JSON first and falls back to the raw
// bytes. Data keeps the whole Secret decoded — the template render context needs it verbatim.
func TestFromSecretData(t *testing.T) {
	tests := []struct {
		name string
		data map[string][]byte
		want Provider
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
			want: Provider{
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
			want: Provider{
				Type: "dvp",
				CAPI: CAPIConfig{ClusterKind: "DVPCluster"},
			},
		},
		{
			name: "the provider's own tree is keyed by its type",
			data: map[string][]byte{
				"type":    []byte(`vsphere`),
				"vsphere": []byte(`{"instances":{"mainNetwork":"vlan-1"}}`),
			},
			want: Provider{
				Type: "vsphere",
				CloudVariables: map[string]any{
					"instances": map[string]any{"mainNetwork": "vlan-1"},
				},
			},
		},
		{
			name: "empty secret",
			data: map[string][]byte{},
			want: Provider{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromSecretData(tc.data)

			assert.Equal(t, tc.want.Type, got.Type)
			assert.Equal(t, tc.want.InstanceClassKind, got.InstanceClassKind)
			assert.Equal(t, tc.want.InstanceClassAPIVersion, got.InstanceClassAPIVersion)
			assert.Equal(t, tc.want.MachineClassKind, got.MachineClassKind)
			assert.Equal(t, tc.want.CAPI.ClusterKind, got.CAPI.ClusterKind)
			assert.Equal(t, tc.want.Zones, got.Zones)
			assert.Equal(t, tc.want.CloudVariables, got.CloudVariables)

			// Data is the same content, undecoded into fields: every key of the Secret, JSON
			// where the value parses as JSON and the raw string where it does not.
			for k := range tc.data {
				require.Contains(t, got.Data, k)
			}
		})
	}
}

// A CAPI key the provider does not publish defaults here rather than at each use site: an empty
// API version would make the MachineTemplate GVK unparseable.
func TestFromSecretData_CAPIAPIVersionsDefault(t *testing.T) {
	got := FromSecretData(map[string][]byte{"type": []byte(`aws`)})

	assert.Equal(t, defaultInfraAPIVersion, got.CAPI.ClusterAPIVersion)
	assert.Equal(t, defaultInfraAPIVersion, got.CAPI.MachineTemplateAPIVersion)
}
