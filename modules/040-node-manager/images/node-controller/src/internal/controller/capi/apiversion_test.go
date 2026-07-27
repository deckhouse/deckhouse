/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package capi

import "testing"

func TestIsClusterRequest(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"cluster:static", true},
		{"cluster:yandex", true},
		{"cluster:", false}, // prefix only, len not > 8
		{"cluster", false},  // no colon
		{"md-worker-abc", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isClusterRequest(c.name); got != c.want {
			t.Fatalf("isClusterRequest(%q)=%v, want %v", c.name, got, c.want)
		}
	}
}
