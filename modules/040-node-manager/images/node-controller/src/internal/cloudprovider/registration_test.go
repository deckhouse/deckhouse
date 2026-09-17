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

func TestDecodeRegistrationAcceptsProviderEncodings(t *testing.T) {
	registration, err := DecodeRegistration(map[string][]byte{
		"type":                           []byte("openstack"),
		"region":                         []byte(`"RegionOne"`),
		"zones":                          []byte(`["zone-a","zone-b"]`),
		"instanceClassKind":              []byte("OpenStackInstanceClass"),
		"instanceClassAPIVersion":        []byte(`"v1"`),
		"machineClassKind":               []byte("OpenStackMachineClass"),
		"sshPublicKey":                   []byte("ssh-ed25519 AAAA"),
		"capiClusterName":                []byte("openstack"),
		"capiClusterKind":                []byte(`"OpenStackCluster"`),
		"capiClusterAPIVersion":          []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
		"capiMachineTemplateKind":        []byte("OpenStackMachineTemplate"),
		"capiMachineTemplateAPIVersion":  []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
		"capiMachineDeploymentSpecPatch": []byte("spec: {}"),
		"openstack":                      []byte(`{"connection":{"region":"RegionOne"}}`),
	})
	require.NoError(t, err)

	assert.Equal(t, "openstack", registration.Type)
	assert.Equal(t, "RegionOne", registration.Region)
	assert.Equal(t, []string{"zone-a", "zone-b"}, registration.Zones)
	assert.Equal(t, "OpenStackInstanceClass", registration.InstanceClassKind)
	assert.Equal(t, "v1", registration.InstanceClassAPIVersion)
	assert.Equal(t, "OpenStackMachineClass", registration.MachineClassKind)
	assert.Equal(t, "ssh-ed25519 AAAA", registration.SSHPublicKey)
	assert.Equal(t, "openstack", registration.CAPIClusterName)
	assert.Equal(t, "OpenStackCluster", registration.CAPIClusterKind)
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1beta1", registration.CAPIClusterAPIVersion)
	assert.Equal(t, "OpenStackMachineTemplate", registration.CAPIMachineTemplateKind)
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1beta1", registration.CAPIMachineTemplateAPIVersion)
	assert.Equal(t, "spec: {}", registration.CAPIMachineDeploymentSpecPatch)

	connection, ok := registration.CloudVariables["connection"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "RegionOne", connection["region"])
}

func TestDecodeRegistrationWithoutProviderSubtreeUsesEmptyMap(t *testing.T) {
	registration, err := DecodeRegistration(map[string][]byte{"type": []byte("dvp")})
	require.NoError(t, err)
	assert.Nil(t, registration.CloudVariables)
}

func TestDecodeRegistrationRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeRegistration(map[string][]byte{"zones": []byte("not-json")})
	require.ErrorContains(t, err, "zones")

	_, err = DecodeRegistration(map[string][]byte{
		"type": []byte("dvp"),
		"dvp":  []byte("not-json"),
	})
	require.ErrorContains(t, err, "dvp subtree")
}

func TestRegistrationValidation(t *testing.T) {
	registration := Registration{
		Type: "dvp", Region: "default", Zones: []string{"default"},
		InstanceClassKind: "DVPInstanceClass", InstanceClassAPIVersion: "v1alpha1",
		CAPIClusterName: "dvp", CAPIClusterKind: "DeckhouseCluster",
		CAPIClusterAPIVersion:         "infrastructure.cluster.x-k8s.io/v1alpha1",
		CAPIMachineTemplateKind:       "DeckhouseMachineTemplate",
		CAPIMachineTemplateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
		CloudVariables:                map[string]any{"project": "test"},
	}
	require.NoError(t, registration.ValidateCore())
	require.NoError(t, registration.ValidateCAPI())

	registration.Region = ""
	require.ErrorContains(t, registration.ValidateCore(), "region")
	registration.Region = "default"
	registration.CAPIMachineTemplateKind = ""
	require.ErrorContains(t, registration.ValidateCAPI(), "capiMachineTemplateKind")
	registration.CAPIMachineTemplateKind = "DeckhouseMachineTemplate"
	registration.Zones = []string{""}
	require.ErrorContains(t, registration.ValidateCore(), "zones")
	registration.Zones = []string{"default"}
	registration.CloudVariables = map[string]any{}
	require.NoError(t, registration.ValidateCore(), "an empty provider object is valid for providers without settings")
	registration.Type = "DVP"
	require.ErrorContains(t, registration.ValidateCore(), "must be lowercase")
}
