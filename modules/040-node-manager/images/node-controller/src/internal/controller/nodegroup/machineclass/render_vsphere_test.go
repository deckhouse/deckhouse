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

package machineclass

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// vsphere lives in ee/se-plus/modules — one level higher than the CE providers.
const vsphereMachineClassTemplatePath = "../../../../../../../../../ee/se-plus/modules/030-cloud-provider-vsphere/cloud-instance-manager/machine-class.yaml"

func vsphereRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{"clusterUUID": "aaaa-bbbb"},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "vsphere",
						"vsphere": map[string]interface{}{
							"region":       "datacenter-1",
							"vmFolderPath": "kube/vms",
							"sshKey":       "ssh-rsa AAAA",
							"instanceClassDefaults": map[string]interface{}{
								"template":        "base-tmpl",
								"datastore":       "ds1",
								"resourcePoolPath": "",
								"disableTimesync": false,
							},
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name": "worker",
			"instanceClass": map[string]interface{}{
				"numCPUs":     float64(4),
				"memory":      float64(8192),
				"mainNetwork": "vlan-100",
			},
		},
		"zoneName": "zone-a",
	}
}

// TestRenderMachineClass_VsphereByteParity renders the real vsphere
// machine-class.yaml, exercising the arithmetic pipeline (memory rounding via
// add+mod, memoryReservation via mul/div), the runtimeOptions default block, and
// the template/datastore cloudProvider fallbacks.
func TestRenderMachineClass_VsphereByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(vsphereMachineClassTemplatePath)
	require.NoError(t, err, "vsphere machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, vsphereRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc))

	assert.Equal(t, "VsphereMachineClass", mc["kind"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "datacenter-1", spec["region"])
	assert.Equal(t, "zone-a", spec["zone"])
	assert.EqualValues(t, 4, spec["numCPUs"])
	// memory = 8192 + (8192 mod 4) = 8192
	assert.EqualValues(t, 8192, spec["memory"])
	assert.EqualValues(t, 20, spec["rootDiskSize"], "rootDiskSize default 20")
	assert.Equal(t, "base-tmpl", spec["template"], "template falls back to cloudProvider default")
	assert.Equal(t, "kube/vms", spec["virtualMachineFolder"])
	assert.Equal(t, "vlan-100", spec["mainNetwork"])
	assert.Equal(t, "ds1", spec["datastore"], "datastore falls back to cloudProvider default")

	rt := spec["runtimeOptions"].(map[string]interface{})
	assert.Equal(t, true, rt["nestedHardwareVirtualization"], "runtimeOptions default block")
	// memoryReservation = (memory / 100) * 80 = (8192/100)*80 = 81*80 = 6480 (integer div)
	rai := rt["resourceAllocationInfo"].(map[string]interface{})
	assert.EqualValues(t, 6480, rai["memoryReservation"])
}
