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

func TestBashibleChecksum_UpscaleInvariance(t *testing.T) {
	small := buildCloudBlob(t, "1.32", float64(1), float64(3), []string{"a", "b"})
	large := buildCloudBlob(t, "1.32", float64(5), float64(10), []string{"a", "b", "c"})

	assert.Equal(t,
		bashibleNodeGroupChecksum(t, small),
		bashibleNodeGroupChecksum(t, large),
		"scaling counters and zones are stripped before hashing, so they must not change the checksum",
	)
}

func TestBashibleChecksum_MeaningfulFieldChanges(t *testing.T) {
	v132 := buildCloudBlob(t, "1.32", float64(1), float64(3), []string{"a", "b"})
	v131 := buildCloudBlob(t, "1.31", float64(1), float64(3), []string{"a", "b"})

	assert.NotEqual(t,
		bashibleNodeGroupChecksum(t, v132),
		bashibleNodeGroupChecksum(t, v131),
		"a non-stripped field (kubernetesVersion) must change the checksum",
	)
}

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
