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

package machinetemplate

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A false "changed" is the expensive failure: it recreates a user's machines for nothing. It
// cannot come from a field the user edited — it comes from the comparison itself disagreeing with
// its own inputs, which is exactly what the v1 byte hash did (int64 vs float64, a re-serialized
// map, a quantity written differently).
//
// So: the same spec, round-tripped the way it really travels — API object → snapshot annotation →
// back — must never report a change, whatever is in it.

func TestChangesNeverFiresOnAnUnchangedSpec(t *testing.T) {
	source := rand.New(rand.NewSource(20260731))

	for i := range 300 {
		t.Run(fmt.Sprintf("spec-%d", i), func(t *testing.T) {
			spec := randomSpec(source, 3)
			fields := allPaths(spec, "")
			if len(fields) == 0 {
				return
			}

			// The path a snapshot really takes: json.Marshal into the annotation, json.Unmarshal
			// back out on the next reconcile.
			encoded, err := json.Marshal(spec)
			require.NoError(t, err)
			decoded := map[string]any{}
			require.NoError(t, json.Unmarshal(encoded, &decoded))

			changes, err := Changes(decoded, spec, fields)
			require.NoError(t, err)
			assert.Empty(t, changes, "an unchanged spec must never look changed: %v", changes)
		})
	}
}

// The same number reaching the two sides as different Go types is the single most likely source of
// a phantom rollout: unstructured gives int64, the snapshot gives float64.
func TestChangesIgnoresNumericRepresentation(t *testing.T) {
	tests := []struct {
		name string
		old  any
		new  any
	}{
		{name: "int64 vs float64", old: int64(50), new: float64(50)},
		{name: "int vs float64", old: 50, new: float64(50)},
		{name: "int32 vs int64", old: int32(4), new: int64(4)},
		{name: "float64 with zero fraction vs int64", old: float64(8192), new: int64(8192)},
		{name: "nested int64 vs float64", old: map[string]any{"cores": int64(4)}, new: map[string]any{"cores": float64(4)}},
		{name: "list of int64 vs float64", old: []any{int64(1), int64(2)}, new: []any{float64(1), float64(2)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := Changes(map[string]any{"field": tc.old}, map[string]any{"field": tc.new}, []string{"field"})
			require.NoError(t, err)
			assert.Empty(t, changes, "the same value in a different Go type must not roll machines")
		})
	}
}

// And the converse: a real difference must still be seen, so the test above cannot pass by
// comparing nothing.
func TestChangesSeesRealDifferences(t *testing.T) {
	tests := []struct {
		name string
		old  any
		new  any
	}{
		{name: "different numbers", old: int64(50), new: int64(51)},
		{name: "number vs string", old: int64(50), new: "50"},
		{name: "quantity suffix", old: "50Gi", new: "50G"},
		{name: "list order", old: []any{"a", "b"}, new: []any{"b", "a"}},
		{name: "extra map key", old: map[string]any{"a": "1"}, new: map[string]any{"a": "1", "b": "2"}},
		{name: "true vs string true", old: true, new: "true"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := Changes(map[string]any{"field": tc.old}, map[string]any{"field": tc.new}, []string{"field"})
			require.NoError(t, err)
			assert.Len(t, changes, 1, "a real change must be reported")
		})
	}
}

func randomSpec(source *rand.Rand, depth int) map[string]any {
	spec := map[string]any{}
	for i := range source.Intn(6) + 1 {
		key := fmt.Sprintf("field%d", i)
		spec[key] = randomValue(source, depth)
	}
	return spec
}

func randomValue(source *rand.Rand, depth int) any {
	kind := source.Intn(8)
	if depth <= 0 && kind >= 6 {
		kind = 0
	}
	switch kind {
	case 0:
		return fmt.Sprintf("value-%d", source.Intn(1000))
	case 1:
		return source.Intn(100)
	case 2:
		return int64(source.Intn(100000))
	case 3:
		return float64(source.Intn(1000)) + 0.5
	case 4:
		return source.Intn(2) == 0
	case 5:
		return nil
	case 6:
		list := make([]any, source.Intn(3)+1)
		for i := range list {
			list[i] = randomValue(source, depth-1)
		}
		return list
	default:
		return randomSpec(source, depth-1)
	}
}

// allPaths lists every path in a spec, including the intermediate maps: Changes must be safe on a
// rolloutField that names a subtree, not only a leaf.
func allPaths(spec map[string]any, prefix string) []string {
	paths := make([]string, 0, len(spec))
	for key, value := range spec {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		paths = append(paths, path)
		if nested, ok := value.(map[string]any); ok {
			paths = append(paths, allPaths(nested, path)...)
		}
	}
	return paths
}
