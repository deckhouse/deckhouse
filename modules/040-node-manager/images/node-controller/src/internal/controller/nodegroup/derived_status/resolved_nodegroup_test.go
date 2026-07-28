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

func TestResolvedNodeGroup_SpecPassthrough(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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

	assert.Equal(t, map[string]interface{}{"sharing": "TimeSlicing"}, nodeGroupValues["gpu"],
		"gpu.sharing must survive verbatim")
	assert.NotContains(t, nodeGroupValues, "update", "spec.update must be dropped (not in nodeGroupForValues)")
	assert.Equal(t, "worker", nodeGroupValues["name"])
	assert.Equal(t, "None", nodeGroupValues["engine"])
	assert.Equal(t, "1.29", nodeGroupValues["kubernetesVersion"])
	assert.Equal(t, "12345", nodeGroupValues["updateEpoch"])
	// cri is synthesized from the resolved type even without a spec cri block.
	assert.Equal(t, map[string]interface{}{"type": "Containerd"}, nodeGroupValues["cri"])
}

func TestResolvedNodeGroup_CRITypeOverride(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
		Name:     "worker",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{
			"nodeType": "CloudEphemeral",
			"cri":      map[string]interface{}{"type": "Docker", "docker": map[string]interface{}{"manage": true}},
		},
	}, Result{CRIType: "NotManaged"})

	cri, ok := nodeGroupValues["cri"].(map[string]interface{})
	if assert.True(t, ok) {
		assert.Equal(t, "NotManaged", cri["type"], "resolved cri.type overrides the spec value")
		assert.Equal(t, map[string]interface{}{"manage": true}, cri["docker"], "other cri fields preserved")
	}
}

func TestResolvedNodeGroup_StaticEmbedded(t *testing.T) {
	static := map[string]interface{}{"internalNetworkCIDRs": []interface{}{"192.168.0.0/24"}}

	staticNG := resolvedMap(ResolveInput{
		Name: "s", NodeType: v1.NodeTypeStatic,
		RawSpec: map[string]interface{}{"nodeType": "Static"},
		Static:  static,
	}, Result{CRIType: "Containerd"})
	assert.Equal(t, static, staticNG["static"], "static value embedded for Static NG")

	cloudNG := resolvedMap(ResolveInput{
		Name: "c", NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec: map[string]interface{}{"nodeType": "CloudEphemeral"},
		Static:  static,
	}, Result{CRIType: "Containerd"})
	assert.NotContains(t, cloudNG, "static", "static must not leak into non-Static NG")
}

func TestResolvedNodeGroup_CloudProcessed(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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

	ci, ok := nodeGroupValues["cloudInstances"].(map[string]interface{})
	if assert.True(t, ok) {
		assert.Equal(t, []string{"a", "b"}, ci["zones"], "resolved zones overlaid")
		assert.Equal(t, float64(3), ci["maxPerZone"], "spec cloudInstances fields preserved")
	}
	assert.Equal(t, map[string]interface{}{"cpu": "4", "memory": "8Gi"}, nodeGroupValues["nodeCapacity"],
		"nodeCapacity embedded as nested structure")
	assert.Equal(t, map[string]interface{}{"flavorName": "m1.large"}, nodeGroupValues["instanceClass"])
}

func TestResolvedNodeGroup_CloudNotProcessed(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
		Name:     "cloud",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec:  map[string]interface{}{"nodeType": "CloudEphemeral"},
	}, Result{
		CRIType:       "Containerd",
		Zones:         []string{"a"},
		InstanceClass: &runtime.RawExtension{Raw: []byte(`{"flavorName":"m1.large"}`)},
	})

	assert.NotContains(t, nodeGroupValues, "instanceClass")
	assert.NotContains(t, nodeGroupValues, "nodeCapacity")
	assert.NotContains(t, nodeGroupValues, "cloudInstances")
}

func TestResolvedNodeGroup_FencingPassthrough(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
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

	assert.Equal(t, map[string]interface{}{"mode": "Watchdog"}, nodeGroupValues["fencing"],
		"fencing must survive verbatim (node-controller v1 has no Fencing field)")
	assert.Equal(t, map[string]interface{}{
		"labelSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{"node-group": "worker"},
		},
	}, nodeGroupValues["staticInstances"], "staticInstances passthrough preserved")
}

func TestResolvedNodeGroup_SerializedTaints(t *testing.T) {
	nodeGroupValues := resolvedMap(ResolveInput{
		Name:     "test",
		NodeType: v1.NodeTypeCloudEphemeral,
		RawSpec:  map[string]interface{}{"nodeType": "CloudEphemeral"},
	}, Result{
		CRIType:          "Containerd",
		SerializedTaints: "b=v:NoExecute,a,d:NoExecute,c=v1:",
	})

	assert.Equal(t, "b=v:NoExecute,a,d:NoExecute,c=v1:", nodeGroupValues["serializedTaints"],
		"serializedTaints placed verbatim, unsorted")
}

func TestResolvedNodeGroup_DoesNotMutateRawSpec(t *testing.T) {
	rawCRI := map[string]interface{}{"type": "Docker"}
	rawSpec := map[string]interface{}{"nodeType": "Static", "cri": rawCRI}

	resolvedMap(ResolveInput{
		Name: "w", NodeType: v1.NodeTypeStatic, RawSpec: rawSpec,
	}, Result{CRIType: "Containerd"})

	assert.Equal(t, "Docker", rawCRI["type"], "source spec cri must not be mutated by the overlay")
}
