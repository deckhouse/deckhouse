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
	"testing"

	"sigs.k8s.io/yaml"
)

// TestLegacyRootGolden captures the marker fallback: a type declaring only the
// pre-kubebuilder runtime.Object deepcopy interface is still treated as a root,
// so its crd-enricher markers are applied rather than dropped.
//
// The golden pins what was actually lost in the real case, not just that
// something was applied: crd:minimal has to strip the listKind, the implicit
// apiVersion/kind/metadata root properties and the generator-version annotation,
// because all of them would otherwise reach a module's public API reference page.
func TestLegacyRootGolden(t *testing.T) {
	got := runFixture(t, "legacyroot.yaml", false)
	assertGolden(t, "legacyroot.yaml", got)

	var crd map[string]any
	if err := yaml.Unmarshal(got, &crd); err != nil {
		t.Fatalf("parse enriched manifest: %v", err)
	}

	spec := childMap(crd, "spec")
	if names := childMap(spec, "names"); names != nil {
		if _, ok := names["listKind"]; ok {
			t.Error("crd:minimal left names.listKind in the manifest")
		}
	}
	if annotations := childMap(childMap(crd, "metadata"), "annotations"); annotations != nil {
		if _, ok := annotations["controller-gen.kubebuilder.io/version"]; ok {
			t.Error("crd:minimal left the generator-version annotation in the manifest")
		}
	}
	if pres, ok := spec["preserveUnknownFields"]; !ok || pres != false {
		t.Errorf("spec.preserveUnknownFields = %#v, want false", pres)
	}

	versions, _ := spec["versions"].([]any)
	if len(versions) != 1 {
		t.Fatalf("versions = %#v, want exactly one", versions)
	}
	version, _ := versions[0].(map[string]any)
	root := childMap(childMap(version, "schema"), "openAPIV3Schema")
	props := childMap(root, "properties")
	for _, implicit := range []string{"apiVersion", "kind", "metadata"} {
		if _, ok := props[implicit]; ok {
			t.Errorf("crd:minimal left the implicit root property %q in the schema", implicit)
		}
	}
	// The field marker has to have landed too: the CRD-level half working on its
	// own would not prove the type entered the root set the walk descends from.
	name := childMap(childMap(props, "spec"), "properties")
	if _, ok := childMap(name, "name")["x-doc-examples"]; !ok {
		t.Error("the field example marker was not applied")
	}
}

// TestIsRootObject pins the two spellings controller-gen accepts, and the two
// that look close enough to be mistaken for them.
func TestIsRootObject(t *testing.T) {
	for _, tc := range []struct {
		name    string
		markers []marker
		want    bool
	}{
		{
			name:    "kubebuilder marker without a value",
			markers: []marker{{name: rootMarker}},
			want:    true,
		},
		{
			name:    "kubebuilder marker set to true",
			markers: []marker{{name: rootMarker, rawValue: "true", hasValue: true}},
			want:    true,
		},
		{
			name:    "kubebuilder marker set to false opts out",
			markers: []marker{{name: rootMarker, rawValue: "false", hasValue: true}},
			want:    false,
		},
		{
			name:    "legacy marker naming runtime.Object",
			markers: []marker{{name: legacyRootMarker, rawValue: runtimeObject, hasValue: true}},
			want:    true,
		},
		{
			name:    "legacy marker naming another interface",
			markers: []marker{{name: legacyRootMarker, rawValue: "example.io/api.Copier", hasValue: true}},
			want:    false,
		},
		{
			name:    "deepcopy-gen alone is not a root",
			markers: []marker{{name: "k8s:deepcopy-gen", rawValue: "true", hasValue: true}},
			want:    false,
		},
		{
			name:    "no markers at all",
			markers: nil,
			want:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRootObject(tc.markers); got != tc.want {
				t.Errorf("isRootObject() = %v, want %v", got, tc.want)
			}
		})
	}
}
