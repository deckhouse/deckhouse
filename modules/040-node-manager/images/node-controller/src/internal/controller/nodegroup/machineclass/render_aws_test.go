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

// awsMachineClassTemplatePath points at the real AWS provider MachineClass
// template the helm node_group_machine_class define renders. The parity test
// renders the exact same file the get_crds path renders, from the same
// internal.cloudProvider tree (= the decoded d8-node-manager-cloud-provider
// Secret), so a divergence here would be the same divergence helm produces.
const awsMachineClassTemplatePath = "../../../../../../../../030-cloud-provider-aws/cloud-instance-manager/machine-class.yaml"

// awsRenderContext builds the tpl context the helm node_group_machine_class define
// passes: Chart (for helm_lib_module_labels), the full Values tree (global.discovery
// + nodeManager.internal.cloudProvider.aws — the decoded cloud-provider Secret),
// nodeGroup (blob element) and zoneName.
func awsRenderContext() map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"global": map[string]interface{}{
				"discovery": map[string]interface{}{
					"clusterUUID": "aaaa-bbbb",
				},
			},
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"type": "aws",
						"aws": map[string]interface{}{
							"region":  "eu-central-1",
							"keyName": "kube-key",
							"instances": map[string]interface{}{
								"ami":                       "ami-default",
								"iamProfileName":            "node-profile",
								"associatePublicIPAddress":  true,
								"additionalSecurityGroups":  []interface{}{"sg-base"},
							},
							"internal": map[string]interface{}{
								"zoneToSubnetIdMap": map[string]interface{}{
									"eu-central-1a": "subnet-aaa",
								},
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
				"instanceType":             "m5.large",
				"additionalSecurityGroups": []interface{}{"sg-extra"},
			},
		},
		"zoneName": "eu-central-1a",
	}
}

// TestRenderMachineClass_AWSByteParity renders the real AWS machine-class.yaml and
// asserts the produced manifest is valid YAML with the fields the template maps
// from the NodeGroup + cloud-provider tree. Parsing (not raw-string compare) keeps
// the assertion robust to insignificant whitespace while still proving the values
// the checksum is computed over land where helm puts them.
func TestRenderMachineClass_AWSByteParity(t *testing.T) {
	tmpl, err := os.ReadFile(awsMachineClassTemplatePath)
	require.NoError(t, err, "AWS machine-class.yaml must exist")

	out, err := RenderMachineClass(tmpl, awsRenderContext())
	require.NoError(t, err)

	var mc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mc), "rendered MachineClass must be valid YAML")

	assert.Equal(t, "machine.sapcloud.io/v1alpha1", mc["apiVersion"])
	assert.Equal(t, "AWSMachineClass", mc["kind"])

	meta := mc["metadata"].(map[string]interface{})
	// name = {ng.name}-{sha256(clusterUUID+zone)[:8]}
	assert.Equal(t, "worker-", meta["name"].(string)[:7])
	assert.Len(t, meta["name"], len("worker-")+8)
	assert.Equal(t, "d8-cloud-instance-manager", meta["namespace"])
	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "deckhouse", labels["heritage"])
	assert.Equal(t, "node-manager", labels["module"])

	spec := mc["spec"].(map[string]interface{})
	assert.Equal(t, "ami-default", spec["ami"], "ami falls back to cloudProvider default")
	assert.Equal(t, "eu-central-1", spec["region"])
	assert.Equal(t, "m5.large", spec["machineType"])
	assert.Equal(t, "kube-key", spec["keyName"])
	assert.Equal(t, "node-profile", spec["iam"].(map[string]interface{})["name"])

	nifs := spec["networkInterfaces"].([]interface{})
	nif := nifs[0].(map[string]interface{})
	assert.Equal(t, "subnet-aaa", nif["subnetID"], "subnetID resolved from zoneToSubnetIdMap")
	assert.Equal(t, true, nif["associatePublicIPAddress"])
	// securityGroupIDs = uniq(base ∪ instanceClass)
	assert.ElementsMatch(t, []interface{}{"sg-base", "sg-extra"}, nif["securityGroupIDs"])

	// secretRef name == metadata.name (same hash idiom)
	secretRef := spec["secretRef"].(map[string]interface{})
	assert.Equal(t, meta["name"], secretRef["name"])
	assert.Equal(t, "d8-cloud-instance-manager", secretRef["namespace"])
}
