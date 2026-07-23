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
	"bytes"
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"
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
func marshalOrdered(v any) ([]byte, error) {
	node, err := toNode(v)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := goyaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	return buf.Bytes(), nil
}

// toNode builds a YAML node tree from the enricher's value model. orderedMaps
// keep their authored order; plain maps are emitted with sorted keys to match
// the sigs.k8s.io/yaml output for every non-example node; sequences and scalars
// are encoded as usual.
func toNode(v any) (*goyaml.Node, error) {
	switch t := v.(type) {
	case orderedMap:
		node := &goyaml.Node{Kind: goyaml.MappingNode}
		for _, entry := range t {
			key, err := scalarNode(entry.key)
			if err != nil {
				return nil, err
			}
			val, err := toNode(entry.val)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, key, val)
		}
		return node, nil

	case map[string]any:
		node := &goyaml.Node{Kind: goyaml.MappingNode}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key, err := scalarNode(k)
			if err != nil {
				return nil, err
			}
			val, err := toNode(t[k])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, key, val)
		}
		return node, nil

	case []any:
		node := &goyaml.Node{Kind: goyaml.SequenceNode}
		for _, item := range t {
			child, err := toNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil

	default:
		return scalarNode(v)
	}
}

// scalarNode encodes a single scalar (or nil) into a YAML node, letting the
// encoder choose the tag and quoting the same way a top-level Marshal would.
func scalarNode(v any) (*goyaml.Node, error) {
	node := &goyaml.Node{}
	if err := node.Encode(v); err != nil {
		return nil, fmt.Errorf("encode scalar %v: %w", v, err)
	}
	return node, nil
}
