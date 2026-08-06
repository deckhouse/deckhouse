//go:build validation
// +build validation

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

// Reading every Secret of a namespace and executing inside its containers are the two capabilities
// that turn access to a namespace into control over whatever runs there, so the level they aggregate
// into is a security decision. That decision lives in a single label, it reaches the project lineage
// indirectly (through the aggregate-to-project-as label the namespace roles carry), and lowering it
// would widen five roles at once without touching any rule. This test resolves the aggregation the way
// the API server does and pins the roles that end up holding both capabilities.
package rbacv2

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRBACv2SensitiveCapabilityLevelsValidation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	objects := parseRBACv2Objects(t, root)

	for _, tc := range []struct {
		marker   string
		expected map[string][]string
	}{
		{
			marker: "namespace-capability.kubernetes.view_secrets",
			expected: map[string][]string{
				"d8:namespace:": {"d8:namespace:admin", "d8:namespace:manager", "d8:namespace:superadmin"},
				"d8:project:":   {"d8:project:admin", "d8:project:manager", "d8:project:superadmin"},
			},
		},
		{
			marker: "namespace-capability.kubernetes.access_terminal",
			expected: map[string][]string{
				"d8:namespace:": {"d8:namespace:admin", "d8:namespace:manager", "d8:namespace:superadmin"},
				"d8:project:":   {"d8:project:admin", "d8:project:manager", "d8:project:superadmin"},
			},
		},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			capability := capabilityByMarker(t, objects, tc.marker)
			if level := capability.labels[labelPrefix+"aggregate-to-namespace-as"]; level != "manager" {
				t.Errorf("capability %q must aggregate into the namespace lineage as %q, got %q", capability.name, "manager", level)
			}

			holders := rolesHolding(t, objects, capability.name)
			for prefix, want := range tc.expected {
				got := filterByPrefix(holders, prefix)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("capability %q is held by %v in the %s lineage, want %v", capability.name, got, strings.TrimSuffix(prefix, ":"), want)
				}
			}
		})
	}
}

func parseRBACv2Objects(t *testing.T, root string) []*clusterRoleFile {
	t.Helper()

	var objects []*clusterRoleFile
	for _, path := range collectRBACv2Files(t, root) {
		obj, err := parseFile(path)
		if err != nil {
			t.Fatalf("%s: %v", rel(root, path), err)
		}
		objects = append(objects, obj)
	}
	return objects
}

func capabilityByMarker(t *testing.T, objects []*clusterRoleFile, marker string) *clusterRoleFile {
	t.Helper()

	for _, obj := range objects {
		if obj.labels[labelPrefix+"capability"] == marker {
			return obj
		}
	}
	t.Fatalf("no capability carries the %scapability marker %q", labelPrefix, marker)
	return nil
}

// rolesHolding returns the sorted names of the roles that end up aggregating the rules of the named
// ClusterRole. A role picks up everything whose labels match one of its selectors, and the roles chain
// into each other, so the answer is the fixed point of that step rather than a single lookup.
func rolesHolding(t *testing.T, objects []*clusterRoleFile, name string) []string {
	t.Helper()

	holders := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, role := range objects {
			if role.aggregation == nil || holders[role.name] {
				continue
			}
			for _, selector := range role.aggregation.ClusterRoleSelectors {
				// The resolver below only implements matchLabels; a matchExpressions selector
				// would silently resolve to less than the API server grants.
				if len(selector.MatchExpressions) > 0 {
					t.Fatalf("role %q uses an aggregation selector with matchExpressions, which this test cannot resolve", role.name)
				}
				for _, obj := range objects {
					if obj.name != name && !holders[obj.name] {
						continue
					}
					if selectorMatches(selector.MatchLabels, obj.labels) {
						holders[role.name] = true
						changed = true
						break
					}
				}
			}
		}
	}

	names := make([]string, 0, len(holders))
	for holder := range holders {
		names = append(names, holder)
	}
	sort.Strings(names)
	return names
}

func selectorMatches(selector, labels map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func filterByPrefix(names []string, prefix string) []string {
	out := []string{}
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out
}
