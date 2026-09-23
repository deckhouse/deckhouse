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

package storage_class

import "testing"

// Cloud-side names are free-form — a zVirt storage domain or a DVP storage class may contain
// spaces, capitals and punctuation Kubernetes will not accept in an object name.
func TestNormalizeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "keeps a valid name", value: "replicated", want: "replicated"},
		{name: "normalizes spaces and case", value: "Excluded Fast", want: "excluded-fast"},
		{name: "removes invalid symbols and trims the ends", value: "-Xx__$()? -foo-", want: "xx--foo"},
		{name: "trims dots and dashes", value: ".. YY fast SSD-foo.-", want: "yy-fast-ssd-foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeStorageClassName(tt.value); got != tt.want {
				t.Errorf("NormalizeName(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
