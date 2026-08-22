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

import "testing"

// TestLegacyRootGolden captures the pre-kubebuilder root marker: a type that
// only declares the runtime.Object deepcopy interface is a CRD root for
// controller-gen, so its crd-enricher markers -- the CRD-level setting and the
// field example alike -- have to be applied rather than dropped.
func TestLegacyRootGolden(t *testing.T) {
	assertGolden(t, "legacyroot.yaml", runFixture(t, "legacyroot.yaml", false))
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
