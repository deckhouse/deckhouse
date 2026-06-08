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

// Package engine holds the pure decision logic shared by the webhooks and the reconciler:
// rule matching, per-version path selection, match-guard evaluation, effective availability
// precedence and quota measures. It has no Kubernetes client dependency so it is trivially
// unit-testable.
package engine

import (
	"fmt"
	"slices"
	"strconv"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

// SelectFieldPath returns the JSONPath to the granted name for the given group/version: the first
// matching paths[] override, else the default fieldPath. An override with empty apiGroups/apiVersions
// matches any matched group/version.
func SelectFieldPath(ref v1alpha1.UsageReference, group, version string) string {
	for _, p := range ref.Paths {
		groupOK := len(p.APIGroups) == 0 || listMatches(p.APIGroups, group)
		versionOK := len(p.APIVersions) == 0 || listMatches(p.APIVersions, version)
		if groupOK && versionOK {
			return p.FieldPath
		}
	}
	return ref.FieldPath
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

// QuantityValuesAt returns the resource.Quantity values selected by the JSONPath expression.
// Values may be strings ("10Gi") or numbers (5); other types are skipped.
func QuantityValuesAt(factory jsonpath.Factory, obj map[string]any, path string) ([]resource.Quantity, error) {
	parsed, err := factory.Path(path)
	if err != nil {
		return nil, fmt.Errorf("parse jsonpath %q: %w", path, err)
	}
	nodes := parsed.Select(obj)
	out := make([]resource.Quantity, 0, len(nodes))
	for _, n := range nodes {
		q, ok := toQuantity(n)
		if ok {
			out = append(out, q)
		}
	}
	return out, nil
}

func toQuantity(v any) (resource.Quantity, bool) {
	switch t := v.(type) {
	case string:
		q, err := resource.ParseQuantity(t)
		if err != nil {
			return resource.Quantity{}, false
		}
		return q, true
	case int64:
		return *resource.NewQuantity(t, resource.DecimalSI), true
	case float64:
		// JSON numbers decode to float64; integer-valued counts are common.
		return resource.MustParse(strconv.FormatFloat(t, 'f', -1, 64)), true
	default:
		return resource.Quantity{}, false
	}
}

// EvalMatch reports whether a usage reference's match guard holds for the object. A nil predicate
// always holds. A predicate with neither equals nor in holds when the field is present.
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

// filterMatches reports whether a ResourceFilter matches an object by name or labels. For
// value-backed resources objLabels is nil, so only Names are consulted.
func filterMatches(f *v1alpha1.ResourceFilter, name string, objLabels labels.Set) (bool, error) {
	if f == nil {
		return false, nil
	}
	if slices.Contains(f.Names, name) {
		return true, nil
	}
	if objLabels == nil || (len(f.MatchLabels) == 0 && len(f.MatchExpressions) == 0) {
		return false, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      f.MatchLabels,
		MatchExpressions: f.MatchExpressions,
	})
	if err != nil {
		return false, fmt.Errorf("invalid excluded selector: %w", err)
	}
	return sel.Matches(objLabels), nil
}

// selectorMatches reports whether a label selector matches the object labels. A nil selector or
// nil labels never matches.
func selectorMatches(ls *metav1.LabelSelector, objLabels labels.Set) (bool, error) {
	if ls == nil || objLabels == nil {
		return false, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return false, fmt.Errorf("invalid selector: %w", err)
	}
	return sel.Matches(objLabels), nil
}

// Available decides whether a project may use a given granted object, applying the precedence:
// excluded → denied → allowed → grant availabilityDefault → registration defaultAvailability.
// grantEntries are the applicable grant resource entries (already filtered to this resourceRef).
// objLabels are the granted object's labels (nil for value-backed resources).
func Available(
	reg *v1alpha1.ClusterGrantableResource,
	grantEntries []v1alpha1.GrantResource,
	name string,
	objLabels labels.Set,
) (bool, error) {
	// 1. registration excluded — hard deny.
	if excluded, err := filterMatches(reg.Spec.Excluded, name, objLabels); err != nil {
		return false, err
	} else if excluded {
		return false, nil
	}

	// 2. grant denied / deniedSelector — admin exclusion.
	for i := range grantEntries {
		e := &grantEntries[i]
		if slices.Contains(e.Denied, name) {
			return false, nil
		}
		if matched, err := selectorMatches(e.DeniedSelector, objLabels); err != nil {
			return false, err
		} else if matched {
			return false, nil
		}
	}

	// 3. grant allowed / allowedSelector — allow.
	for i := range grantEntries {
		e := &grantEntries[i]
		if slices.Contains(e.Allowed, name) {
			return true, nil
		}
		if matched, err := selectorMatches(e.AllowedSelector, objLabels); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}

	// 4. grant availabilityDefault override (most permissive wins among entries that set it).
	sawNone := false
	for i := range grantEntries {
		switch grantEntries[i].AvailabilityDefault {
		case v1alpha1.AvailabilityAll:
			return true, nil
		case v1alpha1.AvailabilityNone:
			sawNone = true
		}
	}
	if sawNone {
		return false, nil
	}

	// 5. registration defaultAvailability (default All).
	return reg.Spec.DefaultAvailability != v1alpha1.AvailabilityNone, nil
}

// EffectiveDefault returns the per-project default granted name: the first grant entry that sets
// default wins. Empty means "use the registration's defaultFrom annotation".
func EffectiveDefault(grantEntries []v1alpha1.GrantResource) string {
	for i := range grantEntries {
		if grantEntries[i].Default != "" {
			return grantEntries[i].Default
		}
	}
	return ""
}

// Measure is a quota measure key declared by a registration.
type Measure struct {
	// Key is the measure key (resource plural for counts, quantities[].name for quantities).
	Key string
	// Count is true for countable measures (integer), false for quantity measures.
	Count bool
}

// Measures returns the deduplicated set of quota measures declared by a registration: the resource
// plural of every countable usage reference, plus every quantities[].name.
func Measures(reg *v1alpha1.ClusterGrantableResource) []Measure {
	seen := map[string]bool{}
	out := make([]Measure, 0)
	add := func(key string, count bool) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Measure{Key: key, Count: count})
	}
	for i := range reg.Spec.UsageReferences {
		ref := &reg.Spec.UsageReferences[i]
		if ref.Countable {
			for _, r := range ref.Rule.Resources {
				add(r, true)
			}
		}
		for _, q := range ref.Quantities {
			add(q.Name, false)
		}
	}
	return out
}

// Contribution is the quota a single usage object adds for one granted name.
type Contribution struct {
	// Name is the granted name referenced by the object.
	Name string
	// Increments maps a measure key to the amount added (1 for counts, the summed quantity otherwise).
	Increments map[string]resource.Quantity
}

// Contributions computes, for a usage object of the given (group, version, resource), the quota it
// adds per referenced granted name. It evaluates the rule match, the match guard and the per-version
// path. Objects that do not match any rule, or whose guard is false, contribute nothing.
func Contributions(
	factory jsonpath.Factory,
	reg *v1alpha1.ClusterGrantableResource,
	obj map[string]any,
	group, version, resourcePlural string,
) ([]Contribution, error) {
	byName := map[string]map[string]resource.Quantity{}
	for i := range reg.Spec.UsageReferences {
		ref := &reg.Spec.UsageReferences[i]
		if !RuleMatches(ref.Rule, group, version, resourcePlural) {
			continue
		}
		ok, err := EvalMatch(factory, ref.Match, obj)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		path := SelectFieldPath(*ref, group, version)
		names, err := StringValuesAt(factory, obj, path)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if name == "" {
				continue
			}
			incr := byName[name]
			if incr == nil {
				incr = map[string]resource.Quantity{}
				byName[name] = incr
			}
			if ref.Countable {
				q := incr[resourcePlural]
				q.Add(*resource.NewQuantity(1, resource.DecimalSI))
				incr[resourcePlural] = q
			}
			for _, qm := range ref.Quantities {
				qvals, err := QuantityValuesAt(factory, obj, qm.FieldPath)
				if err != nil {
					return nil, err
				}
				sum := incr[qm.Name]
				for _, qv := range qvals {
					sum.Add(qv)
				}
				incr[qm.Name] = sum
			}
		}
	}
	out := make([]Contribution, 0, len(byName))
	for name, incr := range byName {
		out = append(out, Contribution{Name: name, Increments: incr})
	}
	return out, nil
}

// IsUnlimited reports whether a quota limit means "unlimited" (negative, e.g. -1).
func IsUnlimited(limit resource.Quantity) bool {
	return limit.Sign() < 0
}

// LimitFor returns the most restrictive limit for a granted name from an objects map
// (grantedName→measure→limit): the named entry and the "*" entry both apply; the smaller wins.
// The bool is false when neither sets the measure (unlimited).
func LimitFor(objects map[string]map[string]resource.Quantity, name, measure string) (resource.Quantity, bool) {
	var best resource.Quantity
	found := false
	consider := func(key string) {
		m, ok := objects[key]
		if !ok {
			return
		}
		lim, ok := m[measure]
		if !ok {
			return
		}
		if IsUnlimited(lim) {
			// Unlimited never tightens; only record if nothing else found.
			if !found {
				best = lim
				found = true
			}
			return
		}
		if !found || (best.Sign() >= 0 && lim.Cmp(best) < 0) {
			best = lim
			found = true
		}
	}
	consider("*")
	consider(name)
	return best, found
}
