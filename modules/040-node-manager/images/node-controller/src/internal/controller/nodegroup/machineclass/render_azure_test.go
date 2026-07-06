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

const azureMachineClassTemplatePath = "../../../../../../../../030-cloud-provider-azure/cloud-instance-manager/machine-class.yaml"

func azureRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{"clusterUUID": "aaaa-bbbb"},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "azure",
						"azure": map[string]interface{}{
							"location":          "westeurope",
							"resourceGroupName": "kube-rg",
							"vnetName":          "kube-vnet",
							"subnetName":        "kube-subnet",
							"additionalTags":    map[string]interface{}{"env": "prod"},
							"urn":               "Canonical:0001:22_04-lts:latest",
							"diskType":          "Premium_LRS",
							"sshPublicKey":      "ssh-rsa AAAA",
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name": "worker",
			"instanceClass": map[string]interface{}{
				"machineSize": "Standard_D4s_v3",
			},
		},
		"zoneName": "1",
	}
}

// TestRenderMachineClass_AzureByteParity renders the real azure machine-class.yaml,
// exercising mergeOverwrite (additionalTags), dig (acceleratedNetworking default
// true) and the urn/diskType cloudProvider fallbacks.
func TestRenderMachineClass_AzureByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(azureMachineClassTemplatePath)
	require.NoError(t, err, "azure machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, azureRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc))

	assert.Equal(t, "AzureMachineClass", mc["kind"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "westeurope", spec["location"])
	assert.Equal(t, "kube-rg", spec["resourceGroup"])

	subnet := spec["subnetInfo"].(map[string]interface{})
	assert.Equal(t, "kube-vnet", subnet["vnetName"])
	assert.Equal(t, "kube-subnet", subnet["subnetName"])

	tags := spec["tags"].(map[string]interface{})
	assert.Equal(t, "prod", tags["env"], "additionalTags merged via mergeOverwrite")
	assert.Equal(t, "1", tags["kubernetes.io-cluster-aaaa-bbbb"])

	props := spec["properties"].(map[string]interface{})
	assert.Equal(t, "Standard_D4s_v3", props["hardwareProfile"].(map[string]interface{})["vmSize"])

	storage := props["storageProfile"].(map[string]interface{})
	assert.Equal(t, "Canonical:0001:22_04-lts:latest",
		storage["imageReference"].(map[string]interface{})["urn"], "urn falls back to cloudProvider")
	osDisk := storage["osDisk"].(map[string]interface{})
	assert.Equal(t, "Premium_LRS", osDisk["managedDisk"].(map[string]interface{})["storageAccountType"])
	assert.EqualValues(t, 50, osDisk["diskSizeGB"], "diskSizeGb default 50")

	net := props["networkProfile"].(map[string]interface{})
	assert.Equal(t, true, net["acceleratedNetworking"], "dig default true when unset")
	// zone is rendered unquoted ({{ .zoneName }}), so YAML parses "1" as a number
	assert.EqualValues(t, 1, props["zone"])
}
