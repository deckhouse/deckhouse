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

// awsChecksumTemplatePath points at the real provider template rendered by the
// machineclass_checksum hooks; the parity test renders the exact same file.
const awsChecksumTemplatePath = "../../../../../../../../030-cloud-provider-aws/cloud-instance-manager/machine-class.checksum"

// yandexCAPIChecksumTemplatePath is the real CAPI instance-class.checksum for the
// yandex provider; it uses the same tail pipeline as the MCM template but is
// rendered for the CAPI MachineTemplate the node-controller reconciler creates.
const yandexCAPIChecksumTemplatePath = "../../../../../../../../030-cloud-provider-yandex/capi/instance-class.checksum"

// expectedChecksum reproduces the tail pipeline of every provider template
// (`$options | toYaml | trimSuffix "\n" | printf "%s\n" | sha256sum`) so the test
// asserts RenderChecksum against an independently computed digest, not a frozen
// constant. toYaml trims the trailing newline and printf re-adds exactly one, so
// the sha256 input is TrimSuffix(yaml,"\n")+"\n".
func expectedChecksum(t *testing.T, options map[string]interface{}) string {
	t.Helper()
	raw, err := yaml.Marshal(options)
	require.NoError(t, err)
	input := strings.TrimSuffix(string(raw), "\n") + "\n"
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// TestRenderChecksum_AWSParity renders the real AWS machine-class.checksum against
// a blob element and checks the digest matches the independently computed one —
// proving the FuncMap (toYaml/sha256sum) and data shape reproduce the hook byte
// for byte.
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

	// The AWS template builds $options from the whitelisted instanceClass fields
	// (diskSizeGb kept because 50 != 20) plus manualRolloutID.
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

// TestRenderChecksum_CAPIYandexParity renders the real yandex CAPI
// instance-class.checksum — the template the active node-controller reconciler
// needs — and checks the digest against the independently computed one, proving
// the same engine covers the CAPI checksum family byte for byte.
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

	// The yandex CAPI template always sets platformID/cores/memory/diskType/imageID
	// and adds manualRolloutID; the optional fields are absent here.
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

// The default diskSizeGb (20) is intentionally excluded from $options by the
// template; a NodeGroup that only differs by that default must not change its
// checksum, so it must render identically to one with no diskSizeGb at all.
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

// BuildChecksumElement must produce a checksum byte-identical to a "full" blob
// element carrying extra fields (nodeType/cri/zones/...): the templates read only
// instanceClass + manualRolloutID, so passing the minimal element at wiring time is
// safe. Proven for both the MCM (aws) and CAPI (yandex) template families.
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

			// A "full" blob element carries the whole get_crds blob shape: the same
			// instanceClass + manualRolloutID plus noise fields the templates ignore.
			full := map[string]interface{}{
				"instanceClass":   tc.ic,
				"manualRolloutID": "rollout-9",
				"name":            "worker",
				"nodeType":        "CloudEphemeral",
				"cri":             map[string]interface{}{"type": "Containerd"},
				"zones":           []interface{}{"a", "b"},
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

// manualRolloutID feeds the checksum (that is the whole point of M19): changing it
// must roll the checksum, so nodes re-bootstrap on a manual rollout bump.
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
