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

// CAPI instance-class.checksum parity for non-yandex CAPI providers (yandex is
// in checksum_test.go). Each case pins which instanceClass fields feed the hash.
const (
	dvpCAPIChecksumPath         = "../../../../../../../../030-cloud-provider-dvp/capi/instance-class.checksum"
	dynamixCAPIChecksumPath     = "../../../../../../../../../ee/modules/030-cloud-provider-dynamix/capi/instance-class.checksum"
	huaweicloudCAPIChecksumPath = "../../../../../../../../../ee/modules/030-cloud-provider-huaweicloud/capi/instance-class.checksum"
	openstackCAPIChecksumPath   = "../../../../../../../../../ee/modules/030-cloud-provider-openstack/capi/instance-class.checksum"
	vcdCAPIChecksumPath         = "../../../../../../../../../ee/modules/030-cloud-provider-vcd/capi/instance-class.checksum"
	zvirtCAPIChecksumPath       = "../../../../../../../../../ee/se-plus/modules/030-cloud-provider-zvirt/capi/instance-class.checksum"
)

func TestRenderChecksum_CAPIProviderParity(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		blob        map[string]interface{}
		wantOptions map[string]interface{}
	}{
		{
			name: "dvp: nested virtualMachine/rootDisk fields, additionalDisks nil",
			path: dvpCAPIChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"virtualMachine": map[string]interface{}{
						"virtualMachineClassName": "vmc",
						"bootloader":              "EFI",
						"memory":                  map[string]interface{}{"size": "8Gi"},
						"cpu":                     map[string]interface{}{"cores": float64(4), "coreFraction": "100%"},
					},
					"rootDisk": map[string]interface{}{
						"size":         "50Gi",
						"storageClass": "linstor",
						"image":        map[string]interface{}{"kind": "ClusterVirtualImage", "name": "ubuntu"},
					},
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"vmClassName":          "vmc",
				"bootloader":           "EFI",
				"memory":               "8Gi",
				"rootDiskSize":         "50Gi",
				"rootDiskStorageClass": "linstor",
				"vmBootloader":         "EFI",
				"cores":                float64(4),
				"coreFraction":         "100%",
				"osImageRefKind":       "ClusterVirtualImage",
				"rootDiskImageName":    "ubuntu",
				"additionalDisks":      nil,
			},
		},
		{
			name: "dynamix: rootDiskSizeGb/externalNetwork nil when absent",
			path: dynamixCAPIChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"imageName": "img",
					"numCPUs":   float64(4),
					"memory":    float64(8192),
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"imageName":       "img",
				"numCPUs":         float64(4),
				"memory":          float64(8192),
				"rootDiskSizeGb":  nil,
				"externalNetwork": nil,
			},
		},
		{
			name: "huaweicloud: unset networking keys nil",
			path: huaweicloudCAPIChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"imageName":    "img",
					"flavorName":   "m1.large",
					"rootDiskSize": float64(40),
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"imageName":          "img",
				"flavorName":         "m1.large",
				"rootDiskSize":       float64(40),
				"rootDiskType":       nil,
				"subnets":            nil,
				"mainNetwork":        nil,
				"additionalNetworks": nil,
				"securityGroups":     nil,
				"serverGroupID":      nil,
				"vipAddress":         nil,
			},
		},
		{
			name: "openstack: truthy-gated optionals, manualRolloutID included",
			path: openstackCAPIChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"flavorName":   "m1.large",
					"imageName":    "ubuntu",
					"rootDiskSize": float64(30),
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"flavorName":      "m1.large",
				"imageName":       "ubuntu",
				"rootDiskSize":    float64(30),
				"manualRolloutID": "r1",
			},
		},
		{
			name: "zvirt: vnicProfileID/rootDiskSizeGb nil when absent",
			path: zvirtCAPIChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"template": "t1",
					"numCPUs":  float64(4),
					"memory":   float64(8192),
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"template":       "t1",
				"vnicProfileID":  nil,
				"numCPUs":        float64(4),
				"memory":         float64(8192),
				"rootDiskSizeGb": nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := os.ReadFile(tc.path)
			require.NoError(t, err, "provider CAPI checksum template must exist")

			got, err := RenderChecksum(tmpl, tc.blob)
			require.NoError(t, err)

			want := expectedChecksum(t, tc.wantOptions)
			assert.Len(t, got, 64, "sha256sum output is 64 hex chars")
			assert.Equal(t, want, got)
		})
	}
}

// vcd folds cloudProvider.vcd.metadata into the hash and reads .Values, so it
// goes through RenderChecksumWithContext (as the CAPI reconciler does).
func TestRenderChecksum_CAPIVcdParity(t *testing.T) {
	tmpl, err := os.ReadFile(vcdCAPIChecksumPath)
	require.NoError(t, err, "vcd CAPI checksum template must exist")

	ctx := map[string]interface{}{
		"nodeGroup": map[string]interface{}{
			"instanceClass": map[string]interface{}{
				"storageProfile": "sp1",
				"template":       "tmpl-1",
				"rootDiskSizeGb": float64(40),
			},
			"manualRolloutID": "r1",
		},
		"Values": map[string]interface{}{
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": map[string]interface{}{
						"vcd": map[string]interface{}{
							"metadata": map[string]interface{}{"owner": "team-x"},
						},
					},
				},
			},
		},
	}

	got, err := RenderChecksumWithContext(tmpl, ctx)
	require.NoError(t, err, "vcd checksum must render once .Values is supplied")

	want := expectedChecksum(t, map[string]interface{}{
		"storageProfile":  "sp1",
		"template":        "tmpl-1",
		"rootDiskSize":    float64(40),
		"sizingPolicy":    nil,
		"placementPolicy": nil,
		"metadata":        map[string]interface{}{"owner": "team-x"},
		"manualRolloutID": "r1",
	})
	assert.Len(t, got, 64)
	assert.Equal(t, want, got)
}
