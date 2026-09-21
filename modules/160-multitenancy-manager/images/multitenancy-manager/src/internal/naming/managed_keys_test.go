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

package naming

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// policyLists reads every occurrence of a CEL list literal variable from the admission policy
// template (both policies carry a copy):
//
//   - name: <variable>
//     expression: '["a", "b"]'
func policyLists(t *testing.T, policy, variable string) [][]string {
	t.Helper()
	re := regexp.MustCompile(`(?s)- name: '?` + regexp.QuoteMeta(variable) + `'?\n\s+expression: '(\[[^']*\])'`)
	matches := re.FindAllStringSubmatch(policy, -1)
	if len(matches) == 0 {
		t.Fatalf("variable %q not found in the policy", variable)
	}
	item := regexp.MustCompile(`"([^"]+)"`)
	out := make([][]string, 0, len(matches))
	for _, m := range matches {
		var list []string
		for _, it := range item.FindAllStringSubmatch(m[1], -1) {
			list = append(list, it[1])
		}
		slices.Sort(list)
		out = append(out, list)
	}
	return out
}

// TestManagedNamespaceKeysMatchThePolicy: the admission policies enforce the module-owned keys as
// CEL literals (one copy per policy) and the controller filters them in Go; every copy must be the
// same list as the Go one.
func TestManagedNamespaceKeysMatchThePolicy(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "templates", "validation.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		// The only job of this test is to fail when the copies drift; skipping itself when it cannot
		// read one of them is the same as not having it.
		t.Fatalf("read the admission policy template at %s: %v", path, err)
	}
	policy := string(raw)

	wantLabels := slices.Clone(ManagedNamespaceLabels)
	wantAnnotations := slices.Clone(ManagedNamespaceAnnotations)
	slices.Sort(wantLabels)
	slices.Sort(wantAnnotations)

	labelCopies := policyLists(t, policy, "managedLabels")
	if len(labelCopies) != 2 {
		t.Fatalf("expected managedLabels in both policies, found %d copies", len(labelCopies))
	}
	for i, labels := range labelCopies {
		if !slices.Equal(labels, wantLabels) {
			t.Fatalf("managed labels differ (copy %d):\n policy: %v\n go:     %v", i, labels, wantLabels)
		}
	}
	for i, annotations := range policyLists(t, policy, "managedAnnotations") {
		if !slices.Equal(annotations, wantAnnotations) {
			t.Fatalf("managed annotations differ (copy %d):\n policy: %v\n go:     %v", i, annotations, wantAnnotations)
		}
	}
}
