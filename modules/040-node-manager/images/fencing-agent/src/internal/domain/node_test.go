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

package domain

import "testing"

func TestInNodeGroup(t *testing.T) {
	const nodeGroup = "worker"

	tests := []struct {
		label string
		want  bool
	}{
		{label: "worker", want: true},
		{label: "", want: false},
		{label: "worker-2", want: false},
		{label: "Worker", want: false},
		{label: " worker", want: false},
		{label: "worker ", want: false},
	}

	for _, tt := range tests {
		if got := InNodeGroup(tt.label, nodeGroup); got != tt.want {
			t.Errorf("InNodeGroup(%q, %q) = %v, want %v", tt.label, nodeGroup, got, tt.want)
		}
	}
}
