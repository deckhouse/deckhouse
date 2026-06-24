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
	"go/ast"
	"strings"
)

// markerPrefix namespaces every marker owned by the enricher. It is the single,
// canonical root every enricher marker carries; no bare or legacy forms are
// honoured. Two shapes are recognised:
//
//	+crd-enricher:raw:<key>[=<value>]                       // raw schema injection
//	+crd-enricher:deckhouse:documentation:<entity>[=<value>] // documentation entity
//
// The raw entity lives directly under the prefix because it injects a standard
// schema field rather than deckhouse-specific documentation. The documentation
// entities (crd, examples, deprecated, default) carry the extra
// "deckhouse:documentation" sub-namespace. Both shapes are reduced to the bare
// entity name during parsing so the rest of the enricher matches on it.
const markerPrefix = "crd-enricher:"

// docSubPrefix is the "deckhouse:documentation" sub-namespace stripped from the
// documentation entities after markerPrefix. The raw entity does not carry it.
const docSubPrefix = "deckhouse:documentation:"

// docKeyPrefix is the schema-field prefix the rendered CRDs use for the simple
// documentation entities: examples, deprecated and default render as
// x-doc-examples, x-doc-deprecated and x-doc-default respectively.
const docKeyPrefix = "x-doc-"

// examplesMarker is the entity whose values are always collected into a list
// instead of overwriting each other. It renders as x-doc-examples.
const examplesMarker = "examples"

// rawMarkerPrefix is the entity that injects an arbitrary standard schema field
// named by the <key> that follows it (not under an x-doc-* key). For example
// "+crd-enricher:raw:pattern=^\d+$" sets the schema
// "pattern" field, which is needed for fields controller-gen cannot annotate
// directly (such as a regex pattern on a metav1.Duration). A dotted <key> walks
// into nested schema nodes.
const rawMarkerPrefix = "raw:"

// crdMarker is the type-level entity that configures CRD-level settings that
// controller-gen cannot express (preserveUnknownFields, the minimal style and
// schema format stripping) and switches the document to the hand-curated
// deckhouse style. Its value is a YAML map, for example:
//
//	+crd-enricher:deckhouse:documentation:crd={preserveUnknownFields: false, minimal: true}
//
// Labels and annotations are not configured here: controller-gen emits them
// natively from the +kubebuilder:metadata:labels and
// +kubebuilder:metadata:annotations markers. It is handled separately from the
// schema-level documentation entities.
const crdMarker = "crd"

// rootMarker is the controller-gen marker that designates a Go type as the
// root object of a CRD. The enricher relies on it to know which types map to a
// generated CRD.
const rootMarker = "kubebuilder:object:root"

// marker is a single parsed comment marker, for example
// "+crd-enricher:deckhouse:documentation:default=3m".
type marker struct {
	// name is the marker name without the leading plus sign. For enricher
	// markers it is the entity name with markerPrefix already stripped, e.g.
	// "default", "examples" or "raw:pattern". For any other marker (such as a
	// kubebuilder marker) it is the verbatim name.
	name string
	// rawValue is the verbatim text after the first "=" sign, or an empty
	// string when the marker has no value.
	rawValue string
	// hasValue reports whether an "=" sign was present at all, so that a
	// genuinely empty value can be told apart from a value-less flag.
	hasValue bool
	// enricher reports whether the marker carried markerPrefix, i.e. it is one
	// of the enricher's own documentation markers.
	enricher bool
}

// isDoc reports whether the marker is one of the enricher's documentation
// markers (it carried markerPrefix) and therefore should be applied to a schema
// node or to the CRD.
func (m marker) isDoc() bool {
	return m.enricher
}

// parseMarkerLine turns a single trimmed comment line into a marker. The
// boolean result is false when the line is not a marker (does not start with a
// plus sign).
func parseMarkerLine(line string) (marker, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "+") {
		return marker{}, false
	}

	body := strings.TrimSpace(line[1:])
	if body == "" {
		return marker{}, false
	}

	var m marker
	if idx := strings.IndexByte(body, '='); idx >= 0 {
		m = marker{
			name:     strings.TrimSpace(body[:idx]),
			rawValue: strings.TrimSpace(body[idx+1:]),
			hasValue: true,
		}
	} else {
		m = marker{name: body}
	}

	// Enricher markers are namespaced with markerPrefix; strip it and the
	// optional "deckhouse:documentation" sub-namespace carried by the
	// documentation entities so downstream code matches on the bare entity name
	// (crd, raw:..., examples, …), and flag them as the enricher's own so they
	// are told apart from other markers.
	if rest, ok := strings.CutPrefix(m.name, markerPrefix); ok {
		m.name = strings.TrimPrefix(rest, docSubPrefix)
		m.enricher = true
	}

	return m, true
}

// parseCommentGroups extracts every marker found in the given comment groups.
// Both leading documentation comments and trailing inline comments are
// supported, and both // line comments and /* */ block comments are handled.
func parseCommentGroups(groups ...*ast.CommentGroup) []marker {
	var markers []marker
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			for _, line := range commentLines(comment.Text) {
				if m, ok := parseMarkerLine(line); ok {
					markers = append(markers, m)
				}
			}
		}
	}
	return markers
}

// commentLines strips the comment syntax from a raw comment token and returns
// its individual lines.
func commentLines(text string) []string {
	switch {
	case strings.HasPrefix(text, "//"):
		return []string{strings.TrimPrefix(text, "//")}
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimLeft(strings.TrimSpace(line), "*")
		}
		return lines
	default:
		return []string{text}
	}
}

// hasMarker reports whether the slice contains a marker with the given name.
func hasMarker(markers []marker, name string) bool {
	for _, m := range markers {
		if m.name == name {
			return true
		}
	}
	return false
}
