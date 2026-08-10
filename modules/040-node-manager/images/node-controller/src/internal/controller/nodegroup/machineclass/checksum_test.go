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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const awsChecksumTemplatePath = "../../../../../../../../030-cloud-provider-aws/cloud-instance-manager/machine-class.checksum"

const yandexMCMChecksumTemplatePath = "../../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.checksum"

func expectedChecksum(t *testing.T, options map[string]interface{}) string {
	t.Helper()
	raw, err := yaml.Marshal(options)
	require.NoError(t, err)
	input := strings.TrimSuffix(string(raw), "\n") + "\n"
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func TestRenderChecksum_AWSParity(t *testing.T) {
	tmpl, err := os.ReadFile(awsChecksumTemplatePath)
	require.NoError(t, err, "provider checksum template must exist")

	blobElement := map[string]interface{}{
		"instanceClass": map[string]interface{}{
			"ami":          "ami-0abc123",
			"instanceType": "m5.large",
			"spot":         true,
			"diskSizeGb":   float64(50),
			"diskType":     "gp3",
		},
		"manualRolloutID": "rollout-42",
	}

	got, err := RenderChecksum(tmpl, blobElement, nil)
	require.NoError(t, err)

	want := expectedChecksum(t, map[string]interface{}{
		"ami":             "ami-0abc123",
		"instanceType":    "m5.large",
		"spot":            true,
		"diskSizeGb":      float64(50),
		"diskType":        "gp3",
		"manualRolloutID": "rollout-42",
	})

	assert.Len(t, got, 64, "sha256sum output is 64 hex chars, no trailing whitespace")
	assert.Equal(t, want, got)
}

func TestRenderChecksum_AWSDefaultDiskSizeExcluded(t *testing.T) {
	tmpl, err := os.ReadFile(awsChecksumTemplatePath)
	require.NoError(t, err)

	withDefault := map[string]interface{}{
		"instanceClass": map[string]interface{}{
			"instanceType": "m5.large",
			"diskSizeGb":   float64(20),
		},
	}
	withoutDisk := map[string]interface{}{
		"instanceClass": map[string]interface{}{
			"instanceType": "m5.large",
		},
	}

	a, err := RenderChecksum(tmpl, withDefault, nil)
	require.NoError(t, err)
	b, err := RenderChecksum(tmpl, withoutDisk, nil)
	require.NoError(t, err)

	assert.Equal(t, b, a, "default diskSizeGb=20 is excluded, so checksum must not change")
}

func TestChecksumDependsOnlyOnInstanceClassAndRollout(t *testing.T) {
	awsTmpl, err := os.ReadFile(awsChecksumTemplatePath)
	require.NoError(t, err)
	yandexTmpl, err := os.ReadFile(yandexMCMChecksumTemplatePath)
	require.NoError(t, err)

	cases := []struct {
		name string
		tmpl []byte
		ic   map[string]interface{}
	}{
		{
			name: "aws-mcm",
			tmpl: awsTmpl,
			ic: map[string]interface{}{
				"ami":          "ami-0abc123",
				"instanceType": "m5.large",
				"diskSizeGb":   float64(50),
			},
		},
		{
			name: "yandex-mcm",
			tmpl: yandexTmpl,
			ic: map[string]interface{}{
				"platformID": "standard-v3",
				"cores":      float64(4),
				"memory":     float64(8589934592),
				"diskType":   "network-ssd",
				"imageID":    "img-abc",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			minimal := buildChecksumElement(tc.ic, "rollout-9")

			full := map[string]interface{}{
				"instanceClass":     tc.ic,
				"manualRolloutID":   "rollout-9",
				"name":              "worker",
				"nodeType":          "CloudEphemeral",
				"cri":               map[string]interface{}{"type": "Containerd"},
				"zones":             []interface{}{"a", "b"},
				"kubernetesVersion": "1.29",
			}

			gotMinimal, err := RenderChecksum(tc.tmpl, minimal, nil)
			require.NoError(t, err)
			gotFull, err := RenderChecksum(tc.tmpl, full, nil)
			require.NoError(t, err)

			assert.Equal(t, gotFull, gotMinimal,
				"only instanceClass + manualRolloutID may affect the checksum")
		})
	}
}

// TestRenderChecksum_HelmEraGoldens pins the exact checksums the helm implementation produced,
// copied from the hook test this code replaced (hooks/machineclass_checksum_assign_test.go, its
// nodeManagerAWS/nodeManagerGCP fixtures). They are literals on purpose: the sibling parity tests
// recompute the template's own serialization, so a change in how the blob is marshalled — int64
// instead of float64 out of unstructured, a dropped key, a different toYAML — would move both
// sides of those assertions together and stay green. These constants cannot move with the code.
//
// Important! The checksum names the MCM MachineClass, which is immutable: a changed checksum
// renames it, which rolls the MachineDeployment and recreates every VM in it. A failure here means
// an upgrade would reboot every node in the group.
//
// The CAPI goldens that used to live here (vcd, dvp) are gone with the v1 CAPI files themselves:
// those providers ship the v2 contract now, where names carry a generation counter instead of a
// hash and "did anything change" is decided by comparing values. What replaced this guarantee is
// internal/machinetemplate's parity harness — it renders the archived v1 files and requires the v2
// decision to match them field by field.
func TestRenderChecksum_HelmEraGoldens(t *testing.T) {
	cases := []struct {
		name            string
		path            string
		instanceClass   map[string]interface{}
		cloudProvider   map[string]interface{}
		manualRolloutID string
		want            string
	}{
		{
			name: "aws",
			path: awsChecksumTemplatePath,
			instanceClass: map[string]interface{}{
				"ami":          "myami",
				"diskSizeGb":   float64(50),
				"diskType":     "gp2",
				"iops":         float64(42),
				"instanceType": "t2.medium",
			},
			want: "21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19",
		},
		{
			name: "aws with manual rollout ID",
			path: awsChecksumTemplatePath,
			instanceClass: map[string]interface{}{
				"ami":          "myami",
				"diskSizeGb":   float64(50),
				"diskType":     "gp2",
				"iops":         float64(42),
				"instanceType": "t2.medium",
			},
			manualRolloutID: "test",
			want:            "7b787b33650a0f9166b6eacfdaff5d7c1e0cc508d2831d392ca938e47b7460f6",
		},
		{
			name: "gcp",
			path: gcpMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":  "m1.large",
				"imageName":   "ubuntu-18-04-cloud-amd64",
				"machineType": "mymachinetype",
				"preemptible": true,
				"diskType":    "superdisk",
				"diskSizeGb":  float64(42),
			},
			want: "a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae",
		},
		{
			name: "gcp with manual rollout ID",
			path: gcpMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":  "m1.large",
				"imageName":   "ubuntu-18-04-cloud-amd64",
				"machineType": "mymachinetype",
				"preemptible": true,
				"diskType":    "superdisk",
				"diskSizeGb":  float64(42),
			},
			manualRolloutID: "test",
			want:            "48aa95710a1ea40e5dc26d36a8a0b2d461a85e4fc47953e94a84cef64a4060ca",
		},
		{
			name: "azure",
			path: azureMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":  "m1.large",
				"imageName":   "ubuntu-18-04-cloud-amd64",
				"machineType": "mymachinetype",
				"preemptible": true,
				"diskType":    "superdisk",
				"diskSizeGb":  float64(42),
			},
			want: "22501f2cc926a805859128046cf1b739f224eda731be0a7f93e0715c0b5ff1d3",
		},
		{
			name: "yandex",
			path: yandexMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":            "m1.large",
				"imageName":             "ubuntu-18-04-cloud-amd64",
				"platformID":            "myplaid",
				"cores":                 float64(42),
				"coreFraction":          float64(50),
				"memory":                float64(42),
				"gpus":                  float64(2),
				"imageID":               "myimageid",
				"preemptible":           true,
				"diskType":              "ssd",
				"diskSizeGB":            float64(42),
				"assignPublicIPAddress": true,
				"mainSubnet":            "mymainsubnet",
				"additionalSubnets":     []interface{}{"aaa", "bbb"},
				"additionalLabels":      map[string]interface{}{"my": "label"},
			},
			want: "e8f505559b08cf2de57171d574feae2b258c66d9adf83808fc173e70cb006c47",
		},
		{
			name: "yandex with manual rollout ID",
			path: yandexMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":            "m1.large",
				"imageName":             "ubuntu-18-04-cloud-amd64",
				"platformID":            "myplaid",
				"cores":                 float64(42),
				"coreFraction":          float64(50),
				"memory":                float64(42),
				"gpus":                  float64(2),
				"imageID":               "myimageid",
				"preemptible":           true,
				"diskType":              "ssd",
				"diskSizeGB":            float64(42),
				"assignPublicIPAddress": true,
				"mainSubnet":            "mymainsubnet",
				"additionalSubnets":     []interface{}{"aaa", "bbb"},
				"additionalLabels":      map[string]interface{}{"my": "label"},
			},
			manualRolloutID: "test",
			want:            "d0de381052e706a0e28a9b2cfde60ed2e29854900549ef253d1283d1673a6625",
		},
		{
			name: "vsphere",
			path: vsphereMCMChecksumPath,
			instanceClass: map[string]interface{}{
				"flavorName":         "m1.large",
				"imageName":          "ubuntu-18-04-cloud-amd64",
				"numCPUs":            float64(3),
				"memory":             float64(3),
				"rootDiskSize":       float64(42),
				"template":           "dev/test",
				"mainNetwork":        "mymainnetwork",
				"additionalNetworks": []interface{}{"aaa", "bbb"},
				"datastore":          "lun-111",
				"runtimeOptions": map[string]interface{}{
					"nestedHardwareVirtualization": true,
					"memoryReservation":            float64(42),
				},
			},
			want: "e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := os.ReadFile(tc.path)
			require.NoError(t, err)

			got, err := RenderChecksum(tmpl, buildChecksumElement(tc.instanceClass, tc.manualRolloutID), tc.cloudProvider)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got,
				"checksum drifted from the helm implementation — this renames the machine template and rolls every node")
		})
	}
}

const (
	yandexMCMChecksumPath    = "../../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.checksum"
	gcpMCMChecksumPath       = "../../../../../../../../030-cloud-provider-gcp/cloud-instance-manager/machine-class.checksum"
	azureMCMChecksumPath     = "../../../../../../../../030-cloud-provider-azure/cloud-instance-manager/machine-class.checksum"
	vsphereMCMChecksumPath   = "../../../../../../../../../ee/se-plus/modules/030-cloud-provider-vsphere/cloud-instance-manager/machine-class.checksum"
	openstackMCMChecksumPath = "../../../../../../../../../ee/modules/030-cloud-provider-openstack/cloud-instance-manager/machine-class.checksum"
)

func TestRenderChecksum_MCMProviderParity(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		blob        map[string]interface{}
		wantOptions map[string]interface{}
	}{
		{
			name: "yandex: default diskSizeGB=50 excluded, coreFraction kept",
			path: yandexMCMChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"platformID":   "standard-v3",
					"cores":        float64(4),
					"coreFraction": float64(100),
					"memory":       float64(8589934592),
					"diskType":     "network-ssd",
					"diskSizeGB":   float64(50),
					"imageID":      "img-abc",
				},
				"manualRolloutID": "r1",
			},
			wantOptions: map[string]interface{}{
				"platformID":      "standard-v3",
				"cores":           float64(4),
				"coreFraction":    float64(100),
				"memory":          float64(8589934592),
				"diskType":        "network-ssd",
				"imageID":         "img-abc",
				"manualRolloutID": "r1",
			},
		},
		{
			name: "gcp: default diskSizeGb=50 excluded, diskType kept",
			path: gcpMCMChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"machineType": "n1-standard-4",
					"image":       "img-1",
					"diskSizeGb":  float64(50),
					"diskType":    "pd-ssd",
					"preemptible": true,
				},
				"manualRolloutID": "r2",
			},
			wantOptions: map[string]interface{}{
				"machineType":     "n1-standard-4",
				"image":           "img-1",
				"diskType":        "pd-ssd",
				"preemptible":     true,
				"manualRolloutID": "r2",
			},
		},
		{
			name: "azure: diskSizeGb key sourced from .diskSize, acceleratedNetworking=false kept",
			path: azureMCMChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"machineSize":           "Standard_D4",
					"urn":                   "urn-1",
					"diskSizeGb":            float64(100),
					"diskSize":              float64(99),
					"diskType":              "Premium_LRS",
					"acceleratedNetworking": false,
				},
				"manualRolloutID": "r3",
			},
			wantOptions: map[string]interface{}{
				"machineSize":           "Standard_D4",
				"urn":                   "urn-1",
				"diskSizeGb":            float64(99),
				"diskType":              "Premium_LRS",
				"acceleratedNetworking": false,
				"manualRolloutID":       "r3",
			},
		},
		{
			name: "vsphere: memory arithmetic, default rootDiskSize=20 becomes nil",
			path: vsphereMCMChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"numCPUs":      float64(4),
					"memory":       float64(8192),
					"rootDiskSize": float64(20),
					"template":     "tmpl-1",
					"datastore":    "ds-1",
					"mainNetwork":  "net-1",
				},
				"manualRolloutID": "r4",
			},
			wantOptions: map[string]interface{}{
				"numCPUs":         float64(4),
				"memory":          float64(8192),
				"rootDiskSize":    nil,
				"template":        "tmpl-1",
				"datastore":       "ds-1",
				"mainNetwork":     "net-1",
				"manualRolloutID": "r4",
			},
		},
		{
			name: "openstack: truthy-gated optionals set",
			path: openstackMCMChecksumPath,
			blob: map[string]interface{}{
				"instanceClass": map[string]interface{}{
					"flavorName":   "m1.large",
					"imageName":    "img-os",
					"mainNetwork":  "net-os",
					"rootDiskSize": float64(30),
				},
				"manualRolloutID": "r5",
			},
			wantOptions: map[string]interface{}{
				"flavorName":      "m1.large",
				"imageName":       "img-os",
				"mainNetwork":     "net-os",
				"rootDiskSize":    float64(30),
				"manualRolloutID": "r5",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := os.ReadFile(tc.path)
			require.NoError(t, err, "provider MCM checksum template must exist")

			got, err := RenderChecksum(tmpl, tc.blob, nil)
			require.NoError(t, err)

			want := expectedChecksum(t, tc.wantOptions)
			assert.Len(t, got, 64, "sha256sum output is 64 hex chars")
			assert.Equal(t, want, got)
		})
	}
}

func TestRenderChecksum_ManualRolloutIDChangesChecksum(t *testing.T) {
	tmpl, err := os.ReadFile(awsChecksumTemplatePath)
	require.NoError(t, err)

	base := map[string]interface{}{
		"instanceClass":   map[string]interface{}{"instanceType": "m5.large"},
		"manualRolloutID": "",
	}
	bumped := map[string]interface{}{
		"instanceClass":   map[string]interface{}{"instanceType": "m5.large"},
		"manualRolloutID": "roll-2",
	}

	a, err := RenderChecksum(tmpl, base, nil)
	require.NoError(t, err)
	b, err := RenderChecksum(tmpl, bumped, nil)
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "a non-empty manualRolloutID must change the checksum")
}
