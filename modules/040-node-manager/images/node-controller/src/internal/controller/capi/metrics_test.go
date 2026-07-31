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

func TestPhaseToFloat(t *testing.T) {
	cases := []struct {
		phase string
		want  float64
	}{
		{"Running", 1},
		{"ScalingUp", 2},
		{"ScalingDown", 3},
		{"Failed", 4},
		{"Unknown", 5},
		{"", 5},
		{"Something", 5},
	}
	for _, c := range cases {
		if got := phaseToFloat(c.phase); got != c.want {
			t.Fatalf("phaseToFloat(%q)=%v, want %v", c.phase, got, c.want)
		}
	}
}
