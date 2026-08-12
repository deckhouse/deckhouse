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

func TestResolvedNodeGroup_LegacyParityProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(propertySeed))

	for i := range propertyIterations {
		in, result := randomResolveInput(rng)

		require.Equalf(t,
			legacyBuildNodeGroupBlob(in, result),
			resolvedMap(in, result),
			"iteration %d\ninput: %#v\nresult: %#v", i, in, result,
		)
	}
}

func randomResolveInput(rng *rand.Rand) (ResolveInput, Result) {
	in := ResolveInput{
		Name:            randomString(rng, "ng"),
		ManualRolloutID: randomString(rng, "rollout"),
		NodeType:        randomNodeType(rng),
		RawSpec:         randomRawSpec(rng),
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

// randomRawSpec puts a random subset of the allowlisted keys — plus keys that must never be
// copied — into a spec, with values of every shape the unstructured NodeGroup can carry.
func randomRawSpec(rng *rand.Rand) map[string]interface{} {
	if rng.Intn(10) == 0 {
		return nil
	}

	spec := map[string]interface{}{"nodeType": "Static"}
	if rng.Intn(2) == 0 {
		spec["update"] = map[string]interface{}{"maxConcurrent": int64(rng.Intn(10))}
	}
	if rng.Intn(2) == 0 {
		spec["approval"] = "not-in-the-allowlist"
	}

	for _, key := range specPassthroughKeys {
		if rng.Intn(3) == 0 {
			continue
		}
		spec[key] = randomValue(rng, 0)
	}
	return spec
}

func randomValue(rng *rand.Rand, depth int) interface{} {
	kinds := 12
	if depth >= 2 {
		kinds = 10
	}

	switch rng.Intn(kinds) {
	case 0:
		return nil
	case 1:
		return ""
	case 2:
		return map[string]interface{}{}
	case 3:
		return []interface{}{}
	case 4:
		return int64(rng.Intn(1000) - 500)
	case 5:
		return float64(rng.Intn(1000)) / 4
	case 6:
		// Larger than 2^53: the JSON round-trip of a float64 would lose it.
		return int64(9007199254740993)
	case 7:
		return rng.Intn(2) == 0
	case 8:
		// Quantities and percentages stay strings all the way to the node.
		return []string{"50Gi", "500m", "15%", "8589934592"}[rng.Intn(4)]
	case 9:
		return "type"
	case 10:
		size := rng.Intn(3) + 1
		out := make(map[string]interface{}, size)
		for i := range size {
			out[fmt.Sprintf("key-%d", i)] = randomValue(rng, depth+1)
		}
		return out
	default:
		size := rng.Intn(3) + 1
		out := make([]interface{}, 0, size)
		for range size {
			out = append(out, randomValue(rng, depth+1))
		}
		return out
	}
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
