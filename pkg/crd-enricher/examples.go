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

// The example generator builds x-doc-examples values from the bottom up. Every
// scalar leaf yields a single representative value, taken either from an
// explicit examples marker or from a hard-coded fallback; every composite node
// (object, array, map) aggregates the values of its children into a structured
// example. This way a full resource example can be assembled automatically
// instead of being hand-written next to the root type.
const (
	// exampleScopeTree attaches a generated composite example to every object
	// node of the schema. Any other scope (including the default empty value)
	// attaches a single example to the CRD root only.
	exampleScopeTree = "tree"

	// exampleMetadataName is the metadata.name used in the synthesized root
	// example.
	exampleMetadataName = "example"

	// examplePlaceholderKey is the sample key used when a map (a node with
	// additionalProperties) is rendered into an example.
	examplePlaceholderKey = "key"

	// exampleStringValue and exampleDateTimeValue are the hard-coded fallbacks
	// for string leaves without an explicit example.
	exampleStringValue   = "string"
	exampleDateTimeValue = "2024-01-01T00:00:00Z"
)

// generateExamples fills in x-doc-examples for a single version schema. In tree
// scope every nested composite node receives its own aggregated example; in any
// case the CRD root receives a synthesized example carrying apiVersion, kind and
// metadata. A node that already has an explicit examples marker is left
// untouched: explicit examples always win over generated ones.
func (e *Enricher) generateExamples(spec, names map[string]any, version string, root map[string]any) {
	if root == nil {
		return
	}

	if e.exampleScope == exampleScopeTree {
		e.attachTreeExamples(root)
	}

	// The root example is always synthesized (unless overridden) so the curated
	// CRDs carry a complete usage example without a hand-written marker.
	if _, ok := root["x-doc-examples"]; ok {
		return
	}
	if ex := e.rootExample(spec, names, version, root); len(ex) > 0 {
		root["x-doc-examples"] = []any{ex}
	}
}

// attachTreeExamples walks the descendants of node and attaches an aggregated
// example to every composite child that lacks an explicit one. The root node
// itself is skipped here: it is handled by rootExample so the synthesized
// apiVersion/kind/metadata are not duplicated as a plain object example.
func (e *Enricher) attachTreeExamples(node map[string]any) {
	for _, child := range exampleChildren(node) {
		e.attachTreeExamples(child)
		e.setComposite(child)
	}
}

// setComposite stores an aggregated example on a composite node when it does not
// already carry one.
func (e *Enricher) setComposite(node map[string]any) {
	if !isComposite(node) {
		return
	}
	if _, ok := node["x-doc-examples"]; ok {
		return
	}
	if v, ok := e.computeExample(node); ok {
		node["x-doc-examples"] = []any{v}
	}
}

// rootExample synthesizes the example for the CRD root: it injects apiVersion,
// kind and metadata (which the curated minimal schema strips) and aggregates the
// remaining spec-side properties. The status subtree is omitted, since examples
// document the desired state a user submits.
func (e *Enricher) rootExample(spec, names map[string]any, version string, root map[string]any) map[string]any {
	out := map[string]any{}

	group, _ := spec["group"].(string)
	if group != "" && version != "" {
		out["apiVersion"] = group + "/" + version
	}
	if kind, _ := names["kind"].(string); kind != "" {
		out["kind"] = kind
	}
	out["metadata"] = map[string]any{"name": exampleMetadataName}

	properties := childMap(root, "properties")
	for name, raw := range properties {
		switch name {
		case "apiVersion", "kind", "metadata", "status":
			continue
		}
		child, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := e.computeExample(child); ok {
			out[name] = v
		}
	}
	return out
}

// computeExample returns the representative example value for a schema node. An
// explicit examples marker wins; otherwise composite nodes aggregate their
// children and scalar leaves fall back to a hard-coded value.
func (e *Enricher) computeExample(node map[string]any) (any, bool) {
	if node == nil {
		return nil, false
	}

	if ex, ok := node["x-doc-examples"].([]any); ok && len(ex) > 0 {
		return ex[0], true
	}

	switch typeOf(node) {
	case "object":
		if props := childMap(node, "properties"); props != nil {
			out := map[string]any{}
			for name, raw := range props {
				child, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if v, ok := e.computeExample(child); ok {
					out[name] = v
				}
			}
			if len(out) > 0 {
				return out, true
			}
			return nil, false
		}
		if ap := childMap(node, "additionalProperties"); ap != nil {
			if v, ok := e.computeExample(ap); ok {
				return map[string]any{examplePlaceholderKey: v}, true
			}
		}
		return nil, false

	case "array":
		if items := childMap(node, "items"); items != nil {
			if v, ok := e.computeExample(items); ok {
				return []any{v}, true
			}
		}
		return nil, false

	default:
		return leafExample(node)
	}
}

// leafExample returns the hard-coded fallback value for a scalar leaf. The
// precedence is: the schema default (kubebuilder:default), then the documented
// default (x-doc-default), then the first enum value, then a placeholder chosen
// by the leaf type. Leaves with no recognisable type (free-form nodes) yield no
// example.
func leafExample(node map[string]any) (any, bool) {
	if d, ok := node["default"]; ok {
		return d, true
	}
	if d, ok := node["x-doc-default"]; ok {
		return d, true
	}
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		return enum[0], true
	}

	switch typeOf(node) {
	case "string":
		if f, _ := node["format"].(string); f == "date-time" {
			return exampleDateTimeValue, true
		}
		return exampleStringValue, true
	case "integer", "number":
		return 0, true
	case "boolean":
		return false, true
	default:
		return nil, false
	}
}

// exampleChildren returns the nested schema nodes that hold child values:
// object properties, array items and map additionalProperties.
func exampleChildren(node map[string]any) []map[string]any {
	var out []map[string]any
	if props := childMap(node, "properties"); props != nil {
		for _, raw := range props {
			if child, ok := raw.(map[string]any); ok {
				out = append(out, child)
			}
		}
	}
	if items := childMap(node, "items"); items != nil {
		out = append(out, items)
	}
	if ap := childMap(node, "additionalProperties"); ap != nil {
		out = append(out, ap)
	}
	return out
}

// isComposite reports whether a node aggregates child values (an object with
// properties or additionalProperties, or an array with items) rather than being
// a scalar leaf.
func isComposite(node map[string]any) bool {
	switch typeOf(node) {
	case "object":
		return childMap(node, "properties") != nil || childMap(node, "additionalProperties") != nil
	case "array":
		return childMap(node, "items") != nil
	}
	return false
}

// typeOf returns the OpenAPI "type" of a schema node, or an empty string.
func typeOf(node map[string]any) string {
	t, _ := node["type"].(string)
	return t
}
