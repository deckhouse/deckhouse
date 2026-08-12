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

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func kubeletDefaults() map[string]interface{} {
	return map[string]interface{}{
		"containerLogMaxSize":  "50Mi",
		"containerLogMaxFiles": float64(4),
		"resourceReservation":  map[string]interface{}{"mode": "Auto"},
		"topologyManager":      map[string]interface{}{},
	}
}

func assertResolvedMatchesGolden(t *testing.T, nodeGroupValues map[string]interface{}, goldenJSON string) {
	t.Helper()

	rawJSON, err := json.Marshal(nodeGroupValues)
	require.NoError(t, err)

	var got, want interface{}
	require.NoError(t, json.Unmarshal(rawJSON, &got))
	require.NoError(t, json.Unmarshal([]byte(goldenJSON), &want))

	assert.Equal(t, want, got)
}

func TestResolvedNodeGroup_Golden_CloudPermanent(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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

	assertResolvedMatchesGolden(t, nodeGroupValues, `{
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

func TestResolvedNodeGroup_Golden_CloudEphemeralProcessed(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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
		InstanceClass:     nil,
		SerializedLabels:  "node-role.kubernetes.io/proper1=,node.deckhouse.io/group=proper1,node.deckhouse.io/type=CloudEphemeral",
		SerializedTaints:  "",
		UpdateEpoch:       "222",
	})

	assertResolvedMatchesGolden(t, nodeGroupValues, `{
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

func TestResolvedNodeGroup_Golden_EmptyZones(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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
		InstanceClass:     nil,
		SerializedLabels:  "node-role.kubernetes.io/proper1=,node.deckhouse.io/group=proper1,node.deckhouse.io/type=CloudEphemeral",
		UpdateEpoch:       "222",
	})

	ci, ok := nodeGroupValues["cloudInstances"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{}, ci["zones"], "empty zones must stay an empty slice, not nil")

	rawJSON, err := json.Marshal(nodeGroupValues)
	require.NoError(t, err)
	assert.Contains(t, string(rawJSON), `"zones":[]`, "empty zones must marshal as [] not null")
}

func TestResolvedNodeGroup_Golden_Static(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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

	assertResolvedMatchesGolden(t, nodeGroupValues, `{
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
