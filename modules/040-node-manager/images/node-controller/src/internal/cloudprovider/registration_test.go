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
	registration := DecodeRegistration(map[string][]byte{
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
	registration := DecodeRegistration(map[string][]byte{"type": []byte("dvp")})
	assert.Equal(t, map[string]any{}, registration.CloudVariables)
}
