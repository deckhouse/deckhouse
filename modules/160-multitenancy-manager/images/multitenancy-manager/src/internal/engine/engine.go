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
// (excluded → denied → allowed → reserved → baseline) and the per-project default live in
// internal/resolve.
package engine

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/theory/jsonpath/spec"

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

// Specificity weights of the FieldPath scope dimensions. They are powers of two, so a resource scope
// alone outranks a group and a version scope together.
const (
	resourcesWeight   = 4
	apiGroupsWeight   = 2
	apiVersionsWeight = 1
)

// scopeWeight returns weight when the dimension actually narrows the entry: the list is non-empty and
// has no "*". An empty list or a wildcard constrains nothing, so it adds no specificity.
func scopeWeight(list []string, weight int) int {
	if len(list) == 0 || slices.Contains(list, "*") {
		return 0
	}
	return weight
}

// SelectFieldPath returns the FieldPath entry that applies to the given resource/group/version. An
// empty dimension on an entry matches anything, so the unscoped entry is the fallback. Among the
// matching entries the most specific one wins: Resources outranks APIGroups, which outranks
// APIVersions, and the weights add up. A dimension listing "*" matches anything and counts as
// unrestricted, so it adds no weight. Ties go to the earliest entry in the list, so the choice is
// deterministic. The bool is false when no entry matches. Only the scope is looked at; the entry that
// applies to an object is chosen by ApplicableFieldPathIndex, which also evaluates the match guards.
//
// The resource scope is what separates paths that collide within one group and version, e.g. core/v1
// pods ($.spec.priorityClassName) versus replicationcontrollers
// ($.spec.template.spec.priorityClassName).
func SelectFieldPath(fieldPaths []v1alpha1.FieldPath, group, version, resource string) (v1alpha1.FieldPath, bool) {
	i := SelectFieldPathIndex(fieldPaths, group, version, resource)
	if i < 0 {
		return v1alpha1.FieldPath{}, false
	}
	return fieldPaths[i], true
}

// SelectFieldPathIndex is SelectFieldPath returning the index of the selected entry, -1 when none
// matches, for callers that report which entry they used.
func SelectFieldPathIndex(fieldPaths []v1alpha1.FieldPath, group, version, resource string) int {
	best, bestWeight := -1, -1
	for i := range fieldPaths {
		fp := &fieldPaths[i]
		if !ScopeMatches(fp, group, version, resource) {
			continue
		}
		if weight := fieldPathScopeWeight(fp); weight > bestWeight {
			best, bestWeight = i, weight
		}
	}
	return best
}

// ApplicableFieldPathIndex returns the index of the FieldPath entry that applies to the object: the
// most specific of the entries whose scope matches the resource/group/version and whose match guard
// holds on the object, -1 when there is none. The scope ranks as in SelectFieldPath; at equal scope an
// entry with a match guard outranks one without it, and ties go to the earliest entry. So an unscoped,
// unguarded entry is the fallback for the objects the guarded entries of the same scope do not match.
//
// The guards are evaluated from the most specific entry down, and the first guard that cannot be
// evaluated stops the selection: the error is returned with the index of its entry.
func ApplicableFieldPathIndex(factory jsonpath.Factory, fieldPaths []v1alpha1.FieldPath, group, version, resource string, obj map[string]any) (int, error) {
	order := make([]int, 0, len(fieldPaths))
	for i := range fieldPaths {
		if ScopeMatches(&fieldPaths[i], group, version, resource) {
			order = append(order, i)
		}
	}
	weight := func(i int) int {
		w := 2 * fieldPathScopeWeight(&fieldPaths[i])
		if fieldPaths[i].Match != nil {
			w++
		}
		return w
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(weight(b), weight(a)) })
	for _, i := range order {
		ok, err := EvalMatch(factory, fieldPaths[i].Match, obj)
		if err != nil {
			return i, err
		}
		if ok {
			return i, nil
		}
	}
	return -1, nil
}

// ScopeMatches reports whether the scope of a FieldPath entry admits the resource/group/version.
func ScopeMatches(fp *v1alpha1.FieldPath, group, version, resource string) bool {
	return (len(fp.Resources) == 0 || listMatches(fp.Resources, resource)) &&
		(len(fp.APIGroups) == 0 || listMatches(fp.APIGroups, group)) &&
		(len(fp.APIVersions) == 0 || listMatches(fp.APIVersions, version))
}

func fieldPathScopeWeight(fp *v1alpha1.FieldPath) int {
	return scopeWeight(fp.Resources, resourcesWeight) +
		scopeWeight(fp.APIGroups, apiGroupsWeight) +
		scopeWeight(fp.APIVersions, apiVersionsWeight)
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

// DefaultingActive reports whether a field path asks for defaulting at all. The CRD defaults the
// field to None, but an object built in code may leave it empty, so both mean "validate only".
// The mutating webhook and the GrantableClusterResourceReference validator must agree on this, or
// the validator would reject entries the mutator silently ignores.
func DefaultingActive(fp v1alpha1.FieldPath) bool {
	return fp.Defaulting != "" && fp.Defaulting != v1alpha1.DefaultingNone
}

// ParsePathSegments returns the member names of a path that selects exactly one field by name only,
// e.g. $.spec.storageClassName or $.metadata.annotations['cert-manager.io/cluster-issuer']. It returns
// ok=false for anything else: a path the RFC 9535 parser rejects, wildcards, indexes, slices, filters,
// descendant segments, several selectors in one segment, the bare root, and an empty member name
// (which would patch an empty root key).
//
// The path is parsed by the same factory /is-granted evaluates it with, and the segments are read off
// that parse tree, so escapes are decoded exactly as the evaluator decodes them and any path accepted
// here is also accepted there. This is the only parser the defaulting path may use: /defaults writes
// a JSON Patch at the returned location, and the GrantableClusterResourceReference validating webhook
// rejects a defaulting entry whose path this function refuses.
func ParsePathSegments(factory jsonpath.Factory, expr string) ([]string, bool) {
	parsed, err := factory.Path(expr)
	if err != nil {
		return nil, false
	}
	query := parsed.Query().Segments()
	if len(query) == 0 {
		return nil, false
	}
	segs := make([]string, 0, len(query))
	for _, seg := range query {
		selectors := seg.Selectors()
		if seg.IsDescendant() || len(selectors) != 1 {
			return nil, false
		}
		name, ok := selectors[0].(spec.Name)
		if !ok || name == "" {
			return nil, false
		}
		segs = append(segs, string(name))
	}
	return segs, true
}
