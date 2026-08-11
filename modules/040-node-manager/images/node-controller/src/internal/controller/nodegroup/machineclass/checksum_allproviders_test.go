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
)

// allChecksumTemplates lists every checksum template still shipped in the repository: the MCM
// ones. The checksum names the MCM MachineClass, which is immutable — a checksum that moves
// renames it and rolls every node in the group. Providers without a helm-era golden (see
// TestRenderChecksum_HelmEraGoldens) are covered here structurally instead.
//
// The CAPI providers are no longer here: they ship the v2 contract (capi/template.yaml), where
// "what rolls machines" is a declared field list compared by value rather than a hash over bytes.
// Their guarantee moved to internal/machinetemplate's parity harness, which still renders the v1
// files (kept as a snapshot in its testdata) and requires the v2 answer to match them.
var allChecksumTemplates = map[string]string{
	"aws-mcm":       "../../../../../../../../030-cloud-provider-aws/cloud-instance-manager/machine-class.checksum",
	"azure-mcm":     "../../../../../../../../030-cloud-provider-azure/cloud-instance-manager/machine-class.checksum",
	"gcp-mcm":       "../../../../../../../../030-cloud-provider-gcp/cloud-instance-manager/machine-class.checksum",
	"yandex-mcm":    "../../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.checksum",
	"openstack-mcm": "../../../../../../../../../ee/modules/030-cloud-provider-openstack/cloud-instance-manager/machine-class.checksum",
	"vsphere-mcm":   "../../../../../../../../../ee/se-plus/modules/030-cloud-provider-vsphere/cloud-instance-manager/machine-class.checksum",
}

// everyProviderField is the union of the instanceClass fields the templates above read. Numbers
// are float64 because that is what the blob carries — see TestChecksumNumbersMustBeFloat64.
func everyProviderField() map[string]interface{} {
	return map[string]interface{}{
		"ami": "ami-1", "instanceType": "t3.large", "spot": true, "iops": float64(3000),
		"diskSizeGb": float64(42), "diskSizeGB": float64(42), "diskType": "gp3",
		"machineType": "n1-standard-4", "image": "img", "imageName": "ubuntu", "imageID": "img-1",
		"flavorName": "m1.large", "preemptible": true, "platformID": "standard-v3",
		"cores": float64(4), "coreFraction": float64(50), "memory": float64(8), "gpus": float64(1),
		"numCPUs": float64(3), "rootDiskSize": float64(50), "rootDiskSizeGb": float64(50),
		"rootDiskType": "ssd", "template": "tmpl", "datastore": "lun-1",
		"mainNetwork": "net", "mainSubnet": "subnet", "externalNetwork": "ext",
		"additionalNetworks": []interface{}{"a", "b"}, "additionalSubnets": []interface{}{"c"},
		"subnets": []interface{}{"d"}, "securityGroups": []interface{}{"sg"},
		"additionalSecurityGroups": []interface{}{"sg2"},
		"additionalTags":           map[string]interface{}{"k": "v"},
		"additionalLabels":         map[string]interface{}{"k": "v"},
		"assignPublicIPAddress":    true, "networkType": "standard",
		"serverGroupID": "sg-1", "vipAddress": "10.0.0.1", "vnicProfileID": "vnic-1",
		"sizingPolicy": "policy", "storageProfile": "profile", "placementPolicy": "pp",
		"runtimeOptions": map[string]interface{}{"nestedHardwareVirtualization": true},
		"rootDisk": map[string]interface{}{
			"image":        map[string]interface{}{"kind": "ClusterVirtualImage", "name": "ubuntu"},
			"size":         "50Gi",
			"storageClass": "sc",
		},
		"virtualMachine": map[string]interface{}{
			"virtualMachineClassName": "class",
			"bootloader":              "EFI",
			"cpu":                     map[string]interface{}{"coreFraction": "50%", "cores": float64(4)},
			"memory":                  map[string]interface{}{"size": "8Gi"},
		},
	}
}

func minimalCloudProvider() map[string]interface{} {
	// Only vcd reads .Values…cloudProvider; the others ignore the argument.
	return map[string]interface{}{"vcd": map[string]interface{}{"server": "https://localhost"}}
}

// TestRenderChecksum_AllProviderTemplates is the structural guard for every provider, including
// the ones with no helm-era golden: each template must render, must be deterministic, and must
// depend on nothing outside instanceClass and manualRolloutID. A NodeGroup rename or a version
// bump leaking into the checksum would roll every node in the group.
func TestRenderChecksum_AllProviderTemplates(t *testing.T) {
	for name, path := range allChecksumTemplates {
		t.Run(name, func(t *testing.T) {
			tmpl, err := os.ReadFile(path)
			require.NoError(t, err, "provider checksum template must exist")

			ic := everyProviderField()
			first, err := RenderChecksum(tmpl, buildChecksumElement(ic, "rollout-1"), minimalCloudProvider())
			require.NoError(t, err, "template must render from a blob-shaped instanceClass")
			assert.Len(t, first, 64, "sha256sum output, no trailing whitespace")

			second, err := RenderChecksum(tmpl, buildChecksumElement(everyProviderField(), "rollout-1"), minimalCloudProvider())
			require.NoError(t, err)
			assert.Equal(t, first, second, "checksum must be a pure function of its inputs")

			noisy := buildChecksumElement(everyProviderField(), "rollout-1")
			noisy["name"] = "worker"
			noisy["nodeType"] = "CloudEphemeral"
			noisy["kubernetesVersion"] = "1.34"
			noisy["cri"] = map[string]interface{}{"type": "Containerd"}
			noisy["zones"] = []interface{}{"a", "b"}
			noisy["updateEpoch"] = "1746532947"
			withNoise, err := RenderChecksum(tmpl, noisy, minimalCloudProvider())
			require.NoError(t, err)
			assert.Equal(t, first, withNoise,
				"only instanceClass and manualRolloutID may affect the checksum — anything else rolls nodes on unrelated edits")

			changed := everyProviderField()
			changed["manualRolloutID"] = "ignored-here"
			rolled, err := RenderChecksum(tmpl, buildChecksumElement(changed, "rollout-2"), minimalCloudProvider())
			require.NoError(t, err)
			assert.NotEqual(t, first, rolled, "manualRolloutID must still force a rollout")
		})
	}
}

// TestChecksumNumbersMustBeFloat64 pins the contract that makes the templates safe: the blob
// decodes instanceClass through json.Unmarshal into interface{} (derived_status.rawExtensionToValue),
// so every number is float64 — the same type helm's values carried. Several templates compare
// against float literals (`ne .diskSizeGB 50.0`), and Go templates refuse to compare int64 with
// float64. The failure is loud rather than silent, which is what keeps a decode change from
// quietly producing a different checksum and rolling every node.
func TestChecksumNumbersMustBeFloat64(t *testing.T) {
	tmpl, err := os.ReadFile(allChecksumTemplates["yandex-mcm"])
	require.NoError(t, err)

	instanceClass := func(disk interface{}) map[string]interface{} {
		return map[string]interface{}{
			"platformID": "standard-v3", "cores": float64(4), "memory": float64(8),
			"diskType": "network-ssd", "imageID": "img", "diskSizeGB": disk,
		}
	}

	atDefault, err := RenderChecksum(tmpl, buildChecksumElement(instanceClass(float64(50)), ""), nil)
	require.NoError(t, err)
	aboveDefault, err := RenderChecksum(tmpl, buildChecksumElement(instanceClass(float64(60)), ""), nil)
	require.NoError(t, err)
	assert.NotEqual(t, atDefault, aboveDefault, "the default-size branch must be observable")

	_, err = RenderChecksum(tmpl, buildChecksumElement(instanceClass(int64(50)), ""), nil)
	require.Error(t, err, "an int64 must fail loudly instead of silently changing the checksum")
	assert.Contains(t, err.Error(), "incompatible types for comparison")
}
