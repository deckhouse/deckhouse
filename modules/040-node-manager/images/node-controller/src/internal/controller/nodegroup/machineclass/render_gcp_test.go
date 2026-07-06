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

const gcpMachineClassTemplatePath = "../../../../../../../../030-cloud-provider-gcp/cloud-instance-manager/machine-class.yaml"

// gcpRenderContext mirrors the helm tpl context for gcp. serviceAccountJSON is a
// JSON string the template feeds through fromJson to extract client_email — the
// case that exercises the machineclass FuncMap's fromJson.
func gcpRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{"clusterUUID": "aaaa-bbbb"},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "gcp",
						"gcp": map[string]interface{}{
							"region":            "europe-west1",
							"diskSizeGb":        float64(30),
							"diskType":          "pd-ssd",
							"image":             "img-default",
							"serviceAccountJSON": `{"client_email":"sa@project.iam.gserviceaccount.com"}`,
							"networkName":       "kube-net",
							"subnetworkName":    "kube-subnet",
							"disableExternalIP": true,
							"labels":            map[string]interface{}{"team": "platform"},
							"sshKey":            "ssh-rsa AAAA",
							"networkTags":       []interface{}{"tag-a"},
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name": "worker",
			"instanceClass": map[string]interface{}{
				"machineType": "n1-standard-4",
			},
		},
		"zoneName": "europe-west1-b",
	}
}

// TestRenderMachineClass_GCPByteParity renders the real gcp machine-class.yaml,
// exercising fromJson (serviceAccountJSON→client_email), cloudProvider disk/image
// fallbacks, and the bool disableExternalIP.
func TestRenderMachineClass_GCPByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(gcpMachineClassTemplatePath)
	require.NoError(t, err, "gcp machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, gcpRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc))

	assert.Equal(t, "GCPMachineClass", mc["kind"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "europe-west1", spec["region"])
	assert.Equal(t, "europe-west1-b", spec["zone"])
	assert.Equal(t, "n1-standard-4", spec["machineType"])

	disks := spec["disks"].([]interface{})
	boot := disks[0].(map[string]interface{})
	assert.EqualValues(t, 30, boot["sizeGb"], "sizeGb falls back to cloudProvider default")
	assert.Equal(t, "pd-ssd", boot["type"])
	assert.Equal(t, "img-default", boot["image"])

	sas := spec["serviceAccounts"].([]interface{})
	sa := sas[0].(map[string]interface{})
	assert.Equal(t, "sa@project.iam.gserviceaccount.com", sa["email"],
		"client_email extracted via fromJson from serviceAccountJSON")

	nifs := spec["networkInterfaces"].([]interface{})
	nif := nifs[0].(map[string]interface{})
	assert.Equal(t, "kube-net", nif["network"])
	assert.Equal(t, "kube-subnet", nif["subnetwork"])
	assert.Equal(t, true, nif["disableExternalIP"])
}
