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
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// Inputs and expected output captured from a running cluster (a DVP/CAPI dev cluster on
// Deckhouse 1.75): the NodeGroup spec as the API server stores it, and the element the deployed
// node-controller published for it into the bashible context Secret.
//
// This pins the rewrite against a real published element rather than against a fixture: the corpus
// proves the code did not change its own output, this proves that output is the one a live cluster
// actually runs on.
const (
	liveWorkerSpecJSON = `{
		"cloudInstances": {
			"classReference": {"kind": "DVPInstanceClass", "name": "worker"},
			"maxPerZone": 2,
			"minPerZone": 2
		},
		"kubelet": {
			"containerLogMaxFiles": 4,
			"containerLogMaxSize": "50Mi",
			"resourceReservation": {"mode": "Auto"}
		},
		"nodeType": "CloudEphemeral"
	}`

	liveWorkerInstanceClassJSON = `{
		"rootDisk": {
			"image": {"kind": "ClusterVirtualImage", "name": "ubuntu-24-04-lts"},
			"size": "50Gi",
			"storageClass": "replicated"
		},
		"virtualMachine": {
			"bootloader": "EFI",
			"cpu": {"coreFraction": "5%", "cores": 4},
			"liveMigrationPolicy": "PreferForced",
			"memory": {"size": "8Gi"},
			"runPolicy": "AlwaysOnUnlessStoppedManually",
			"virtualMachineClassName": "amd-epyc-gen-3"
		}
	}`
)

// liveWorkerKeys is the exact key set the deployed controller published for this NodeGroup.
var liveWorkerKeys = []string{
	"cloudInstances", "cri", "engine", "instanceClass", "kubelet", "kubernetesVersion",
	"manualRolloutID", "name", "nodeType", "serializedLabels", "serializedTaints", "updateEpoch",
}

func TestResolvedNodeGroup_MatchesLiveClusterElement(t *testing.T) {
	var rawSpec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(liveWorkerSpecJSON), &rawSpec))

	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	require.NoError(t, json.Unmarshal([]byte(liveWorkerSpecJSON), &ng.Spec))

	in := ResolveInput{
		Name:           "worker",
		NodeType:       v1.NodeTypeCloudEphemeral,
		RawSpec:        rawSpec,
		CloudProcessed: true,
	}
	result := Result{
		Engine:            "CAPI",
		KubernetesVersion: "1.35",
		CRIType:           "Containerd",
		Zones:             []string{"default"},
		InstanceClass:     rawExtension(liveWorkerInstanceClassJSON),
		SerializedLabels:  serializeLabels(ng),
		UpdateEpoch:       "1786000000",
	}

	got := ResolveNodeGroup(in, result).ToMap()

	gotKeys := make([]string, 0, len(got))
	for key := range got {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	require.Equal(t, liveWorkerKeys, gotKeys, "published key set must match the live cluster element")

	// The subtrees the node actually reads, byte for byte against the live element.
	require.Equal(t, map[string]interface{}{
		"classReference": map[string]interface{}{"kind": "DVPInstanceClass", "name": "worker"},
		"maxPerZone":     float64(2),
		"minPerZone":     float64(2),
		"zones":          []string{"default"},
	}, got["cloudInstances"])
	require.Equal(t, map[string]interface{}{"type": "Containerd"}, got["cri"])
	require.Equal(t, rawExtension(liveWorkerInstanceClassJSON), got["instanceClass"])
	require.Equal(t, map[string]interface{}{
		"containerLogMaxFiles": float64(4),
		"containerLogMaxSize":  "50Mi",
		"resourceReservation":  map[string]interface{}{"mode": "Auto"},
	}, got["kubelet"])
	require.Equal(t,
		"node-role.kubernetes.io/worker=,node.deckhouse.io/group=worker,node.deckhouse.io/type=CloudEphemeral",
		got["serializedLabels"])
	require.Equal(t, "", got["manualRolloutID"])
	require.Equal(t, "", got["serializedTaints"])
}
