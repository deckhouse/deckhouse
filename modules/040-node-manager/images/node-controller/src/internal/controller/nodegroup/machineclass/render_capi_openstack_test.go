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

const openstackCAPIMachineTemplatePath = "../../../../../../../../../ee/modules/030-cloud-provider-openstack/capi/machine-template.yaml"

func openstackCAPIRenderContext(instanceClass map[string]interface{}) map[string]interface{} {
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
							"connection":           map[string]interface{}{"region": "RegionOne"},
							"internalNetworkNames": []interface{}{"internal-net"},
							"podNetworkMode":       "DirectRoutingWithPortSecurityEnabled",
							"instances": map[string]interface{}{
								"imageName":      "ubuntu-22",
								"mainNetwork":    "internal-net",
								"sshKeyPairName": "kube-key",
								"securityGroups": []interface{}{"sg-base"},
							},
						},
					},
				},
			},
		},
		"nodeGroup": map[string]interface{}{
			"name":          "worker",
			"instanceClass": instanceClass,
		},
		"zoneName":              "nova",
		"templateName":          "worker-abc12345",
		"instanceClassChecksum": "deadbeef",
	}
}

func openstackCAPIMachineTemplateTags(t *testing.T, ctx map[string]interface{}) []interface{} {
	t.Helper()

	tmpl, err := os.ReadFile(openstackCAPIMachineTemplatePath)
	require.NoError(t, err, "openstack capi/machine-template.yaml must exist")

	out, err := RenderMachineClass(tmpl, ctx)
	require.NoError(t, err)

	var mt map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &mt))

	assert.Equal(t, "OpenStackMachineTemplate", mt["kind"])
	spec := mt["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
	tags, ok := spec["tags"].([]interface{})
	require.True(t, ok, "spec.tags must be a list")
	return tags
}

// TestRenderMachineTemplate_OpenstackCAPI_RawTags verifies that OpenStackInstanceClass.spec.tags
// is emitted verbatim next to the mandatory deckhouse-*/role-*/use-cluster-api tags used by the
// safety controller to identify VMs. If this layout changes, VMs stop being adopted on rollout.
func TestRenderMachineTemplate_OpenstackCAPI_RawTags(t *testing.T) {
	ctx := openstackCAPIRenderContext(map[string]interface{}{
		"flavorName": "m1.large",
		"tags":       []interface{}{"preemptible", "spot"},
	})

	tags := openstackCAPIMachineTemplateTags(t, ctx)

	assert.Contains(t, tags, "preemptible", "raw OpenStack tag must be present as-is (no key=value)")
	assert.Contains(t, tags, "spot")
	assert.Contains(t, tags, "deckhouse-aaaa-bbbb=1", "mandatory safety-controller tag must remain")
	assert.Contains(t, tags, "role-deckhouse-worker-nova=1")
	assert.Contains(t, tags, "use-cluster-api=1")
}

// TestRenderMachineTemplate_OpenstackCAPI_TagsDedup pins that duplicate raw tags collapse to a
// single entry — matches | uniq in the template and matches Nova's own de-duplication behaviour.
func TestRenderMachineTemplate_OpenstackCAPI_TagsDedup(t *testing.T) {
	ctx := openstackCAPIRenderContext(map[string]interface{}{
		"flavorName": "m1.large",
		"tags":       []interface{}{"preemptible", "preemptible"},
	})

	tags := openstackCAPIMachineTemplateTags(t, ctx)

	count := 0
	for _, v := range tags {
		if v == "preemptible" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate raw tags must be de-duplicated in the template")
}

// TestRenderMachineTemplate_OpenstackCAPI_NoTags verifies the field is optional: when the user
// omits tags, only the mandatory ones (plus any additionalTags) are rendered.
func TestRenderMachineTemplate_OpenstackCAPI_NoTags(t *testing.T) {
	ctx := openstackCAPIRenderContext(map[string]interface{}{
		"flavorName": "m1.large",
	})

	tags := openstackCAPIMachineTemplateTags(t, ctx)

	for _, v := range tags {
		assert.NotEqual(t, "preemptible", v, "no raw tag must appear when spec.tags is unset")
	}
	assert.Contains(t, tags, "deckhouse-aaaa-bbbb=1")
	assert.Contains(t, tags, "use-cluster-api=1")
}
