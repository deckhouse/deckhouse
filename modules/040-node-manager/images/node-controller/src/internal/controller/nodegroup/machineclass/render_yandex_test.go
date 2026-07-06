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

const yandexMachineClassTemplatePath = "../../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.yaml"

// yandexRenderContext mirrors the helm node_group_machine_class tpl context for
// yandex: the decoded cloud-provider Secret lives under
// nodeManager.internal.cloudProvider.yandex.
func yandexRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{"clusterUUID": "aaaa-bbbb"},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "yandex",
						"yandex": map[string]interface{}{
							"region": "ru-central1",
							"instanceClassDefaults": map[string]interface{}{
								"imageID": "img-default",
							},
							"zoneToSubnetIdMap": map[string]interface{}{
								"ru-central1-a": "subnet-a",
							},
							"shouldAssignPublicIPAddress": false,
							"labels":                      map[string]interface{}{"team": "platform"},
							"sshKey":                      "ssh-rsa AAAA",
							"nodeNetworkCIDR":             "10.0.0.0/16",
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name": "worker",
			"instanceClass": map[string]interface{}{
				"cores":  float64(4),
				"memory": float64(8192),
			},
		},
		"zoneName": "ru-central1-a",
	}
}

// TestRenderMachineClass_YandexByteParity renders the real yandex
// machine-class.yaml and checks the fields the template maps, including the sprig
// arithmetic (memory MiB→bytes, disk GiB→bytes) and the subnet lookup.
func TestRenderMachineClass_YandexByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(yandexMachineClassTemplatePath)
	require.NoError(t, err, "yandex machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, yandexRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc))

	assert.Equal(t, "YandexMachineClass", mc["kind"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "ru-central1", spec["regionID"])
	assert.Equal(t, "ru-central1-a", spec["zoneID"])
	assert.Equal(t, "standard-v3", spec["platformID"], "platformID default")

	res := spec["resourcesSpec"].(map[string]interface{})
	assert.EqualValues(t, 4, res["cores"])
	assert.EqualValues(t, 100, res["coreFraction"], "coreFraction default")
	assert.EqualValues(t, 8192*1024*1024, res["memory"], "memory MiB→bytes via mul")

	boot := spec["bootDiskSpec"].(map[string]interface{})
	assert.EqualValues(t, 50*1024*1024*1024, boot["size"], "disk default 50GiB→bytes")
	assert.Equal(t, "img-default", boot["imageID"], "imageID falls back to cloudProvider default")

	nifs := spec["networkInterfaceSpecs"].([]interface{})
	nif := nifs[0].(map[string]interface{})
	assert.Equal(t, "subnet-a", nif["subnetID"], "subnet resolved from zoneToSubnetIdMap")
	assert.Equal(t, false, nif["assignPublicIPAddress"], "falls back to shouldAssignPublicIPAddress")

	labels := spec["labels"].(map[string]interface{})
	assert.Equal(t, "platform", labels["team"], "cloudProvider labels merged in")

	meta := spec["metadata"].(map[string]interface{})
	assert.Equal(t, "user:ssh-rsa AAAA", meta["ssh-keys"])
	assert.Equal(t, "10.0.0.0/16", meta["node-network-cidr"])
}
