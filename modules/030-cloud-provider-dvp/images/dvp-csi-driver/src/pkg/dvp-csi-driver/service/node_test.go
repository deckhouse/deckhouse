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

package service

import "testing"

func TestMaxVolumesPerNode(t *testing.T) {
	for _, tc := range []struct {
		staticDisks int
		want        int64
	}{
		{staticDisks: 0, want: 16},
		{staticDisks: 1, want: 15},
		{staticDisks: 3, want: 13},
		{staticDisks: 15, want: 1},
		{staticDisks: 16, want: 1},
		{staticDisks: 20, want: 1},
	} {
		if got := maxVolumesPerNode(tc.staticDisks); got != tc.want {
			t.Errorf("maxVolumesPerNode(%d) = %d, want %d", tc.staticDisks, got, tc.want)
		}
	}
}
