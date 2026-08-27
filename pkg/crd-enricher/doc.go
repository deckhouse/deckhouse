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

// Package crdenricher enriches CustomResourceDefinition manifests that were
// rendered by controller-gen (kubebuilder) with custom, non-standard schema
// fields that controller-gen is not able to emit on its own, such as
// x-doc-examples, x-doc-default or x-doc-deprecated.
//
// The enricher reads kubebuilder-style markers placed next to Go API structs
// and injects the corresponding x-doc-* keys into the matching nodes of the
// already generated openAPIV3Schema.
//
// # Markers
//
// Markers are regular Go comments that start with a plus sign, exactly like
// the markers consumed by controller-gen. Every enricher marker is namespaced
// with the canonical "crd-enricher:" prefix and comes in one of these shapes:
//
//	+crd-enricher:raw:<key>=<value>                          // raw schema injection
//	+crd-enricher:unset:<key>                                // raw schema removal
//	+crd-enricher:deckhouse:documentation:<entity>[=<value>] // documentation entity
//	+crd-enricher:crd:<key>[=<value>]                        // CRD-level setting
//	+crd-enricher:deckhouse:sensitive-data                   // sensitive field flag
//
// The raw and unset entities inject and remove a standard schema field and live
// directly under the prefix, as does the crd entity, which configures the CRD
// document itself; the documentation entities (examples, deprecated, default)
// carry the extra "deckhouse:documentation" sub-namespace, and sensitive-data
// carries the shorter "deckhouse" one. No bare or legacy form is recognised:
//
//	type ModuleSourceSpec struct {
//		// +crd-enricher:deckhouse:documentation:default=3m
//		// +crd-enricher:deckhouse:documentation:examples=5m
//		// +crd-enricher:deckhouse:documentation:examples=1h
//		// +crd-enricher:deckhouse:documentation:examples=6h30m
//		ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
//	}
//
// The value after the "=" sign is parsed as YAML, so scalars, lists and maps
// are all supported. The entities are:
//
//   - examples — collected into a list and rendered as x-doc-examples (the
//     marker may be repeated, and a value that is itself a YAML list is
//     flattened into it);
//
//   - examples-name / examples-description — attach a short name and/or a
//     description to the example introduced by the preceding examples marker. As
//     soon as any example has a name or a description, every entry of
//     x-doc-examples switches to the wrapper form {x-description, x-name,
//     x-example} (an entry missing either attribute omits its key); when no
//     example has one the list stays a plain list of values. The wrapper form
//     is shown in the example after this list.
//
//   - deprecated — a value-less flag rendered as x-doc-deprecated: true (any
//     value-less simple entity becomes a boolean x-doc-<entity>);
//
//   - default — rendered as x-doc-default set to the parsed YAML value (any
//     valued simple entity becomes x-doc-<entity>);
//
//   - raw:<key>=<value> — injects an arbitrary standard schema field named <key>
//     directly (a dotted <key> walks into nested schema nodes). The value is
//     required: without one the marker would write null, and the one thing its
//     author might have meant by that is what unset:<key> is for;
//
//   - unset:<key> — deletes the standard schema field named <key>, the mirror of
//     raw:<key> and the only way to take a node out rather than overwrite it.
//     controller-gen renders a description for every node it can reach, the
//     vendored ones included: items.description on a []metav1.Condition field
//     comes from the metav1.Condition godoc of whatever apimachinery the API
//     module pins, and a manifest that is not supposed to carry it cannot be
//     reproduced from the Go types while raw: is the only tool. The marker takes
//     no value, since a field set to null is not the same schema as a field that
//     is absent, and a <key> that is already missing is reported as a warning so
//     a marker that has outlived its target does not pass for a working one. A
//     <key> naming a field the structural schema requires (type, items) is
//     refused outright, and so is a removal that would leave the parent node an
//     empty mapping: obeying either would produce a manifest the apiserver
//     rejects, with nothing in the refusal pointing back at the marker. Removing
//     a validation key (required, enum, pattern, x-kubernetes-validations and the
//     rest of that vocabulary) is obeyed but reported, because the result applies
//     cleanly while the API starts admitting values it used to reject, and a
//     re-render gate cannot see it;
//
//   - sensitive-data — a schema-level flag rendered as
//     x-kubernetes-sensitive-data: true. It marks a field (or an object/array
//     subtree) as sensitive so the apiserver's CRDSensitiveData feature
//     encrypts the resource in etcd, filters the field by RBAC and masks it in
//     audit logs. It must not be placed on the root type;
//
//   - crd:<key> — a type-level entity configuring CRD-level settings
//     (preserveUnknownFields, the minimal style, schema format stripping) and
//     the curated deckhouse style. Each setting is its own "crd:<key>=<value>"
//     marker in the kubebuilder style, for example
//     "crd:preserveUnknownFields=false" or "crd:stripFormat=[int32]". CRD
//     labels and annotations are not set here; they are emitted natively by
//     controller-gen from the +kubebuilder:metadata:labels and
//     +kubebuilder:metadata:annotations markers.
//
// A named or described example renders as a wrapper object. For example, the
// markers
//
//	// +crd-enricher:deckhouse:documentation:examples={field: value}
//	// +crd-enricher:deckhouse:documentation:examples-name=My example
//	// +crd-enricher:deckhouse:documentation:examples-description=A longer note
//
// render on the field's schema node as
//
//	x-doc-examples:
//	  - x-description: A longer note
//	    x-name: My example
//	    x-example:
//	      field: value
//
// Markers may be attached both to struct fields and to the struct types
// themselves. Type-level markers are applied to the schema node of the type
// (for the root type this is openAPIV3Schema).
//
// The enricher walks the root types and the fields reachable from them, so a
// type no root reaches is never visited. A root is any type that embeds both
// metav1.TypeMeta and metav1.ObjectMeta, which is the only thing
// controller-gen's CRD generator looks at (FindKubeKinds, "locates all types
// that contain TypeMeta and ObjectMeta"). The root markers --
// +kubebuilder:object:root=true and the pre-kubebuilder
// +k8s:deepcopy-gen:interfaces naming k8s.io/apimachinery/pkg/runtime.Object --
// are read by controller-gen's deepcopy generator, not its CRD one, so they
// decide whether the type gets a DeepCopyObject method and nothing else. The
// enricher accepts them as a fallback for types that declare themselves objects
// without embedding the metav1 structs, but neither of them, and in particular
// not object:root=false, keeps a type that embeds both structs from getting a
// CRD -- and therefore from having its markers applied.
//
// The value after "=" is YAML, which means prose containing a colon followed by
// a space parses as a mapping rather than a string; such a value has to be
// quoted, and the enricher warns when it sees one that was not.
//
// # Example generation
//
// Beyond the explicit examples markers, the enricher can synthesize
// x-doc-examples from the bottom up. This synthesis is opt-in and off by
// default: it runs only when the caller passes the "auto-examples" flag
// (Options.GenerateExamples). Explicit examples markers are always applied
// regardless of the flag.
//
// When enabled, every scalar leaf yields one representative value: its first
// explicit example if present, otherwise a hard-coded fallback chosen from the
// schema default, the documented default, the first enum value, or a type-based
// placeholder (string, 0, false). Composite nodes (objects, arrays and maps)
// aggregate the values of their children into a structured example.
//
// The CRD root then receives a synthesized example carrying apiVersion, kind
// and metadata together with the aggregated spec; the status subtree is omitted.
// By default only the root is annotated; the crd:exampleScope=tree setting makes
// every object node carry its own aggregated example as well. A node that
// already has an explicit examples marker is never overwritten — explicit
// examples win over generated ones.
//
// # Contract
//
// The command in cmd/crd-enricher mirrors the controller-gen invocation used
// in the project Makefile. controller-gen is called as:
//
//	controller-gen crd paths="./..." output:crd:artifacts:config=DIR
//
// and the enricher is meant to run right after it against the same inputs:
//
//	crd-enricher paths="./..." crds=DIR
//
// The "paths" argument selects the Go packages that hold the API structs (the
// source of the markers) and "crds" points at the directory with the CRD YAML
// files produced by controller-gen, which are enriched in place.
//
// # Output layout
//
// By default the enricher writes block sequences flush with their parent key,
// matching sigs.k8s.io/yaml, so files without enriched nodes round-trip
// byte-for-byte; documents with authored (ordered) examples use goyaml.v2, which
// shares that layout. The "reindent" flag (Options.Reindent) switches to the
// goyaml.v3 layout, which indents every block sequence item under its parent key
// (SetIndent(2)). Only the indentation changes — key ordering is identical.
package crdenricher
