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

// Package engine holds the pure decision helpers shared by the webhooks and the reconciler: rule
// matching, version-scoped field-path selection and match-guard evaluation. The availability decision
// (excluded → denied → allowed → baseline) and the per-project default live in internal/resolve.
package engine

import (
	"fmt"
	"slices"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

// listMatches reports whether want is in list, treating "*" as a wildcard.
func listMatches(list []string, want string) bool {
	for _, x := range list {
		if x == "*" || x == want {
			return true
		}
	}
	return false
}

// RuleMatches reports whether the (group, version, resource) of a usage object matches the rule.
// The core group is the empty string; "*" in any dimension matches anything.
func RuleMatches(rule v1alpha1.UsageRule, group, version, resource string) bool {
	return listMatches(rule.APIGroups, group) &&
		listMatches(rule.APIVersions, version) &&
		listMatches(rule.Resources, resource)
}

// SelectFieldPath returns the FieldPath entry that applies to the given group/version: a scoped entry
// (apiGroups/apiVersions set) wins over an unscoped one; the unscoped entry is the fallback. The bool
// is false when no entry matches.
func SelectFieldPath(fieldPaths []v1alpha1.FieldPath, group, version string) (v1alpha1.FieldPath, bool) {
	var fallback *v1alpha1.FieldPath
	for i := range fieldPaths {
		fp := &fieldPaths[i]
		groupOK := len(fp.APIGroups) == 0 || listMatches(fp.APIGroups, group)
		versionOK := len(fp.APIVersions) == 0 || listMatches(fp.APIVersions, version)
		if !groupOK || !versionOK {
			continue
		}
		if len(fp.APIGroups) > 0 || len(fp.APIVersions) > 0 {
			return *fp, true // scoped entry wins
		}
		if fallback == nil {
			fallback = fp
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return v1alpha1.FieldPath{}, false
}

// StringValuesAt returns the string-typed values selected by the JSONPath expression.
func StringValuesAt(factory jsonpath.Factory, obj map[string]any, path string) ([]string, error) {
	parsed, err := factory.Path(path)
	if err != nil {
		return nil, fmt.Errorf("parse jsonpath %q: %w", path, err)
	}
	nodes := parsed.Select(obj)
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s, ok := n.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// EvalMatch reports whether a field path's match guard holds for the object. A nil predicate always
// holds. A predicate with neither equals nor in holds when the field is present.
func EvalMatch(factory jsonpath.Factory, pred *v1alpha1.MatchPredicate, obj map[string]any) (bool, error) {
	if pred == nil {
		return true, nil
	}
	vals, err := StringValuesAt(factory, obj, pred.FieldPath)
	if err != nil {
		return false, err
	}
	if pred.Equals == "" && len(pred.In) == 0 {
		return len(vals) > 0, nil
	}
	for _, v := range vals {
		if pred.Equals != "" && v == pred.Equals {
			return true, nil
		}
		if len(pred.In) > 0 && slices.Contains(pred.In, v) {
			return true, nil
		}
	}
	return false, nil
}
