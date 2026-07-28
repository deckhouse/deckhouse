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

package nodeconfig

import "testing"

func TestPickDigest(t *testing.T) {
	tests := []struct {
		name     string
		packages map[string]string
		prefix   string
		want     string
	}{
		{
			name: "two-digit patch wins over one-digit (numeric, not lexicographic)",
			packages: map[string]string{
				"kubeletSysext1356":  "sha256:patch6",
				"kubeletSysext13510": "sha256:patch10",
			},
			prefix: "kubeletSysext135",
			want:   "sha256:patch10", // 1.35.10 > 1.35.6
		},
		{
			name: "containerd two-digit patch wins",
			packages: map[string]string{
				"containerdSysext224":  "sha256:v224",
				"containerdSysext2210": "sha256:v2210",
			},
			prefix: "containerdSysext",
			want:   "sha256:v2210", // 2.2.10 > 2.2.4
		},
		{
			name:     "no matching prefix yields empty",
			packages: map[string]string{"other": "x"},
			prefix:   "kubeletSysext",
			want:     "",
		},
		{
			name:     "non-numeric suffix is ignored",
			packages: map[string]string{"kubeletSysextabc": "x", "kubeletSysext5": "sha256:v5"},
			prefix:   "kubeletSysext",
			want:     "sha256:v5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickDigest(tt.packages, tt.prefix); got != tt.want {
				t.Fatalf("pickDigest = %q, want %q", got, tt.want)
			}
		})
	}
}
