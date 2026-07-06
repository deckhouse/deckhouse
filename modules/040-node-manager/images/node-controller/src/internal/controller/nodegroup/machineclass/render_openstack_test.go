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

// openstack lives in ee/modules — one level higher than the CE providers.
const openstackMachineClassTemplatePath = "../../../../../../../../../ee/modules/030-cloud-provider-openstack/cloud-instance-manager/machine-class.yaml"

func openstackRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{
					"clusterUUID": "aaaa-bbbb",
					"podSubnet":   "10.111.0.0/16",
				},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "openstack",
						"openstack": map[string]interface{}{
							"connection":          map[string]interface{}{"region": "RegionOne"},
							"externalNetworkDHCP": false,
							"internalNetworkNames": []interface{}{"internal-net"},
							"podNetworkMode":      "DirectRoutingWithPortSecurityEnabled",
							"instances": map[string]interface{}{
								"imageName":      "ubuntu-22",
								"mainNetwork":    "internal-net",
								"sshKeyPairName": "kube-key",
								"securityGroups": []interface{}{"sg-base"},
							},
							"tags": map[string]interface{}{"env": "prod"},
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name": "worker",
			"instanceClass": map[string]interface{}{
				"flavorName":               "m1.large",
				"additionalSecurityGroups": []interface{}{"sg-extra"},
			},
		},
		"zoneName": "nova",
	}
}

// TestRenderMachineClass_OpenstackByteParity renders the real openstack
// machine-class.yaml, exercising useConfigDrive (externalNetworkDHCP=false), the
// network prepend/uniq + podNetwork logic, security-group concat/uniq, and the
// tag merge.
func TestRenderMachineClass_OpenstackByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(openstackMachineClassTemplatePath)
	require.NoError(t, err, "openstack machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, openstackRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc))

	assert.Equal(t, "OpenStackMachineClass", mc["kind"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "RegionOne", spec["region"])
	assert.Equal(t, "nova", spec["availabilityZone"])
	assert.Equal(t, true, spec["useConfigDrive"], "set when externalNetworkDHCP is false")
	assert.Equal(t, "m1.large", spec["flavorName"])
	assert.Equal(t, "ubuntu-22", spec["imageName"], "imageName from cloudProvider instances")
	assert.Equal(t, "10.111.0.0/16", spec["podNetworkCidr"])
	assert.Equal(t, "kube-key", spec["keyName"])

	nets := spec["networks"].([]interface{})
	net := nets[0].(map[string]interface{})
	assert.Equal(t, "internal-net", net["name"])
	assert.Equal(t, true, net["podNetwork"], "internal net + DirectRouting → podNetwork:true")

	sgs := spec["securityGroups"].([]interface{})
	assert.ElementsMatch(t, []interface{}{"sg-base", "sg-extra"}, sgs)

	tags := spec["tags"].(map[string]interface{})
	assert.Equal(t, "prod", tags["env"])
	assert.Equal(t, "1", tags["kubernetes.io-cluster-deckhouse-aaaa-bbbb"])
}
