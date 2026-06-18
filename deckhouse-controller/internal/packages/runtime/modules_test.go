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

package runtime

import "testing"

func TestIsEmbeddedPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "embedded module", path: "/deckhouse/modules/002-deckhouse", want: true},
		{name: "embedded module no weight", path: "/deckhouse/modules/deckhouse", want: true},
		{name: "downloaded module symlink", path: "/deckhouse/downloaded/modules/cni-cilium", want: false},
		{name: "downloaded module versioned", path: "/deckhouse/downloaded/cni-cilium/v1.2.3", want: false},
		{name: "unrelated path", path: "/tmp/whatever", want: false},
		{name: "empty path", path: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEmbeddedPath(tc.path); got != tc.want {
				t.Fatalf("isEmbeddedPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
