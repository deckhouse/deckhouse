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
	"fmt"
	"math/rand"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

// The corpus covers the shapes a NodeGroup really takes; this covers the ones it could take.
// The seed is fixed so a failure is reproducible from the reported iteration alone.
const (
	propertySeed       = 20260728
	propertyIterations = 2000
)

// The published element has invariants that hold for every NodeGroup, whatever its spec: which
// keys are always there, which appear only with the cloud overlay, and which must never leak in.
// A key published empty is different data from an absent key — bashible-apiserver hashes the
// parsed element, so either mistake re-runs bashible on every node of the group.
func TestResolvedNodeGroup_ShapeInvariantsProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(propertySeed))

	alwaysPresent := []string{
		"name", "nodeType", "engine", "manualRolloutID",
		"kubernetesVersion", "serializedLabels", "serializedTaints", "updateEpoch", "cri",
	}

	for i := range propertyIterations {
		in, result := randomResolveInput(rng)
		out := resolvedMap(in, result)

		for _, key := range alwaysPresent {
			require.Containsf(t, out, key, "iteration %d: %q must always be published", i, key)
		}

		// cri.type is the resolved value, never the spec's own.
		cri, ok := out["cri"].(map[string]interface{})
		require.Truef(t, ok, "iteration %d: cri must be an object", i)
		require.Equalf(t, result.CRIType, cri["type"], "iteration %d: cri.type must be the resolved one", i)

		// The cloud overlay is gated as a whole, and instanceClass is present-with-null rather
		// than absent once the gate opens.
		if in.CloudProcessed {
			require.Containsf(t, out, "instanceClass", "iteration %d: overlay must publish instanceClass", i)
			require.Containsf(t, out, "cloudInstances", "iteration %d: overlay must publish cloudInstances", i)
			ci, ok := out["cloudInstances"].(map[string]interface{})
			require.Truef(t, ok, "iteration %d: cloudInstances must be an object", i)
			require.Equalf(t, result.Zones, ci["zones"], "iteration %d: resolved zones overlaid verbatim", i)
		} else {
			require.NotContainsf(t, out, "instanceClass", "iteration %d: no overlay without the gate", i)
			require.NotContainsf(t, out, "nodeCapacity", "iteration %d: no overlay without the gate", i)
		}

		// static belongs to Static NodeGroups only.
		if in.NodeType != v1.NodeTypeStatic {
			require.NotContainsf(t, out, "static", "iteration %d: static must not leak into %s", i, in.NodeType)
		}

		// Nothing outside the allowlist ever reaches the node.
		for key := range out {
			require.NotEqualf(t, "update", key, "iteration %d: spec.update is not published", i)
			require.NotEqualf(t, "approval", key, "iteration %d: spec.approval is not published", i)
		}

		// The element must not carry a value the resolver never set.
		require.Equalf(t, in.Name, out["name"], "iteration %d", i)
		require.Equalf(t, string(in.NodeType), out["nodeType"], "iteration %d", i)
		require.Equalf(t, result.UpdateEpoch, out["updateEpoch"], "iteration %d", i)
	}
}

func randomResolveInput(rng *rand.Rand) (ResolveInput, Result) {
	in := ResolveInput{
		Name:            randomString(rng, "ng"),
		ManualRolloutID: randomString(rng, "rollout"),
		NodeType:        randomNodeType(rng),
		Spec:            randomSpec(rng),
		Static:          randomStatic(rng),
		CloudProcessed:  rng.Intn(2) == 0,
	}
	result := Result{
		Engine:            randomString(rng, "engine"),
		KubernetesVersion: randomString(rng, "1.3"),
		CRIType:           randomString(rng, "cri"),
		Zones:             randomZones(rng),
		NodeCapacity:      randomNodeCapacity(rng),
		InstanceClass:     randomInstanceClass(rng),
		SerializedLabels:  randomString(rng, "labels"),
		SerializedTaints:  randomString(rng, "taints"),
		UpdateEpoch:       randomString(rng, "epoch"),
	}
	return in, result
}

func randomString(rng *rand.Rand, prefix string) string {
	switch rng.Intn(4) {
	case 0:
		return ""
	default:
		return fmt.Sprintf("%s-%d", prefix, rng.Intn(1000))
	}
}

func randomNodeType(rng *rand.Rand) v1.NodeType {
	types := []v1.NodeType{
		v1.NodeTypeStatic,
		v1.NodeTypeCloudEphemeral,
		v1.NodeTypeCloudPermanent,
		"",
		"Unknown",
	}
	return types[rng.Intn(len(types))]
}

// randomSpec builds a spec out of the typed fields, turning on a random subset of the allowlisted
// subtrees plus the ones that must never be published. It covers the shapes a NodeGroup can take
// rather than the shapes a map can take: the API server rejects everything else, so a generator
// that produced them would be testing states no cluster can reach.
func randomSpec(rng *rand.Rand) v1.NodeGroupSpec {
	spec := v1.NodeGroupSpec{NodeType: randomNodeType(rng)}

	// Never published, whatever it holds.
	if rng.Intn(2) == 0 {
		spec.Update = &v1.UpdateSpec{MaxConcurrent: ptrIntOrString(rng.Intn(10))}
	}

	if rng.Intn(3) != 0 {
		spec.CRI = &v1.CRISpec{Type: v1.CRIType(randomString(rng, "cri"))}
	}
	if rng.Intn(3) != 0 {
		spec.Kubelet = &v1.KubeletSpec{
			ContainerLogMaxFiles: ptrInt32(int32(rng.Intn(20))),
			ContainerLogMaxSize:  randomString(rng, "size"),
			MaxPods:              ptrInt32(int32(rng.Intn(300))),
		}
	}
	if rng.Intn(3) != 0 {
		spec.NodeTemplate = &v1.NodeTemplate{
			Labels:      map[string]string{"role": randomString(rng, "role")},
			Annotations: map[string]string{"ann": randomString(rng, "ann")},
		}
	}
	if rng.Intn(3) != 0 {
		spec.CloudInstances = &v1.CloudInstancesSpec{
			ClassReference: v1.ClassReference{Kind: randomString(rng, "Kind"), Name: randomString(rng, "name")},
			MinPerZone:     int32(rng.Intn(5)),
			MaxPerZone:     int32(rng.Intn(20)),
			Zones:          randomZones(rng),
		}
	}
	if rng.Intn(3) != 0 {
		spec.StaticInstances = &v1.StaticInstancesSpec{Count: ptrInt32(int32(rng.Intn(10)))}
	}
	if rng.Intn(3) != 0 {
		spec.Fencing = &v1.FencingSpec{Mode: "Watchdog"}
	}
	if rng.Intn(3) != 0 {
		spec.Disruptions = &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalMode(randomString(rng, "mode"))}
	}
	if rng.Intn(3) != 0 {
		spec.GPU = &v1.GPUSpec{Sharing: randomString(rng, "sharing")}
	}
	if rng.Intn(3) != 0 {
		spec.NodeDrainTimeoutSecond = ptrInt(rng.Intn(600))
	}
	return spec
}

func ptrInt(v int) *int       { return &v }
func ptrInt32(v int32) *int32 { return &v }

func ptrIntOrString(v int) *intstr.IntOrString {
	out := intstr.FromInt(v)
	return &out
}

func randomStatic(rng *rand.Rand) map[string]interface{} {
	switch rng.Intn(3) {
	case 0:
		return nil
	case 1:
		return map[string]interface{}{}
	default:
		return map[string]interface{}{"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"}}
	}
}

func randomZones(rng *rand.Rand) []string {
	switch rng.Intn(3) {
	case 0:
		return nil
	case 1:
		return []string{}
	default:
		return []string{"a", "b"}
	}
}

// randomInstanceClass covers the shapes an InstanceClass spec arrives in, including the ones that
// do not decode into a map at all — the field holds only objects, so those must land as nil rather
// than as a half-decoded value.
func randomInstanceClass(rng *rand.Rand) map[string]any {
	raws := []string{
		"",
		"null",
		"{}",
		"[]",
		`{"cores":4,"memory":8589934592,"coreFraction":0.5}`,
		`{"cpu":"3900m","memory":"7969960Ki"}`,
		"not json at all",
		"0",
		`""`,
	}

	if rng.Intn(6) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raws[rng.Intn(len(raws))]), &out); err != nil {
		return nil
	}
	return out
}

func randomNodeCapacity(rng *rand.Rand) *capacity.InstanceType {
	raws := []string{
		`{"cpu":"3900m","memory":"7969960Ki"}`,
		`{"cpu":"4","memory":"8Gi"}`,
		"{}",
	}

	if rng.Intn(6) == 0 {
		return nil
	}
	out := &capacity.InstanceType{}
	if err := json.Unmarshal([]byte(raws[rng.Intn(len(raws))]), out); err != nil {
		return nil
	}
	return out
}
