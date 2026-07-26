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

const yandexCAPIChecksumTemplatePath = "../../../../../../../../030-cloud-provider-yandex/capi/instance-class.checksum"

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

	got, err := RenderChecksum(tmpl, blobElement)
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

func TestRenderChecksum_CAPIYandexParity(t *testing.T) {
	tmpl, err := os.ReadFile(yandexCAPIChecksumTemplatePath)
	require.NoError(t, err, "provider CAPI checksum template must exist")

	blobElement := map[string]interface{}{
		"instanceClass": map[string]interface{}{
			"platformID": "standard-v3",
			"cores":      float64(4),
			"memory":     float64(8589934592),
			"diskType":   "network-ssd",
			"imageID":    "img-abc",
		},
		"manualRolloutID": "rollout-7",
	}

	got, err := RenderChecksum(tmpl, blobElement)
	require.NoError(t, err)

	want := expectedChecksum(t, map[string]interface{}{
		"platformID":      "standard-v3",
		"cores":           float64(4),
		"memory":          float64(8589934592),
		"diskType":        "network-ssd",
		"imageID":         "img-abc",
		"manualRolloutID": "rollout-7",
	})

	assert.Len(t, got, 64)
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

	a, err := RenderChecksum(tmpl, withDefault)
	require.NoError(t, err)
	b, err := RenderChecksum(tmpl, withoutDisk)
	require.NoError(t, err)

	assert.Equal(t, b, a, "default diskSizeGb=20 is excluded, so checksum must not change")
}

func TestBuildChecksumElement_OnlyInstanceClassAndRolloutMatter(t *testing.T) {
	awsTmpl, err := os.ReadFile(awsChecksumTemplatePath)
	require.NoError(t, err)
	yandexTmpl, err := os.ReadFile(yandexCAPIChecksumTemplatePath)
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
			name: "yandex-capi",
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
			minimal := BuildChecksumElement(tc.ic, "rollout-9")

			full := map[string]interface{}{
				"instanceClass":     tc.ic,
				"manualRolloutID":   "rollout-9",
				"name":              "worker",
				"nodeType":          "CloudEphemeral",
				"cri":               map[string]interface{}{"type": "Containerd"},
				"zones":             []interface{}{"a", "b"},
				"kubernetesVersion": "1.29",
			}

			gotMinimal, err := RenderChecksum(tc.tmpl, minimal)
			require.NoError(t, err)
			gotFull, err := RenderChecksum(tc.tmpl, full)
			require.NoError(t, err)

			assert.Equal(t, gotFull, gotMinimal,
				"only instanceClass + manualRolloutID may affect the checksum")
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

			got, err := RenderChecksum(tmpl, tc.blob)
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

	a, err := RenderChecksum(tmpl, base)
	require.NoError(t, err)
	b, err := RenderChecksum(tmpl, bumped)
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "a non-empty manualRolloutID must change the checksum")
}
