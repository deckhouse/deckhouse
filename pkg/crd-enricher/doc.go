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
// with the canonical "crd-enricher:" prefix and comes in two shapes:
//
//	+crd-enricher:raw:<key>[=<value>]                        // raw schema injection
//	+crd-enricher:deckhouse:documentation:<entity>[=<value>] // documentation entity
//	+crd-enricher:deckhouse:crd:<key>[=<value>]              // CRD-level setting
//	+crd-enricher:deckhouse:sensitive-data                   // sensitive field flag
//
// The raw entity injects a standard schema field and lives directly under the
// prefix; the documentation entities (examples, deprecated, default) carry the
// extra "deckhouse:documentation" sub-namespace; the crd entity configures the
// CRD itself and carries the shorter "deckhouse" sub-namespace. No bare or
// legacy form is recognised:
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
//   - deprecated — a value-less flag rendered as x-doc-deprecated: true (any
//     value-less simple entity becomes a boolean x-doc-<entity>);
//   - default — rendered as x-doc-default set to the parsed YAML value (any
//     valued simple entity becomes x-doc-<entity>);
//   - raw:<key> — injects an arbitrary standard schema field named <key>
//     directly (a dotted <key> walks into nested schema nodes);
//   - sensitive-data — a schema-level flag rendered as
//     x-kubernetes-sensitive-data: true. It marks a field (or an object/array
//     subtree) as sensitive so the apiserver's CRDSensitiveData feature
//     encrypts the resource in etcd, filters the field by RBAC and masks it in
//     audit logs. It must not be placed on the root type;
//   - crd:<key> — a type-level entity configuring CRD-level settings
//     (preserveUnknownFields, the minimal style, schema format stripping) and
//     the curated deckhouse style. Each setting is its own "crd:<key>=<value>"
//     marker in the kubebuilder style, for example
//     "crd:preserveUnknownFields=false" or "crd:stripFormat=[int32]". CRD
//     labels and annotations are not set here; they are emitted natively by
//     controller-gen from the +kubebuilder:metadata:labels and
//     +kubebuilder:metadata:annotations markers.
//
// Markers may be attached both to struct fields and to the struct types
// themselves. Type-level markers are applied to the schema node of the type
// (for the root type this is openAPIV3Schema).
//
// # Example generation
//
// Beyond the explicit examples markers, the enricher synthesizes x-doc-examples
// from the bottom up. Every scalar leaf yields one representative value: its
// first explicit example if present, otherwise a hard-coded fallback chosen from
// the schema default, the documented default, the first enum value, or a
// type-based placeholder (string, 0, false). Composite nodes (objects, arrays
// and maps) aggregate the values of their children into a structured example.
//
// The CRD root always receives a synthesized example carrying apiVersion, kind
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
package crdenricher
