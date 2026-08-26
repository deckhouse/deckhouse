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
	"go/token"
	"go/types"
	"testing"
)

// The types here are synthesized rather than loaded from a fixture package on
// purpose: the rule under test is keyed on the *import path* of the embedded
// structs, and a fixture would have to pull k8s.io/apimachinery into this tool
// module's go.mod (or add a stub module under testdata, of which this repository
// has none) just to spell that path. Building the types directly pins the rule
// exactly -- embedded, right package, both names -- with nothing in between.

// metaStruct builds a stand-in for one of the metav1 structs: only its name and
// package path matter to embedsObjectMeta.
func metaStruct(pkg *types.Package, name string) *types.Named {
	obj := types.NewTypeName(token.NoPos, pkg, name, nil)
	return types.NewNamed(obj, types.NewStruct(nil, nil), nil)
}

// objectLike builds a named struct type embedding (or merely holding) the given
// metav1 stand-ins, the way a Kubernetes API type does.
func objectLike(name string, embedded bool, metas ...*types.Named) *types.Named {
	apiPkg := types.NewPackage("example.io/api/v1alpha1", "v1alpha1")

	fields := make([]*types.Var, 0, len(metas))
	tags := make([]string, 0, len(metas))
	for _, meta := range metas {
		fields = append(fields, types.NewField(token.NoPos, apiPkg, meta.Obj().Name(), meta, embedded))
		tags = append(tags, `json:",inline"`)
	}

	obj := types.NewTypeName(token.NoPos, apiPkg, name, nil)
	return types.NewNamed(obj, types.NewStruct(fields, tags), nil)
}

// TestEmbedsObjectMeta pins the rule controller-gen's CRD generator applies:
// FindKubeKinds selects a type by the metav1 structs it embeds, and nothing else.
func TestEmbedsObjectMeta(t *testing.T) {
	metav1Pkg := types.NewPackage(metav1PkgPath, "v1")
	vendored := types.NewPackage("example.io/mod/vendor/"+metav1PkgPath, "v1")
	lookalike := types.NewPackage("example.io/api/meta/v1", "v1")

	typeMeta := metaStruct(metav1Pkg, "TypeMeta")
	objectMeta := metaStruct(metav1Pkg, "ObjectMeta")

	notAStruct := types.NewNamed(
		types.NewTypeName(token.NoPos, metav1Pkg, "Duration", nil), types.Typ[types.String], nil)

	for _, tc := range []struct {
		name string
		typ  *types.Named
		want bool
		why  string
	}{
		{
			name: "both structs embedded",
			typ:  objectLike("Thing", true, typeMeta, objectMeta),
			want: true,
			why:  "this is exactly what FindKubeKinds looks for",
		},
		{
			name: "only TypeMeta embedded",
			typ:  objectLike("ThingList", true, typeMeta),
			want: false,
			why:  "a List embeds TypeMeta and ListMeta, and gets no CRD of its own",
		},
		{
			name: "only ObjectMeta embedded",
			typ:  objectLike("Partial", true, objectMeta),
			want: false,
			why:  "both are required, as in FindKubeKinds",
		},
		{
			name: "both present but as named fields",
			typ:  objectLike("Wrapper", false, typeMeta, objectMeta),
			want: false,
			why:  "FindKubeKinds skips fields that carry a name",
		},
		{
			name: "vendored apimachinery",
			typ: objectLike("Vendored", true,
				metaStruct(vendored, "TypeMeta"), metaStruct(vendored, "ObjectMeta")),
			want: true,
			why:  "a vendored path names the same structs",
		},
		{
			name: "same names from another package",
			typ: objectLike("Impostor", true,
				metaStruct(lookalike, "TypeMeta"), metaStruct(lookalike, "ObjectMeta")),
			want: false,
			why:  "controller-gen compares the metav1 package, not the type names",
		},
		{
			name: "not a struct at all",
			typ:  notAStruct,
			want: false,
			why:  "a named scalar has no fields to embed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := embedsObjectMeta(tc.typ); got != tc.want {
				t.Errorf("embedsObjectMeta() = %v, want %v: %s", got, tc.want, tc.why)
			}
		})
	}
}

// TestIsCRDRoot pins the regression that made this rule necessary: a type that
// embeds the metav1 structs gets a CRD from controller-gen whatever its root
// marker says, so its crd-enricher markers have a manifest to land in. Keying the
// root set on the marker alone dropped every one of them without a word.
func TestIsCRDRoot(t *testing.T) {
	metav1Pkg := types.NewPackage(metav1PkgPath, "v1")
	object := objectLike("Thing", true,
		metaStruct(metav1Pkg, "TypeMeta"), metaStruct(metav1Pkg, "ObjectMeta"))
	plain := objectLike("Plain", true)

	for _, tc := range []struct {
		name    string
		markers []marker
		typ     *types.Named
		want    bool
		why     string
	}{
		{
			name:    "object:root=false does not opt out of CRD generation",
			markers: []marker{{name: rootMarker, rawValue: "false", hasValue: true}},
			typ:     object,
			want:    true,
			why:     "controller-gen renders the CRD regardless; the markers must be applied",
		},
		{
			name:    "no root marker at all",
			markers: nil,
			typ:     object,
			want:    true,
			why:     "FindKubeKinds needs no marker, and neither may the enricher",
		},
		{
			name:    "the kubebuilder marker without the embedding",
			markers: []marker{{name: rootMarker}},
			typ:     plain,
			want:    true,
			why:     "the marker stays a fallback for types that declare themselves objects",
		},
		{
			name:    "the legacy marker without the embedding",
			markers: []marker{{name: legacyRootMarker, rawValue: runtimeObject, hasValue: true}},
			typ:     plain,
			want:    true,
			why:     "same fallback, pre-kubebuilder spelling",
		},
		{
			name:    "neither the embedding nor a marker",
			markers: []marker{{name: "k8s:deepcopy-gen", rawValue: "true", hasValue: true}},
			typ:     plain,
			want:    false,
			why:     "nothing here gets a CRD",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCRDRoot(tc.markers, tc.typ); got != tc.want {
				t.Errorf("isCRDRoot() = %v, want %v: %s", got, tc.want, tc.why)
			}
		})
	}
}
