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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The golden blobs below are lifted verbatim from the get_crds hook test suite
// (modules/040-node-manager/hooks/get_crds_test.go, the "NGs must be stored to
// nodeManager.internal.nodeGroups" expectation). They are the byte-parity ground
// truth: BuildNodeGroupBlob must reproduce them exactly given the corresponding
// raw spec passthrough and computed Result.
//
// updateEpoch is elided from the fixtures (it is a pure Result passthrough,
// covered by the derived_status compute tests).

// kubeletDefaults is the CRD-defaulted kubelet block the apiserver stores on the
// live object; it flows through the blob as verbatim spec passthrough.
func kubeletDefaults() map[string]interface{} {
	return map[string]interface{}{
		"containerLogMaxSize":  "50Mi",
		"containerLogMaxFiles": float64(4),
		"resourceReservation":  map[string]interface{}{"mode": "Auto"},
		"topologyManager":      map[string]interface{}{},
	}
}

func assertBlobMatchesGolden(t *testing.T, blob map[string]interface{}, goldenJSON string) {
	t.Helper()

	blobJSON, err := json.Marshal(blob)
	require.NoError(t, err)

	var got, want interface{}
	require.NoError(t, json.Unmarshal(blobJSON, &got))
	require.NoError(t, json.Unmarshal([]byte(goldenJSON), &want))

	assert.Equal(t, want, got)
}

func TestBuildNodeGroupBlob_Golden_CloudPermanent(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "cp1",
		NodeType: v1.NodeTypeCloudPermanent,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudPermanent",
			"kubelet":  kubeletDefaults(),
		},
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.32",
		CRIType:           "Containerd",
		SerializedLabels:  "node-role.kubernetes.io/cp1=,node.deckhouse.io/group=cp1,node.deckhouse.io/type=CloudPermanent",
		SerializedTaints:  "",
		UpdateEpoch:       "111",
	})

	assertBlobMatchesGolden(t, blob, `{
		"kubernetesVersion": "1.32",
		"cri": { "type": "Containerd" },
		"engine": "None",
		"kubelet": {
			"containerLogMaxSize": "50Mi",
			"containerLogMaxFiles": 4,
			"resourceReservation": { "mode": "Auto" },
			"topologyManager": {}
		},
		"serializedLabels": "node-role.kubernetes.io/cp1=,node.deckhouse.io/group=cp1,node.deckhouse.io/type=CloudPermanent",
		"serializedTaints": "",
		"manualRolloutID": "",
		"name": "cp1",
		"nodeType": "CloudPermanent",
		"updateEpoch": "111"
	}`)
}

func TestBuildNodeGroupBlob_Golden_CloudEphemeralProcessed(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "proper1",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cloudInstances": map[string]interface{}{
				"classReference": map[string]interface{}{
					"kind": "D8TestInstanceClass",
					"name": "proper1",
				},
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

	assertBlobMatchesGolden(t, blob, `{
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
	}`)
}

// get_crds emits an empty resolved zones list as [] (get_crds_test.go:380), not
// null. resolveZones/readDefaultZones return non-nil empty slices to preserve
// this; a nil slice would marshal as null and break the bashible checksum.
func TestBuildNodeGroupBlob_Golden_EmptyZones(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "proper1",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cloudInstances": map[string]interface{}{
				"classReference": map[string]interface{}{
					"kind": "D8TestInstanceClass",
					"name": "proper1",
				},
			},
		},
		CloudProcessed: true,
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.32",
		CRIType:           "Containerd",
		Zones:             []string{},
		InstanceClass:     &runtime.RawExtension{Raw: []byte("null")},
		SerializedLabels:  "node-role.kubernetes.io/proper1=,node.deckhouse.io/group=proper1,node.deckhouse.io/type=CloudEphemeral",
		UpdateEpoch:       "222",
	})

	ci, ok := blob["cloudInstances"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{}, ci["zones"], "empty zones must stay an empty slice, not nil")

	blobJSON, err := json.Marshal(blob)
	require.NoError(t, err)
	assert.Contains(t, string(blobJSON), `"zones":[]`, "empty zones must marshal as [] not null")
}

func TestBuildNodeGroupBlob_Golden_Static(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "static1",
		NodeType: v1.NodeTypeStatic,
		RawSpec: map[string]interface{}{
			"nodeType": "Static",
			"kubelet":  kubeletDefaults(),
		},
		Static: map[string]interface{}{
			"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"},
		},
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.32",
		CRIType:           "Containerd",
		SerializedLabels:  "node-role.kubernetes.io/static1=,node.deckhouse.io/group=static1,node.deckhouse.io/type=Static",
		SerializedTaints:  "",
		UpdateEpoch:       "333",
	})

	assertBlobMatchesGolden(t, blob, `{
		"kubernetesVersion": "1.32",
		"cri": { "type": "Containerd" },
		"engine": "None",
		"kubelet": {
			"containerLogMaxSize": "50Mi",
			"containerLogMaxFiles": 4,
			"resourceReservation": { "mode": "Auto" },
			"topologyManager": {}
		},
		"serializedLabels": "node-role.kubernetes.io/static1=,node.deckhouse.io/group=static1,node.deckhouse.io/type=Static",
		"serializedTaints": "",
		"manualRolloutID": "",
		"name": "static1",
		"nodeType": "Static",
		"updateEpoch": "333",
		"static": { "internalNetworkCIDRs": ["172.18.200.0/24"] }
	}`)
}
