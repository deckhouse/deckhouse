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
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// Inputs and expected output captured from a running cluster (a DVP/CAPI cluster on Deckhouse
// 1.75): every NodeGroup spec as the API server stores it, and the element the deployed
// node-controller published for it into the bashible context Secret.
//
//	kubectl get ng -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec}{"\n"}{end}'
//	kubectl -n d8-cloud-instance-manager get secret bashible-apiserver-context \
//	  -o jsonpath='{.data.input\.yaml}' | base64 -d
//
// The corpus proves the code did not change its own output; this proves that output is the one a
// live cluster actually runs on — the same key set, the same passthrough, the same serialized
// labels and taints, for every node type the cluster has.
type liveNodeGroup struct {
	name             string
	nodeType         v1.NodeType
	specJSON         string
	engine           string
	criType          string
	serializedLabels string
	serializedTaints string
	instanceClass    string
	zones            []string
	expKeys          []string
}

func liveNodeGroups() []liveNodeGroup {
	return []liveNodeGroup{
		{
			name:     "immutable",
			nodeType: v1.NodeTypeStatic,
			specJSON: `{"disruptions":{"approvalMode":"Manual"},"kubelet":{"containerLogMaxFiles":4,"containerLogMaxSize":"50Mi","resourceReservation":{"mode":"Auto"}},"nodeTemplate":{},"nodeType":"Static"}`,
			engine:   "None", criType: "Containerd",
			serializedLabels: "node-role.kubernetes.io/immutable=,node.deckhouse.io/group=immutable,node.deckhouse.io/type=Static",
			expKeys: []string{"cri", "disruptions", "engine", "kubelet", "kubernetesVersion",
				"manualRolloutID", "name", "nodeType", "serializedLabels", "serializedTaints", "updateEpoch"},
		},
		{
			name:     "immutable-test",
			nodeType: v1.NodeTypeCloudEphemeral,
			specJSON: `{"cloudInstances":{"classReference":{"kind":"DVPInstanceClass","name":"immutable-test"},"maxPerZone":2,"minPerZone":2},"disruptions":{"approvalMode":"Automatic"},"kubelet":{"containerLogMaxFiles":4,"containerLogMaxSize":"50Mi","resourceReservation":{"mode":"Auto"}},"nodeTemplate":{"labels":{"node-role.deckhouse.io/immutable":""}},"nodeType":"CloudEphemeral"}`,
			engine:   "CAPI", criType: "Containerd",
			serializedLabels: "node-role.deckhouse.io/immutable=,node-role.kubernetes.io/immutable-test=,node.deckhouse.io/group=immutable-test,node.deckhouse.io/type=CloudEphemeral",
			instanceClass:    liveWorkerInstanceClassJSON,
			zones:            []string{"default"},
			expKeys: []string{"cloudInstances", "cri", "disruptions", "engine", "instanceClass", "kubelet",
				"kubernetesVersion", "manualRolloutID", "name", "nodeTemplate", "nodeType",
				"serializedLabels", "serializedTaints", "updateEpoch"},
		},
		{
			name:     "master",
			nodeType: v1.NodeTypeCloudPermanent,
			specJSON: `{"disruptions":{"approvalMode":"Manual"},"kubelet":{"containerLogMaxFiles":4,"containerLogMaxSize":"50Mi","resourceReservation":{"mode":"Auto"}},"nodeTemplate":{"labels":{"node-role.kubernetes.io/control-plane":"","node-role.kubernetes.io/master":""},"taints":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane"}]},"nodeType":"CloudPermanent"}`,
			engine:   "None", criType: "Containerd",
			serializedLabels: "node-role.kubernetes.io/control-plane=,node-role.kubernetes.io/master=,node.deckhouse.io/group=master,node.deckhouse.io/type=CloudPermanent",
			serializedTaints: "node-role.kubernetes.io/control-plane:NoSchedule",
			expKeys: []string{"cri", "disruptions", "engine", "kubelet", "kubernetesVersion",
				"manualRolloutID", "name", "nodeTemplate", "nodeType", "serializedLabels",
				"serializedTaints", "updateEpoch"},
		},
		{
			name:     "worker",
			nodeType: v1.NodeTypeCloudEphemeral,
			specJSON: liveWorkerSpecJSON,
			engine:   "CAPI", criType: "Containerd",
			serializedLabels: "node-role.kubernetes.io/worker=,node.deckhouse.io/group=worker,node.deckhouse.io/type=CloudEphemeral",
			instanceClass:    liveWorkerInstanceClassJSON,
			zones:            []string{"default"},
			expKeys: []string{"cloudInstances", "cri", "engine", "instanceClass", "kubelet",
				"kubernetesVersion", "manualRolloutID", "name", "nodeType", "serializedLabels",
				"serializedTaints", "updateEpoch"},
		},
		{
			name:     "worker-s",
			nodeType: v1.NodeTypeStatic,
			specJSON: `{"kubelet":{"containerLogMaxFiles":4,"containerLogMaxSize":"50Mi","resourceReservation":{"mode":"Auto"}},"nodeType":"Static","staticInstances":{"count":0,"labelSelector":{"matchLabels":{"role":"worker-s"}}}}`,
			engine:   "CAPI", criType: "Containerd",
			serializedLabels: "node-role.kubernetes.io/worker-s=,node.deckhouse.io/group=worker-s,node.deckhouse.io/type=Static",
			expKeys: []string{"cri", "engine", "kubelet", "kubernetesVersion", "manualRolloutID",
				"name", "nodeType", "serializedLabels", "serializedTaints", "staticInstances", "updateEpoch"},
		},
	}
}

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

func TestResolvedNodeGroup_MatchesLiveClusterElements(t *testing.T) {
	for _, live := range liveNodeGroups() {
		t.Run(live.name, func(t *testing.T) {
			var rawSpec map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(live.specJSON), &rawSpec))

			ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: live.name}}
			require.NoError(t, json.Unmarshal([]byte(live.specJSON), &ng.Spec))

			in := ResolveInput{
				Name:           live.name,
				NodeType:       live.nodeType,
				Spec:           specFrom(rawSpec),
				CloudProcessed: live.instanceClass != "",
			}
			result := Result{
				Engine:            live.engine,
				KubernetesVersion: "1.35",
				CRIType:           live.criType,
				Zones:             live.zones,
				SerializedLabels:  serializeLabels(ng),
				SerializedTaints:  serializeTaints(ng),
				UpdateEpoch:       "1786000000",
			}
			if live.instanceClass != "" {
				result.InstanceClass = rawExtension(live.instanceClass)
			}

			got := ResolveNodeGroup(in, result).ToMap()

			gotKeys := make([]string, 0, len(got))
			for key := range got {
				gotKeys = append(gotKeys, key)
			}
			sort.Strings(gotKeys)
			require.Equal(t, live.expKeys, gotKeys, "published key set must match the live element")

			require.Equal(t, live.serializedLabels, got["serializedLabels"])
			require.Equal(t, live.serializedTaints, got["serializedTaints"])
			require.Equal(t, map[string]interface{}{"type": live.criType}, got["cri"])
			require.Equal(t, live.engine, got["engine"])
			require.Equal(t, "", got["manualRolloutID"])

			// Passthrough subtrees reach the node verbatim. Compared as YAML, which is the form
			// the node actually receives and the form bashible-apiserver hashes — the Go type
			// behind a number is invisible there, and asserting on it would pin the fixture's
			// own decoding rather than the published bytes.
			for _, key := range []string{"kubelet", "disruptions", "nodeTemplate", "staticInstances"} {
				expected, present := rawSpec[key]
				if !present || isEmptySpecValue(expected) {
					require.NotContains(t, got, key)
					continue
				}
				expectedYAML, err := sigsyaml.Marshal(expected)
				require.NoError(t, err)
				gotYAML, err := sigsyaml.Marshal(got[key])
				require.NoError(t, err)
				require.Equal(t, string(expectedYAML), string(gotYAML),
					"passthrough %q must reach the node verbatim", key)
			}
		})
	}
}

// liveInstanceClassChecksum is the value the running cluster carries on the MachineDeployment it
// renders for the worker NodeGroup:
//
//	kubectl -n d8-cloud-instance-manager get machinedeployments.cluster.x-k8s.io \
//	  zykov-dev-u2-worker-0f7c2f04 -o jsonpath='{.metadata.annotations.checksum/instance-class}'
//
// It names an immutable machine template, so reproducing it byte for byte is the strongest
// statement this package can make: the rewrite publishes an element that hashes to what the
// deployed controller already hashed, and no machine is recreated by the change.
//
// The DVP template reads only .nodeGroup.instanceClass, so no cluster credentials are involved.
const liveInstanceClassChecksum = "3040a219bf773e7f8d8926575bbb4beb339c7c4ca000758d39a7d0d1be629172"

func TestResolvedNodeGroup_ReproducesLiveInstanceClassChecksum(t *testing.T) {
	template, err := os.ReadFile("../../../machinetemplate/testdata/v1/dvp/instance-class.checksum")
	require.NoError(t, err, "DVP instance-class checksum template")

	var rawSpec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(liveWorkerSpecJSON), &rawSpec))

	in := ResolveInput{
		Name:           "worker",
		NodeType:       v1.NodeTypeCloudEphemeral,
		Spec:           specFrom(rawSpec),
		CloudProcessed: true,
	}
	result := Result{
		Engine:        "CAPI",
		CRIType:       "Containerd",
		Zones:         []string{"default"},
		InstanceClass: rawExtension(liveWorkerInstanceClassJSON),
	}

	nodeGroupValues := ResolveNodeGroup(in, result).ToMap()

	got, err := machineclass.RenderChecksum(template, nodeGroupValues, map[string]interface{}{})
	require.NoError(t, err)
	require.Equal(t, liveInstanceClassChecksum, got,
		"the published element must hash to what the deployed controller hashed; a mismatch renames "+
			"the machine template and recreates every VM in the NodeGroup")
}
