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

package derived_status

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// bashibleNodeGroupChecksum mirrors the node-group contribution to the bashible
// bootstrap checksum computed by bashible-apiserver
// (images/bashible-apiserver/src/pkg/template/context_builder.go:431 AddToChecksum):
// it strips the scaling counters and the resolved zones from cloudInstances
// (the apiserver deletes maxPerZone/maxSurgePerZone/maxUnavailablePerZone/
// minPerZone/zones so scaling up or down never re-bootstraps existing nodes),
// then sha256s the sigs.k8s.io/yaml marshalling — the exact lib (:28) and
// procedure the apiserver uses.
//
// This isolates the nodeGroup element that the node-controller-written Secret
// must reproduce byte-for-byte: if this digest changes for the same NodeGroup,
// every node in the group re-bootstraps. When node-controller starts writing
// bashible-apiserver-context it must feed a nodeGroups element whose digest here
// equals the one the current get_crds+helm path produces (proven elsewhere by
// the golden blob tests), otherwise the cutover rolls the whole cluster.
func bashibleNodeGroupChecksum(t *testing.T, blob map[string]interface{}) string {
	t.Helper()

	// JSON round-trip deep-copy so stripping does not mutate the caller's blob.
	raw, err := json.Marshal(blob)
	require.NoError(t, err)
	var cpy map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &cpy))

	if ci, ok := cpy["cloudInstances"].(map[string]interface{}); ok {
		delete(ci, "maxPerZone")
		delete(ci, "maxSurgePerZone")
		delete(ci, "maxUnavailablePerZone")
		delete(ci, "minPerZone")
		delete(ci, "zones")
	}

	y, err := yaml.Marshal(cpy)
	require.NoError(t, err)
	sum := sha256.Sum256(y)
	return hex.EncodeToString(sum[:])
}

// buildCloudBlob assembles a CloudEphemeral blob element with the given scaling
// counters and zones, holding every checksum-relevant field constant, so tests
// can vary only the stripped-vs-meaningful fields.
func buildCloudBlob(t *testing.T, kubernetesVersion string, minPerZone, maxPerZone float64, zones []string) map[string]interface{} {
	t.Helper()
	return BuildNodeGroupBlob(BlobInput{
		Name:     "worker",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cloudInstances": map[string]interface{}{
				"classReference": map[string]interface{}{"kind": "D8TestInstanceClass", "name": "worker"},
				"minPerZone":     minPerZone,
				"maxPerZone":     maxPerZone,
			},
			"kubelet": kubeletDefaults(),
		},
		CloudProcessed: true,
	}, Result{
		Engine:            "None",
		KubernetesVersion: kubernetesVersion,
		CRIType:           "Containerd",
		Zones:             zones,
		InstanceClass:     &runtime.RawExtension{Raw: []byte("null")},
		SerializedLabels:  "node-role.kubernetes.io/worker=,node.deckhouse.io/group=worker,node.deckhouse.io/type=CloudEphemeral",
		SerializedTaints:  "",
		UpdateEpoch:       "1",
	})
}

// Upscale invariance: two blobs that differ ONLY in the stripped fields
// (min/maxPerZone and resolved zones) must yield the same bashible checksum —
// this is the apiserver's "prevent updating nodes while upscale" contract
// (context_builder.go:439-444). Our blob must be compatible with it, i.e. those
// fields must land under cloudInstances where the strip removes them.
func TestBashibleChecksum_UpscaleInvariance(t *testing.T) {
	small := buildCloudBlob(t, "1.32", float64(1), float64(3), []string{"a", "b"})
	large := buildCloudBlob(t, "1.32", float64(5), float64(10), []string{"a", "b", "c"})

	assert.Equal(t,
		bashibleNodeGroupChecksum(t, small),
		bashibleNodeGroupChecksum(t, large),
		"scaling counters and zones are stripped before hashing, so they must not change the checksum",
	)
}

// A meaningful field (kubernetesVersion) survives the strip and so MUST change
// the checksum — proving the strip is targeted, not blanket, and that a real
// config change still re-bootstraps nodes as intended.
func TestBashibleChecksum_MeaningfulFieldChanges(t *testing.T) {
	v132 := buildCloudBlob(t, "1.32", float64(1), float64(3), []string{"a", "b"})
	v131 := buildCloudBlob(t, "1.31", float64(1), float64(3), []string{"a", "b"})

	assert.NotEqual(t,
		bashibleNodeGroupChecksum(t, v132),
		bashibleNodeGroupChecksum(t, v131),
		"a non-stripped field (kubernetesVersion) must change the checksum",
	)
}

// End-to-end parity: a blob built by BuildNodeGroupBlob must produce the same
// bashible checksum as the golden get_crds element decoded from its stored YAML.
// The golden JSON is the byte-parity ground truth (get_crds_test.go); feeding
// both through the exact apiserver procedure proves the cutover keeps the
// bootstrap checksum stable, not merely the JSON representation.
func TestBashibleChecksum_GoldenParity(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "proper1",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cloudInstances": map[string]interface{}{
				"classReference": map[string]interface{}{"kind": "D8TestInstanceClass", "name": "proper1"},
			},
			"kubelet": kubeletDefaults(),
		},
		CloudProcessed: true,
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.32",
		CRIType:           "Containerd",
		Zones:             []string{"a", "b", "c"},
		InstanceClass:     &runtime.RawExtension{Raw: []byte("null")},
		SerializedLabels:  "node-role.kubernetes.io/proper1=,node.deckhouse.io/group=proper1,node.deckhouse.io/type=CloudEphemeral",
		SerializedTaints:  "",
		UpdateEpoch:       "222",
	})

	const goldenJSON = `{
		"nodeType": "CloudEphemeral",
		"cloudInstances": {
			"classReference": { "kind": "D8TestInstanceClass", "name": "proper1" },
			"zones": ["a", "b", "c"]
		},
		"instanceClass": null,
		"kubelet": {
			"containerLogMaxSize": "50Mi",
			"containerLogMaxFiles": 4,
			"resourceReservation": { "mode": "Auto" },
			"topologyManager": {}
		},
		"serializedLabels": "node-role.kubernetes.io/proper1=,node.deckhouse.io/group=proper1,node.deckhouse.io/type=CloudEphemeral",
		"serializedTaints": "",
		"manualRolloutID": "",
		"kubernetesVersion": "1.32",
		"cri": { "type": "Containerd" },
		"engine": "None",
		"name": "proper1",
		"updateEpoch": "222"
	}`
	var golden map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(goldenJSON), &golden))

	assert.Equal(t,
		bashibleNodeGroupChecksum(t, golden),
		bashibleNodeGroupChecksum(t, blob),
		"BuildNodeGroupBlob must reproduce the get_crds element's bashible checksum",
	)
}
