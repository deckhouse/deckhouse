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

const yandexCAPIMachineTemplatePath = "../../../../../../../../030-cloud-provider-yandex/capi/machine-template.yaml"

// yandexCAPIRenderContext mirrors the helm capi_node_group_machine_template tpl
// context: the same cloudProvider tree plus the reconciler-supplied templateName
// and instanceClassChecksum. The node-controller renders this same template via
// the shared RenderMachineClass engine instead of helm.
func yandexCAPIRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "yandex",
						"yandex": map[string]interface{}{
							"instanceClassDefaults": map[string]interface{}{
								"imageID": "img-default",
							},
							"zoneToSubnetIdMap": map[string]interface{}{
								"ru-central1-a": "subnet-a",
							},
							"shouldAssignPublicIPAddress": false,
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
		"zoneName":              "ru-central1-a",
		"templateName":          "worker-abc12345",
		"instanceClassChecksum": "deadbeef",
	}
}

// TestRenderMachineTemplate_YandexCAPIByteParity renders the real yandex CAPI
// machine-template.yaml through the shared engine, asserting the reconciler-owned
// fields (templateName, checksum/instance-class annotation) and the two-argument
// helm_lib_module_labels form (node-group label) the template uses.
func TestRenderMachineTemplate_YandexCAPIByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(yandexCAPIMachineTemplatePath)
	require.NoError(t, err, "yandex capi/machine-template.yaml must exist")

	out, err := RenderMachineClass(tmpl, yandexCAPIRenderContext())
	require.NoError(t, err)

	var mt map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mt))

	assert.Equal(t, "YandexMachineTemplate", mt["kind"])

	meta := mt["metadata"].(map[string]interface{})
	assert.Equal(t, "worker-abc12345", meta["name"], "name = reconciler templateName")

	ann := meta["annotations"].(map[string]interface{})
	assert.Equal(t, "deadbeef", ann["checksum/instance-class"], "instance-class checksum annotation")
	assert.Equal(t, "keep", ann["helm.sh/resource-policy"])

	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "deckhouse", labels["heritage"])
	assert.Equal(t, "node-manager", labels["module"])
	assert.Equal(t, "worker", labels["node-group"], "two-arg helm_lib_module_labels adds node-group")

	spec := mt["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
	assert.Equal(t, "ru-central1-a", spec["zoneID"])
	assert.Equal(t, "standard-v3", spec["platformID"], "platformID default")

	res := spec["resources"].(map[string]interface{})
	assert.EqualValues(t, 4, res["cores"])
	assert.EqualValues(t, 100, res["coreFraction"], "coreFraction default")
	assert.Equal(t, "8192Mi", res["memory"], "memory printf %dMi")
	assert.EqualValues(t, 0, res["gpus"], "gpus default")

	boot := spec["bootDisk"].(map[string]interface{})
	assert.Equal(t, "network-hdd", boot["typeID"], "diskType default")
	assert.Equal(t, "50Gi", boot["size"], "diskSizeGB default 50Gi")
	assert.Equal(t, "img-default", boot["imageID"], "imageID falls back to cloudProvider default")

	nifs := spec["networkInterfaces"].([]interface{})
	nif := nifs[0].(map[string]interface{})
	assert.Equal(t, "subnet-a", nif["subnetID"], "subnet resolved from zoneToSubnetIdMap")
	assert.Equal(t, false, nif["hasPublicIP"], "falls back to shouldAssignPublicIPAddress")

	tmeta := spec["metadata"].(map[string]interface{})
	assert.Equal(t, "user:ssh-rsa AAAA", tmeta["ssh-keys"])
	assert.Equal(t, "10.0.0.0/16", tmeta["node-network-cidr"])
}
