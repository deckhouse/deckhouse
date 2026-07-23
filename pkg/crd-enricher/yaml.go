// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crdenricher

import (
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"
	goyaml2 "sigs.k8s.io/yaml/goyaml.v2"
	goyaml "sigs.k8s.io/yaml/goyaml.v3"
)

// childMap returns the nested mapping stored under key, or nil when it is
// absent or not a mapping. Using sigs.k8s.io/yaml means every mapping decodes
// to a map[string]any with string keys.
func childMap(node map[string]any, key string) map[string]any {
	if node == nil {
		return nil
	}
	if child, ok := node[key].(map[string]any); ok {
		return child
	}
	return nil
}

// decodeValue parses a marker value as YAML, yielding scalars, lists or maps
// that mirror the representation used by the rest of the document. Mappings
// decode to map[string]any, so their keys are rendered in sorted order.
func decodeValue(raw string) (any, error) {
	var out any
	if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode value %q: %w", raw, err)
	}
	return out, nil
}

// orderedEntry is a single key/value pair of an orderedMap.
type orderedEntry struct {
	key string
	val any
}

// orderedMap is a mapping that preserves the key order in which it was authored.
// It is used for x-doc-examples values so that the fields of an example are
// rendered in the same order the author wrote them, instead of the alphabetical
// order a plain map[string]any is forced into by the YAML encoder. Examples
// double as ready-to-copy manifests, so their field order is meaningful.
type orderedMap []orderedEntry

// decodeOrderedValue parses a marker value like decodeValue, but preserves the
// key order of every mapping by decoding it into an orderedMap. Scalars and
// sequences keep their natural Go representation, so scalar examples still
// compare and render exactly as before.
func decodeOrderedValue(raw string) (any, error) {
	var node goyaml.Node
	if err := goyaml.Unmarshal([]byte(raw), &node); err != nil {
		return nil, fmt.Errorf("decode value %q: %w", raw, err)
	}
	if node.Kind == goyaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil, nil
		}
		return nodeToOrdered(node.Content[0])
	}
	return nodeToOrdered(&node)
}

// nodeToOrdered converts a decoded YAML node into the enricher's value model,
// turning mappings into order-preserving orderedMaps while decoding sequences
// and scalars into the usual []any and typed scalar values.
func nodeToOrdered(n *goyaml.Node) (any, error) {
	switch n.Kind {
	case goyaml.MappingNode:
		out := make(orderedMap, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			val, err := nodeToOrdered(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			out = append(out, orderedEntry{key: n.Content[i].Value, val: val})
		}
		return out, nil
	case goyaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, child := range n.Content {
			v, err := nodeToOrdered(child)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	default:
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, fmt.Errorf("decode scalar %q: %w", n.Value, err)
		}
		return v, nil
	}
}

// containsOrdered reports whether a value holds an orderedMap anywhere in its
// tree. It tells enrichFile whether the order-preserving encoder is needed: a
// document without ordered examples can keep the default sigs.k8s.io/yaml
// encoding untouched.
func containsOrdered(v any) bool {
	switch t := v.(type) {
	case orderedMap:
		return true
	case map[string]any:
		for _, child := range t {
			if containsOrdered(child) {
				return true
			}
		}
	case []any:
		for _, child := range t {
			if containsOrdered(child) {
				return true
			}
		}
	}
	return false
}

// plainIfSorted collapses a decoded example value into the plain
// map[string]any / []any model when every mapping it contains is already in
// ascending key order. Such an example renders identically under the default
// sigs.k8s.io/yaml encoder (which sorts map keys), so preserving the authored
// order is a no-op — collapsing it keeps the document on the default encoder
// and leaves every other node byte for byte unchanged, avoiding a whole-file
// reindent. When any mapping is authored out of order the key order carries
// meaning, so the value is left untouched (ok=false) and the order-preserving
// encoder handles it as before.
func plainIfSorted(v any) (any, bool) {
	switch t := v.(type) {
	case orderedMap:
		out := make(map[string]any, len(t))
		keys := make([]string, 0, len(t))
		for _, entry := range t {
			child, ok := plainIfSorted(entry.val)
			if !ok {
				return nil, false
			}
			out[entry.key] = child
			keys = append(keys, entry.key)
		}
		if !sort.StringsAreSorted(keys) {
			return nil, false
		}
		return out, true
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			child, ok := plainIfSorted(item)
			if !ok {
				return nil, false
			}
			out[i] = child
		}
		return out, true
	default:
		return v, true
	}
}

// marshalOrdered encodes a CRD document while preserving the key order of every
// orderedMap and sorting the keys of every plain map[string]any, so that the
// only nodes that escape the alphabetical order sigs.k8s.io/yaml would impose
// are the authored examples. It is used only for documents that actually carry
// ordered examples; everything else keeps the sigs.k8s.io/yaml encoding.
//
// It marshals with goyaml.v2 — the same encoder sigs.k8s.io/yaml serialises
// through — so block sequences stay flush with their parent key. This is what
// keeps a document that gains an ordered example byte-identical to the default
// encoding everywhere except the ordered nodes themselves; goyaml.v3 (used only
// to decode marker values) indents sequences and would reindent the whole file.
func marshalOrdered(v any) ([]byte, error) {
	out, err := goyaml2.Marshal(toV2(v))
	if err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	return out, nil
}

// toV2 converts the enricher value model into the types the goyaml.v2 encoder
// renders in sigs.k8s.io/yaml's style: orderedMaps become MapSlice so their
// authored key order is preserved, plain maps stay maps so goyaml.v2 emits their
// keys sorted (matching the default encoder), and sequences and scalars pass
// through unchanged.
func toV2(v any) any {
	switch t := v.(type) {
	case orderedMap:
		out := make(goyaml2.MapSlice, 0, len(t))
		for _, entry := range t {
			out = append(out, goyaml2.MapItem{Key: entry.key, Value: toV2(entry.val)})
		}
		return out

	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = toV2(val)
		}
		return out

	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = toV2(item)
		}
		return out

	default:
		return v
	}
}
