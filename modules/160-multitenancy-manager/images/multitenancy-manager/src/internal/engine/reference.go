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

package engine

import (
	"fmt"
	"slices"
	"strings"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

// This file holds the validity rules of a GrantableClusterResourceReference. They are shared by the
// GrantableClusterResourceReference validating webhook, which applies them with ratcheting on UPDATE,
// and by the reference reconciler, which applies all of them to the stored object and reports the
// result as the FieldPathsValid condition. Both print the same problem strings.

// ReferenceProblems returns every problem of a reference spec, empty when it is valid: per entry the
// path problems, then the scope problems, and the coverage problem last.
func ReferenceProblems(factory jsonpath.Factory, spec v1alpha1.GrantableClusterResourceReferenceSpec) []string {
	var problems []string
	for i, fp := range spec.FieldPaths {
		problems = append(problems, EntryPathProblems(factory, i, fp)...)
		problems = append(problems, EntryScopeProblems(spec.Rule, i, fp)...)
	}
	return append(problems, CoverageProblems(spec.Rule, spec.FieldPaths)...)
}

// EntryPathProblems reports the paths of one spec.fieldPaths entry that /is-granted cannot compile,
// and a defaulting path /defaults cannot write to.
func EntryPathProblems(factory jsonpath.Factory, i int, fp v1alpha1.FieldPath) []string {
	var problems []string
	pathCompiles := true
	if _, err := factory.Path(fp.Path); err != nil {
		pathCompiles = false
		problems = append(problems, fmt.Sprintf(
			"'spec.fieldPaths[%d].path' %q is not a valid RFC 9535 JSONPath: %v", i, fp.Path, err))
	}
	if fp.Match != nil {
		if _, err := factory.Path(fp.Match.FieldPath); err != nil {
			problems = append(problems, fmt.Sprintf(
				"'spec.fieldPaths[%d].match.fieldPath' %q is not a valid RFC 9535 JSONPath: %v", i, fp.Match.FieldPath, err))
		}
	}
	// Exactly the condition /defaults acts on, so this never rejects an entry the mutator would have
	// skipped anyway. A path that does not compile is already reported above.
	if pathCompiles && DefaultingActive(fp) {
		if _, ok := ParsePathSegments(factory, fp.Path); !ok {
			problems = append(problems, fmt.Sprintf(
				"'spec.fieldPaths[%d]' has path %q, which cannot be used with defaulting '%s': "+
					"defaulting writes a single field, so it supports simple member paths only "+
					"(for example '$.spec.storageClassName' or \"$.metadata.annotations['cert-manager.io/cluster-issuer']\") "+
					"and accepts no wildcards, array indexes or filters. "+
					"Set 'defaulting: None' to keep this path validated without defaulting, or point it at a single field",
				i, fp.Path, fp.Defaulting))
		}
	}
	return problems
}

// EntryScopeProblems reports scope values of one entry that spec.rule never admits. Such a value is
// almost always a typo (pod, Pods) and scopes the entry to requests that never arrive. "*" in the entry
// restricts nothing and "*" in the rule admits any value, so neither is a problem.
func EntryScopeProblems(rule v1alpha1.UsageRule, i int, fp v1alpha1.FieldPath) []string {
	var problems []string
	for _, d := range []struct {
		name        string
		entry, rule []string
	}{
		{"apiGroups", fp.APIGroups, rule.APIGroups},
		{"apiVersions", fp.APIVersions, rule.APIVersions},
		{"resources", fp.Resources, rule.Resources},
	} {
		if slices.Contains(d.rule, "*") {
			continue
		}
		for _, value := range d.entry {
			if value == "*" || slices.Contains(d.rule, value) {
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"'spec.fieldPaths[%d].%s' lists %q, which 'spec.rule.%s' does not include (allowed: %s)",
				i, d.name, value, d.name, quoteAll(d.rule)))
		}
	}
	return problems
}

// anyOther stands for "a value the rule admits through '*'" in the coverage check. It is not a valid
// group, version or resource name, so it appears in no entry's explicit list: an entry selects it only
// when that dimension is omitted or lists "*", which is exactly what covers every value of a "*".
const anyOther = "<any other>"

// CoverageProblems reports the (group, version, resource) combinations spec.rule admits for which
// SelectFieldPath finds no entry. /is-granted, /defaults and the violation scan skip such a request
// without a check, so a hole here is a silent fail-open. The rule's lists are finite ("*" is replaced
// by anyOther; the CRD rejects "*" in rule.apiGroups, but a code-built object is handled the same way),
// so the check enumerates them.
func CoverageProblems(rule v1alpha1.UsageRule, fieldPaths []v1alpha1.FieldPath) []string {
	var uncovered []string
	for _, group := range dimension(rule.APIGroups) {
		for _, version := range dimension(rule.APIVersions) {
			for _, resource := range dimension(rule.Resources) {
				if _, ok := SelectFieldPath(fieldPaths, group, version, resource); !ok {
					uncovered = append(uncovered, fmt.Sprintf("(apiGroup %s, apiVersion %s, resource %s)",
						showValue(group), showValue(version), showValue(resource)))
				}
			}
		}
	}
	if len(uncovered) == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"no 'spec.fieldPaths' entry applies to %s, which 'spec.rule' matches, so such requests would not be checked at all: "+
			"add entries scoped to them, or an entry without 'resources', 'apiGroups' and 'apiVersions' as the fallback",
		strings.Join(uncovered, ", "))}
}

// dimension returns the distinct values of a rule dimension, with "*" replaced by anyOther.
func dimension(list []string) []string {
	var out []string
	for _, v := range list {
		if v == "*" {
			v = anyOther
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func showValue(v string) string {
	if v == anyOther {
		return anyOther
	}
	return fmt.Sprintf("%q", v)
}

func quoteAll(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	quoted := make([]string, len(list))
	for i, v := range list {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(quoted, ", ")
}
