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

package main

import "testing"

func TestToSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "6.1.0+deb13+1-cloud-amd64",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0-custom-build+abc",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0-38-amd64",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0beta1",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0-rc1",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0~rc1",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0",
			expected: "6.1.0",
		},
		{
			input:    "6.1",
			expected: "6.1.0",
		},
		{
			input:    "6",
			expected: "6.0.0",
		},
		{
			input:    "6.",
			expected: "6.0.0",
		},
		{
			input:    ".",
			expected: "0.0.0",
		},
		{
			input:    "",
			expected: "0.0.0",
		},
		{
			input:    "unknown",
			expected: "0.0.0",
		},
		{
			input:    "6.1.0-very-long-suffix-with-many-parts-and-build-metadata-xyz123",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0+20240101+git+sha+dirty",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0_beta",
			expected: "6.1.0",
		},
		{
			input:    "6.1.0.beta",
			expected: "6.1.0",
		},
		{
			input:    "1.2.3extra",
			expected: "1.2.3",
		},
		{
			input:    "01.2.3",
			expected: "1.2.3",
		},
		{
			input:    "000001.2.3",
			expected: "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSemver(tt.input)
			if got != tt.expected {
				t.Fatalf("toSemver(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
