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
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the blob goldens under testdata/blob")

// blobFixture is one NodeGroup shape the published element is pinned for. The corpus covers
// every branch of BuildNodeGroupBlob, so any change in which keys reach the bashible context —
// and a key present with an empty value is different data from an absent key — shows up as a
// golden diff.
type blobFixture struct {
	name   string
	input  BlobInput
	result Result
}

func rawExtension(json string) *runtime.RawExtension {
	return &runtime.RawExtension{Raw: []byte(json)}
}

// buildNodeGroupBlob is the published element of a NodeGroup, which is what most of these tests
// assert on. Production code holds the ResolvedNodeGroup and serializes it at the two boundaries
// that need a map, so this shortcut lives here.
func buildNodeGroupBlob(in BlobInput, r Result) map[string]interface{} {
	return ResolveNodeGroup(in, r).ToMap()
}

// Integral numbers arrive as int64 and fractional ones as float64: the raw spec comes from the
// unstructured NodeGroup, not from encoding/json.
func blobCorpus() []blobFixture {
	return []blobFixture{
		{
			name: "static-minimal",
			input: BlobInput{
				Name:     "static-minimal",
				NodeType: v1.NodeTypeStatic,
				RawSpec:  map[string]interface{}{"nodeType": "Static"},
			},
			result: Result{
				Engine:            engineNone,
				KubernetesVersion: "1.32",
				CRIType:           criTypeContainerd,
				SerializedLabels:  "node-role.kubernetes.io/static-minimal=,node.deckhouse.io/group=static-minimal,node.deckhouse.io/type=Static",
				UpdateEpoch:       "1000",
			},
		},
		{
			name: "static-with-cluster-config",
			input: BlobInput{
				Name:     "static-full",
				NodeType: v1.NodeTypeStatic,
				RawSpec: map[string]interface{}{
					"nodeType": "Static",
					"staticInstances": map[string]interface{}{
						"count":         int64(3),
						"labelSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"node-group": "static-full"}},
					},
					"fencing": map[string]interface{}{"mode": "Watchdog"},
					"kubelet": kubeletDefaults(),
					"nodeTemplate": map[string]interface{}{
						"labels":      map[string]interface{}{"role": "worker"},
						"annotations": map[string]interface{}{"ann": "value"},
						"taints": []interface{}{
							map[string]interface{}{"key": "dedicated", "value": "worker", "effect": "NoExecute"},
						},
					},
					"disruptions": map[string]interface{}{"approvalMode": "Manual"},
				},
				Static: map[string]interface{}{
					"internalNetworkCIDRs": []interface{}{"172.18.200.0/24", "10.0.0.0/8"},
				},
			},
			result: Result{
				Engine:            engineCAPI,
				KubernetesVersion: "1.32",
				CRIType:           criTypeContainerd,
				SerializedLabels:  "node-role.kubernetes.io/static-full=,node.deckhouse.io/group=static-full,node.deckhouse.io/type=Static,role=worker",
				SerializedTaints:  "dedicated=worker:NoExecute",
				UpdateEpoch:       "1001",
			},
		},
		{
			name: "static-without-cluster-config",
			input: BlobInput{
				Name:     "static-no-config",
				NodeType: v1.NodeTypeStatic,
				RawSpec:  map[string]interface{}{"nodeType": "Static"},
				Static:   nil,
			},
			result: Result{
				Engine:           engineNone,
				CRIType:          criTypeContainerd,
				SerializedLabels: "node.deckhouse.io/group=static-no-config",
				UpdateEpoch:      "1002",
			},
		},
		{
			name: "cloud-permanent",
			input: BlobInput{
				Name:     "master",
				NodeType: v1.NodeTypeCloudPermanent,
				RawSpec: map[string]interface{}{
					"nodeType":    "CloudPermanent",
					"kubelet":     kubeletDefaults(),
					"disruptions": map[string]interface{}{"approvalMode": "Manual"},
					"nodeTemplate": map[string]interface{}{
						"taints": []interface{}{
							map[string]interface{}{"key": "node-role.kubernetes.io/control-plane", "effect": "NoSchedule"},
						},
					},
				},
				// A CloudPermanent NodeGroup never publishes the static cluster configuration,
				// even when the controller happens to have read it.
				Static: map[string]interface{}{"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"}},
			},
			result: Result{
				Engine:            engineNone,
				KubernetesVersion: "1.32",
				CRIType:           criTypeContainerd,
				SerializedLabels:  "node-role.kubernetes.io/master=,node.deckhouse.io/group=master,node.deckhouse.io/type=CloudPermanent",
				SerializedTaints:  "node-role.kubernetes.io/control-plane:NoSchedule",
				UpdateEpoch:       "1003",
			},
		},
		{
			name: "cloud-ephemeral-processed-full",
			input: BlobInput{
				Name:            "worker",
				ManualRolloutID: "rollout-1",
				NodeType:        v1.NodeTypeCloudEphemeral,
				RawSpec: map[string]interface{}{
					"nodeType": "CloudEphemeral",
					"cloudInstances": map[string]interface{}{
						"classReference":        map[string]interface{}{"kind": "YandexInstanceClass", "name": "worker"},
						"minPerZone":            int64(1),
						"maxPerZone":            int64(5),
						"maxSurgePerZone":       int64(2),
						"maxUnavailablePerZone": int64(0),
						"quickShutdown":         true,
						"zones":                 []interface{}{"ru-central1-a"},
					},
					"nodeTemplate": map[string]interface{}{
						"labels": map[string]interface{}{"role": "worker"},
					},
					"kubelet":                kubeletDefaults(),
					"chaos":                  map[string]interface{}{"mode": "DrainAndDelete", "period": "6h"},
					"operatingSystem":        map[string]interface{}{"manageKernel": false},
					"disruptions":            map[string]interface{}{"approvalMode": "Automatic"},
					"gpu":                    map[string]interface{}{"sharing": "TimeSlicing"},
					"nodeDrainTimeoutSecond": int64(300),
				},
				CloudProcessed: true,
			},
			result: Result{
				Engine:            engineMCM,
				KubernetesVersion: "1.32",
				CRIType:           criTypeContainerd,
				Zones:             []string{"ru-central1-a", "ru-central1-b"},
				NodeCapacity:      rawExtension(`{"cpu":"4","memory":"8Gi"}`),
				InstanceClass:     rawExtension(`{"platformID":"standard-v3","cores":4,"memory":8589934592,"coreFraction":100,"diskType":"network-ssd"}`),
				SerializedLabels:  "node-role.kubernetes.io/worker=,node.deckhouse.io/group=worker,node.deckhouse.io/type=CloudEphemeral,role=worker",
				UpdateEpoch:       "1004",
			},
		},
		{
			name: "cloud-ephemeral-processed-nil-instance-class",
			input: BlobInput{
				Name:     "worker-nil-class",
				NodeType: v1.NodeTypeCloudEphemeral,
				// No cloudInstances in the spec at all: the cloud overlay still creates the block.
				RawSpec:        map[string]interface{}{"nodeType": "CloudEphemeral"},
				CloudProcessed: true,
			},
			result: Result{
				Engine:            engineCAPI,
				KubernetesVersion: "1.32",
				CRIType:           criTypeContainerd,
				Zones:             []string{"a"},
				InstanceClass:     rawExtension("null"),
				SerializedLabels:  "node.deckhouse.io/group=worker-nil-class",
				UpdateEpoch:       "1005",
			},
		},
		{
			name: "cloud-ephemeral-processed-empty-zones",
			input: BlobInput{
				Name:     "worker-empty-zones",
				NodeType: v1.NodeTypeCloudEphemeral,
				RawSpec: map[string]interface{}{
					"nodeType": "CloudEphemeral",
					"cloudInstances": map[string]interface{}{
						"classReference": map[string]interface{}{"kind": "AWSInstanceClass", "name": "worker"},
					},
				},
				CloudProcessed: true,
			},
			result: Result{
				Engine:           engineMCM,
				CRIType:          criTypeContainerd,
				Zones:            []string{},
				InstanceClass:    rawExtension(`{"instanceType":"m5.large"}`),
				SerializedLabels: "node.deckhouse.io/group=worker-empty-zones",
				UpdateEpoch:      "1006",
			},
		},
		{
			name: "cloud-ephemeral-processed-nil-zones",
			input: BlobInput{
				Name:     "worker-nil-zones",
				NodeType: v1.NodeTypeCloudEphemeral,
				RawSpec: map[string]interface{}{
					"nodeType": "CloudEphemeral",
					"cloudInstances": map[string]interface{}{
						"classReference": map[string]interface{}{"kind": "AWSInstanceClass", "name": "worker"},
					},
				},
				CloudProcessed: true,
			},
			result: Result{
				Engine:           engineMCM,
				CRIType:          criTypeContainerd,
				Zones:            nil,
				InstanceClass:    rawExtension(`{"instanceType":"m5.large"}`),
				SerializedLabels: "node.deckhouse.io/group=worker-nil-zones",
				UpdateEpoch:      "1007",
			},
		},
		{
			name: "cloud-ephemeral-not-processed",
			input: BlobInput{
				Name:     "worker-unprocessed",
				NodeType: v1.NodeTypeCloudEphemeral,
				RawSpec: map[string]interface{}{
					"nodeType": "CloudEphemeral",
					"cloudInstances": map[string]interface{}{
						"classReference": map[string]interface{}{"kind": "AWSInstanceClass", "name": "worker"},
						"zones":          []interface{}{"eu-west-1a"},
					},
					"kubelet": kubeletDefaults(),
				},
				CloudProcessed: false,
			},
			// A failed (or skipped) cloud check drops the whole cloud overlay even though the
			// computed values are there.
			result: Result{
				Engine:           engineMCM,
				CRIType:          criTypeContainerd,
				Zones:            []string{"eu-west-1a", "eu-west-1b"},
				NodeCapacity:     rawExtension(`{"cpu":"2","memory":"4Gi"}`),
				InstanceClass:    rawExtension(`{"instanceType":"m5.large"}`),
				SerializedLabels: "node.deckhouse.io/group=worker-unprocessed",
				UpdateEpoch:      "1008",
			},
		},
		{
			name: "empty-allowlist-values",
			input: BlobInput{
				Name:     "empty-values",
				NodeType: v1.NodeTypeStatic,
				RawSpec: map[string]interface{}{
					"nodeType":               "Static",
					"cri":                    map[string]interface{}{},
					"gpu":                    nil,
					"staticInstances":        map[string]interface{}{},
					"cloudInstances":         map[string]interface{}{},
					"nodeTemplate":           map[string]interface{}{},
					"chaos":                  map[string]interface{}{},
					"operatingSystem":        map[string]interface{}{},
					"disruptions":            nil,
					"kubelet":                map[string]interface{}{},
					"fencing":                map[string]interface{}{},
					"nodeDrainTimeoutSecond": int64(0),
				},
			},
			// nodeDrainTimeoutSecond: 0 is not an empty value — only nil, "", {} and [] are
			// dropped, so an explicit zero reaches the context.
			result: Result{
				Engine:           engineNone,
				CRIType:          criTypeContainerd,
				SerializedLabels: "node.deckhouse.io/group=empty-values",
				UpdateEpoch:      "1009",
			},
		},
		{
			name: "unknown-spec-keys",
			input: BlobInput{
				Name:     "unknown-keys",
				NodeType: v1.NodeTypeStatic,
				RawSpec: map[string]interface{}{
					"nodeType":     "Static",
					"update":       map[string]interface{}{"maxConcurrent": int64(5)},
					"approval":     map[string]interface{}{"automatic": true},
					"unknownField": "value",
					"gpu":          map[string]interface{}{"sharing": "MPS"},
				},
			},
			result: Result{
				Engine:           engineNone,
				CRIType:          criTypeContainerd,
				SerializedLabels: "node.deckhouse.io/group=unknown-keys",
				UpdateEpoch:      "1010",
			},
		},
		{
			name: "manual-rollout-id",
			input: BlobInput{
				Name:            "rolled",
				ManualRolloutID: "2026-07-28",
				NodeType:        v1.NodeTypeStatic,
				RawSpec:         map[string]interface{}{"nodeType": "Static"},
			},
			result: Result{
				Engine:           engineNone,
				CRIType:          criTypeContainerd,
				SerializedLabels: "node.deckhouse.io/group=rolled",
				UpdateEpoch:      "1011",
			},
		},
		{
			name: "numbers-and-quantities",
			input: BlobInput{
				Name:     "numbers",
				NodeType: v1.NodeTypeCloudEphemeral,
				RawSpec: map[string]interface{}{
					"nodeType": "CloudEphemeral",
					"cloudInstances": map[string]interface{}{
						"minPerZone": int64(0),
						"maxPerZone": int64(20),
						"standby":    "15%",
						"standbyHolder": map[string]interface{}{
							"overprovisioningRate": float64(0.5),
						},
					},
					"kubelet": map[string]interface{}{
						"containerLogMaxSize":  "50Mi",
						"containerLogMaxFiles": int64(4),
						"maxPods":              int64(110),
						"resourceReservation": map[string]interface{}{
							"mode": "Static",
							"static": map[string]interface{}{
								"cpu":              "500m",
								"memory":           "1Gi",
								"ephemeralStorage": "2Gi",
							},
						},
					},
					// Fractional on purpose: the passthrough must never coerce a spec number
					// to the Go type the CRD happens to declare for it.
					"nodeDrainTimeoutSecond": float64(45.5),
				},
				CloudProcessed: true,
			},
			result: Result{
				Engine:            engineMCM,
				KubernetesVersion: "1.33",
				CRIType:           criTypeContainerd,
				Zones:             []string{"a"},
				NodeCapacity:      rawExtension(`{"cpu":"3900m","memory":"7969960Ki"}`),
				InstanceClass:     rawExtension(`{"cores":4,"memory":8589934592,"coreFraction":0.5,"gpus":0,"spot":false,"additionalTags":{},"subnets":[]}`),
				SerializedLabels:  "node.deckhouse.io/group=numbers",
				UpdateEpoch:       "1012",
			},
		},
		{
			name: "cri-docker-not-managed",
			input: BlobInput{
				Name:     "docker-ng",
				NodeType: v1.NodeTypeStatic,
				RawSpec: map[string]interface{}{
					"nodeType": "Static",
					"cri": map[string]interface{}{
						"type":       "Docker",
						"docker":     map[string]interface{}{"manage": false, "maxConcurrentDownloads": int64(3)},
						"containerd": map[string]interface{}{"maxConcurrentDownloads": int64(3)},
					},
				},
			},
			result: Result{
				Engine:           engineNone,
				CRIType:          criTypeNotManaged,
				SerializedLabels: "node.deckhouse.io/group=docker-ng",
				UpdateEpoch:      "1013",
			},
		},
		{
			name: "empty-node-type",
			input: BlobInput{
				Name:    "no-type",
				RawSpec: map[string]interface{}{},
			},
			result: Result{
				CRIType:     criTypeContainerd,
				UpdateEpoch: "1014",
			},
		},
	}
}

// TestBuildNodeGroupBlob_CorpusGoldens pins the published element of every NodeGroup shape.
// The comparison is on the parsed documents, not on bytes: bashible-apiserver re-marshals what
// it reads, so key order is irrelevant — but key presence and values are the node configuration
// checksum, and a drift here rebuilds every node's bashible steps.
func TestBuildNodeGroupBlob_CorpusGoldens(t *testing.T) {
	for _, fixture := range blobCorpus() {
		t.Run(fixture.name, func(t *testing.T) {
			assertBlobGolden(t, fixture.name, buildNodeGroupBlob(fixture.input, fixture.result))
		})
	}
}

func assertBlobGolden(t *testing.T, name string, blob map[string]interface{}) {
	t.Helper()

	raw, err := sigsyaml.Marshal(blob)
	require.NoError(t, err)

	path := filepath.Join("testdata", "blob", name+".yaml")
	if *updateGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, raw, 0o644))
	}

	golden, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s: regenerate with go test ./internal/controller/nodegroup/derived_status/ -run CorpusGoldens -update-goldens", path)

	var got, want map[string]interface{}
	require.NoError(t, sigsyaml.Unmarshal(raw, &got))
	require.NoError(t, sigsyaml.Unmarshal(golden, &want))

	assert.Equal(t, want, got)
}
