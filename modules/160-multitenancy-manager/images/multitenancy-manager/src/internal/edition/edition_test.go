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

package edition

import "testing"

func TestCurrent(t *testing.T) {
	saved := built
	t.Cleanup(func() { built = saved })

	for _, tc := range []struct {
		name     string
		built    string
		expected string
	}{
		{name: "a build without an edition", built: "", expected: Unknown},
		{name: "a CE build", built: "CE", expected: CE},
		{name: "an EE build", built: "EE", expected: "EE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			built = tc.built
			if got := Current(); got != tc.expected {
				t.Fatalf("Current() = %q, expected %q", got, tc.expected)
			}
		})
	}
}

func TestSingleNamespaceProjects(t *testing.T) {
	t.Parallel()

	// The edition names are the ones of editions.yaml of the repository, which werf builds with.
	for _, tc := range []struct {
		edition  string
		expected bool
	}{
		{edition: "CE", expected: true},
		{edition: "BE", expected: false},
		{edition: "SE", expected: false},
		{edition: "SE-plus", expected: false},
		{edition: "EE", expected: false},
		{edition: "FE", expected: false},
		{edition: "CSE", expected: false},
		{edition: Unknown, expected: false},
		{edition: "ce", expected: false},
	} {
		t.Run(tc.edition, func(t *testing.T) {
			t.Parallel()
			if got := SingleNamespaceProjects(tc.edition); got != tc.expected {
				t.Fatalf("SingleNamespaceProjects(%q) = %t, expected %t", tc.edition, got, tc.expected)
			}
		})
	}
}
