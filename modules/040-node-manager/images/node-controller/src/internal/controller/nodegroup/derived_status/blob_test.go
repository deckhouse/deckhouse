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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestBuildNodeGroupBlob_SpecPassthrough(t *testing.T) {
	// gpu.sharing is a CRD field absent from the hand-rolled node-controller
	// v1.GPUSpec. Passthrough must preserve it verbatim, proving the blob does
	// not round-trip through the divergent typed struct.
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "worker",
		NodeType: v1.NodeTypeStatic,
		RawSpec: map[string]interface{}{
			"nodeType": "Static",
			"gpu":      map[string]interface{}{"sharing": "TimeSlicing"},
			"update":   map[string]interface{}{"maxConcurrent": 5},
		},
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.29",
		CRIType:           "Containerd",
		SerializedLabels:  "node.deckhouse.io/group=worker",
		SerializedTaints:  "",
		UpdateEpoch:       "12345",
	})

	assert.Equal(t, map[string]interface{}{"sharing": "TimeSlicing"}, blob["gpu"],
		"gpu.sharing must survive verbatim")
	assert.NotContains(t, blob, "update", "spec.update must be dropped (not in nodeGroupForValues)")
	assert.Equal(t, "worker", blob["name"])
	assert.Equal(t, "None", blob["engine"])
	assert.Equal(t, "1.29", blob["kubernetesVersion"])
	assert.Equal(t, "12345", blob["updateEpoch"])
	// cri is synthesized from the resolved type even without a spec cri block.
	assert.Equal(t, map[string]interface{}{"type": "Containerd"}, blob["cri"])
}

func TestBuildNodeGroupBlob_CRITypeOverride(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "worker",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cri":      map[string]interface{}{"type": "Docker", "docker": map[string]interface{}{"manage": true}},
		},
	}, Result{CRIType: "NotManaged"})

	cri, ok := blob["cri"].(map[string]interface{})
	if assert.True(t, ok) {
		assert.Equal(t, "NotManaged", cri["type"], "resolved cri.type overrides the spec value")
		assert.Equal(t, map[string]interface{}{"manage": true}, cri["docker"], "other cri fields preserved")
	}
}

func TestBuildNodeGroupBlob_StaticEmbedded(t *testing.T) {
	static := map[string]interface{}{"internalNetworkCIDRs": []interface{}{"192.168.0.0/24"}}

	staticNG := BuildNodeGroupBlob(BlobInput{
		Name:     "s", NodeType: v1.NodeTypeStatic,
		RawSpec: map[string]interface{}{"nodeType": "Static"},
		Static:  static,
	}, Result{CRIType: "Containerd"})
	assert.Equal(t, static, staticNG["static"], "static value embedded for Static NG")

	cloudNG := BuildNodeGroupBlob(BlobInput{
		Name:     "c", NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{"nodeType": "CloudEphemeral"},
		Static:  static,
	}, Result{CRIType: "Containerd"})
	assert.NotContains(t, cloudNG, "static", "static must not leak into non-Static NG")
}

func TestBuildNodeGroupBlob_CloudProcessed(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "cloud",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType":       "CloudEphemeral",
			"cloudInstances": map[string]interface{}{"minPerZone": float64(0), "maxPerZone": float64(3)},
		},
		CloudProcessed: true,
	}, Result{
		Engine:        "CAPI",
		CRIType:       "Containerd",
		Zones:         []string{"a", "b"},
		NodeCapacity:  &runtime.RawExtension{Raw: []byte(`{"cpu":"4","memory":"8Gi"}`)},
		InstanceClass: &runtime.RawExtension{Raw: []byte(`{"flavorName":"m1.large"}`)},
	})

	ci, ok := blob["cloudInstances"].(map[string]interface{})
	if assert.True(t, ok) {
		assert.Equal(t, []string{"a", "b"}, ci["zones"], "resolved zones overlaid")
		assert.Equal(t, float64(3), ci["maxPerZone"], "spec cloudInstances fields preserved")
	}
	assert.Equal(t, map[string]interface{}{"cpu": "4", "memory": "8Gi"}, blob["nodeCapacity"],
		"nodeCapacity embedded as nested structure")
	assert.Equal(t, map[string]interface{}{"flavorName": "m1.large"}, blob["instanceClass"])
}

func TestBuildNodeGroupBlob_CloudNotProcessed(t *testing.T) {
	// When the provider/instance-class checks are skipped, get_crds does not add
	// instanceClass/nodeCapacity/zones.
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "cloud",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec:  map[string]interface{}{"nodeType": "CloudEphemeral"},
	}, Result{
		CRIType:       "Containerd",
		Zones:         []string{"a"},
		InstanceClass: &runtime.RawExtension{Raw: []byte(`{"flavorName":"m1.large"}`)},
	})

	assert.NotContains(t, blob, "instanceClass")
	assert.NotContains(t, blob, "nodeCapacity")
	assert.NotContains(t, blob, "cloudInstances")
}

// fencing is a CRD field the hand-rolled node-controller v1.NodeGroupSpec lacks
// entirely, yet it is part of the blob and the bashible bootstrap-checksum. Raw
// passthrough must carry it verbatim; assembling from the typed struct would drop
// it and trigger a mass node re-bootstrap.
func TestBuildNodeGroupBlob_FencingPassthrough(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "worker",
		NodeType: v1.NodeTypeStatic,
		RawSpec: map[string]interface{}{
			"nodeType": "Static",
			"staticInstances": map[string]interface{}{
				"labelSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"node-group": "worker"},
				},
			},
			"fencing": map[string]interface{}{"mode": "Watchdog"},
		},
	}, Result{
		Engine:            "None",
		KubernetesVersion: "1.32",
		CRIType:           "Containerd",
		SerializedLabels:  "node.deckhouse.io/group=worker",
	})

	assert.Equal(t, map[string]interface{}{"mode": "Watchdog"}, blob["fencing"],
		"fencing must survive verbatim (node-controller v1 has no Fencing field)")
	assert.Equal(t, map[string]interface{}{
		"labelSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{"node-group": "worker"},
		},
	}, blob["staticInstances"], "staticInstances passthrough preserved")
}

// serializedTaints is a computed Result field placed verbatim into the blob. The
// get_crds format is "key=value:effect" joined by "," with no sorting; the blob
// must reproduce it byte-for-byte (input to the bashible checksum).
func TestBuildNodeGroupBlob_SerializedTaints(t *testing.T) {
	blob := BuildNodeGroupBlob(BlobInput{
		Name:     "test",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec:  map[string]interface{}{"nodeType": "CloudEphemeral"},
	}, Result{
		CRIType:          "Containerd",
		SerializedTaints: "b=v:NoExecute,a,d:NoExecute,c=v1:",
	})

	assert.Equal(t, "b=v:NoExecute,a,d:NoExecute,c=v1:", blob["serializedTaints"],
		"serializedTaints placed verbatim, unsorted")
}

func TestBuildNodeGroupBlob_DoesNotMutateRawSpec(t *testing.T) {
	rawCRI := map[string]interface{}{"type": "Docker"}
	rawSpec := map[string]interface{}{"nodeType": "Static", "cri": rawCRI}

	BuildNodeGroupBlob(BlobInput{
		Name: "w", NodeType: v1.NodeTypeStatic, RawSpec: rawSpec,
	}, Result{CRIType: "Containerd"})

	assert.Equal(t, "Docker", rawCRI["type"], "source spec cri must not be mutated by the overlay")
}
