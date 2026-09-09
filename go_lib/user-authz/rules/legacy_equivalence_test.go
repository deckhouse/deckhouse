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

package rules

// This file pins the package against the implementation it replaced.
//
// Before this library existed, the authorization webhook built its own directory from a rendered
// config.json and decided namespaced requests inline. That code is transcribed below, unchanged in
// behaviour, from ee/be/modules/140-user-authz/images/webhook/src/internal/web/hook at the commit
// before the switch. The test then puts both implementations through the same rules and namespaces
// and requires that they answer identically.
//
// The point is not to keep the old code alive. It is that "the library decides what the webhook
// used to decide" is a claim about a few thousand combinations, and a claim that size should be
// checked by a machine on every run rather than argued once in a review.
//
// What this file does NOT establish, stated so nobody reads more into it than it proves:
//
//   - The comparison is against the WEBHOOK's implementation. permission-browser had a second copy
//     of the same decision, and it is not transcribed here. The two had drifted, which is why the
//     library exists; only one side of that drift is pinned below.
//   - The cases are a cross-product of a hand-picked list of rule shapes and namespace names. It is
//     a few thousand comparisons, not a few thousand independent scenarios, and nothing is
//     generated or fuzzed: a disagreement on a shape outside the list is unreachable from here.
//
// Deliberate differences are asserted rather than hidden, at the bottom of the file: the namespaced
// ones in TestLegacyEquivalence_DeliberateDifferences, and the cluster-scoped one - the 403 that
// should have been a 404 - in TestLegacyEquivalence_ClusterScoped.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ---------------------------------------------------------------- the old code

type legacyEntry struct {
	AllowAccessToSystemNamespaces bool
	LimitNamespaces               []*regexp.Regexp
	NamespaceSelectors            []*NamespaceSelector
	NamespaceFiltersAbsent        bool
}

var legacySystemNamespaces = []string{"kube-.*", "d8-.*", "default", "antiopa", "loghouse"}

var legacySystemNamespacesRegex []*regexp.Regexp

func init() {
	for _, ns := range legacySystemNamespaces {
		r, _ := regexp.Compile("^" + ns + "$")
		legacySystemNamespacesRegex = append(legacySystemNamespacesRegex, r)
	}
}

func legacyWrapRegex(ln string) string {
	if !strings.HasPrefix(ln, "^") {
		ln = "^" + ln
	}
	if !strings.HasSuffix(ln, "$") {
		ln += "$"
	}
	return ln
}

func legacyIsLabelSelectorApplied(s *NamespaceSelector) bool {
	return s != nil && s.LabelSelector != nil
}

// legacyBuildDirectory is renewDirectories without the file handling. Note the early return when a
// pattern does not compile: the whole rebuild was abandoned and the previous directory kept.
func legacyBuildDirectory(rules []Rule) (map[string]map[string]legacyEntry, bool) {
	directory := map[string]map[string]legacyEntry{
		"User":           make(map[string]legacyEntry),
		"Group":          make(map[string]legacyEntry),
		"ServiceAccount": make(map[string]legacyEntry),
	}

	for _, crd := range rules {
		for _, subject := range crd.Subjects {
			name := subject.Name
			if subject.Kind == "ServiceAccount" {
				name = "system:serviceaccount:" + subject.Namespace + ":" + subject.Name
			}
			if _, ok := directory[subject.Kind]; !ok {
				continue
			}
			dirEntry := directory[subject.Kind][name]

			dirEntry.NamespaceFiltersAbsent = dirEntry.NamespaceFiltersAbsent ||
				(len(crd.LimitNamespaces) == 0 && !legacyIsLabelSelectorApplied(crd.NamespaceSelector))

			if crd.NamespaceSelector == nil {
				for _, ln := range crd.LimitNamespaces {
					r, err := regexp.Compile(legacyWrapRegex(ln))
					if err != nil {
						return nil, false // the rebuild was abandoned
					}
					dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
				}
				if !dirEntry.AllowAccessToSystemNamespaces {
					dirEntry.AllowAccessToSystemNamespaces = crd.AllowAccessToSystemNamespaces
				}
			} else {
				dirEntry.NamespaceSelectors = append(dirEntry.NamespaceSelectors, crd.NamespaceSelector)
			}

			directory[subject.Kind][name] = dirEntry
		}
	}
	return directory, true
}

func legacyAffected(dir map[string]map[string]legacyEntry, user string, groups []string) []legacyEntry {
	var out []legacyEntry
	if e, ok := dir["User"][user]; ok {
		out = append(out, e)
	}
	if e, ok := dir["ServiceAccount"][user]; ok {
		out = append(out, e)
	}
	for _, g := range groups {
		if e, ok := dir["Group"][g]; ok {
			out = append(out, e)
		}
	}
	return out
}

func legacyCombine(entries []legacyEntry) legacyEntry {
	var combined legacyEntry
	for _, e := range entries {
		if !combined.AllowAccessToSystemNamespaces {
			combined.AllowAccessToSystemNamespaces = e.AllowAccessToSystemNamespaces
		}
		if len(e.NamespaceSelectors) > 0 {
			combined.NamespaceSelectors = append(combined.NamespaceSelectors, e.NamespaceSelectors...)
		}
		if len(e.LimitNamespaces) > 0 {
			combined.LimitNamespaces = append(combined.LimitNamespaces, e.LimitNamespaces...)
		}
		combined.NamespaceFiltersAbsent = combined.NamespaceFiltersAbsent || e.NamespaceFiltersAbsent
	}
	return combined
}

func legacyHasAnyFilters(entry *legacyEntry) bool {
	for _, s := range entry.NamespaceSelectors {
		if s.MatchAny {
			return false
		}
	}
	if entry.NamespaceFiltersAbsent {
		return !entry.AllowAccessToSystemNamespaces
	}
	for _, r := range entry.LimitNamespaces {
		switch r.String() {
		case "^.*$", "^.+$":
			return !entry.AllowAccessToSystemNamespaces
		}
	}
	return true
}

func legacySelectorMatches(nsLabels labels.Set, selectors []*NamespaceSelector) bool {
	for _, s := range selectors {
		if s.LabelSelector == nil {
			continue
		}
		sel, err := metav1.LabelSelectorAsSelector(s.LabelSelector)
		if err != nil {
			return false
		}
		if sel.Matches(nsLabels) {
			return true
		}
	}
	return false
}

// legacyDenied is authorizeRequest plus authorizeNamespacedRequest, minus the CAR-independent RBAC
// rescue, which lives outside this package in both implementations.
func legacyDenied(dir map[string]map[string]legacyEntry, user string, groups []string, ns string, nsLabels labels.Set) bool {
	entries := legacyAffected(dir, user, groups)
	if len(entries) == 0 {
		return false
	}
	entry := legacyCombine(entries)

	if !legacyHasAnyFilters(&entry) {
		return false
	}

	denied := true
	if !entry.NamespaceFiltersAbsent {
		for _, p := range entry.LimitNamespaces {
			if p.MatchString(ns) {
				denied = false
				break
			}
		}
	} else {
		denied = false
	}

	if !denied && !entry.AllowAccessToSystemNamespaces {
		for _, p := range legacySystemNamespacesRegex {
			if p.MatchString(ns) {
				denied = true
				break
			}
		}
	}

	if denied && len(entry.NamespaceSelectors) > 0 && legacySelectorMatches(nsLabels, entry.NamespaceSelectors) {
		denied = false
	}
	return denied
}

// ------------------------------------------------------------- the new code

func libraryDenied(dir *Directory, user string, groups []string, ns string, nsLabels labels.Set) bool {
	entries := dir.Lookup(user, groups)
	if len(entries) == 0 {
		return false
	}
	combined := Combine(entries)
	if !combined.HasAnyFilters() {
		return false
	}
	allowed, _ := NamespaceAllowed(&combined, ns, func(string) (labels.Set, error) { return nsLabels, nil })
	return !allowed
}

// ------------------------------------------------------------- the comparison

// equivalenceNamespaces covers a regular namespace, each system pattern, the two legacy ones, and a
// namespace whose labels a selector can match.
var equivalenceNamespaces = map[string]labels.Set{
	"team-a":      {"tier": "app"},
	"team-b":      {},
	"picked":      {"pick": "yes", "tier": "app"},
	"kube-system": {},
	"kube-public": {},
	"d8-system":   {"pick": "yes"},
	"default":     {},
	"antiopa":     {},
	"loghouse":    {"pick": "yes"},
}

func selectorMatchLabels(m map[string]string) *NamespaceSelector {
	return &NamespaceSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: m}}
}

// equivalenceRules are the shapes a ClusterAuthorizationRule can take that the decision depends on.
func equivalenceRules(subject Subject) []Rule {
	name := func(i int) string { return fmt.Sprintf("rule-%d", i) }
	specs := []Rule{
		{LimitNamespaces: []string{"team-a"}},
		{LimitNamespaces: []string{"team-.*"}},
		{LimitNamespaces: []string{"^team-a$"}},
		{LimitNamespaces: []string{".*"}},
		{LimitNamespaces: []string{".+"}},
		{LimitNamespaces: []string{"kube-system"}},
		{LimitNamespaces: []string{"team-a", "kube-.*"}},
		{LimitNamespaces: []string{"team-a"}, AllowAccessToSystemNamespaces: true},
		{LimitNamespaces: []string{".*"}, AllowAccessToSystemNamespaces: true},
		{},
		{AllowAccessToSystemNamespaces: true},
		{NamespaceSelector: &NamespaceSelector{MatchAny: true}},
		{NamespaceSelector: selectorMatchLabels(map[string]string{"pick": "yes"})},
		{NamespaceSelector: selectorMatchLabels(map[string]string{"tier": "app"})},
		{NamespaceSelector: selectorMatchLabels(map[string]string{"absent": "1"})},
		{NamespaceSelector: &NamespaceSelector{}},
		// a selector wins over limits and over the system flag, in both implementations
		{LimitNamespaces: []string{"team-b"}, NamespaceSelector: selectorMatchLabels(map[string]string{"pick": "yes"}), AllowAccessToSystemNamespaces: true},
	}
	out := make([]Rule, 0, len(specs))
	for i, s := range specs {
		s.Name = name(i)
		s.Subjects = []Subject{subject}
		out = append(out, s)
	}
	return out
}

// TestLegacyEquivalence_SingleRule compares every rule shape on its own.
func TestLegacyEquivalence_SingleRule(t *testing.T) {
	t.Parallel()
	subject := Subject{Kind: "User", Name: "alice"}
	compared := 0
	for _, rule := range equivalenceRules(subject) {
		legacyDir, ok := legacyBuildDirectory([]Rule{rule})
		if !ok {
			t.Fatalf("the reference implementation abandoned the build for %+v", rule)
		}
		dir, _ := NewBuilder().Build([]Rule{rule})

		for ns, nsLabels := range equivalenceNamespaces {
			want := legacyDenied(legacyDir, "alice", nil, ns, nsLabels)
			got := libraryDenied(dir, "alice", nil, ns, nsLabels)
			compared++
			if want != got {
				t.Errorf("rule %s in %s: the webhook denied=%v, the library denies=%v\n  rule: %+v",
					rule.Name, ns, want, got, rule)
			}
		}
	}
	t.Logf("compared %d decisions", compared)
}

// TestLegacyEquivalence_RulePairs compares every ordered pair, which is where the union of entries
// and the priority between limits, selectors and the system flag actually get exercised.
func TestLegacyEquivalence_RulePairs(t *testing.T) {
	t.Parallel()
	subject := Subject{Kind: "User", Name: "alice"}
	all := equivalenceRules(subject)
	compared, mismatches := 0, 0

	for i := range all {
		for j := range all {
			pair := []Rule{all[i], all[j]}
			pair[1].Name = "rule-b"

			legacyDir, ok := legacyBuildDirectory(pair)
			if !ok {
				continue
			}
			dir, _ := NewBuilder().Build(pair)

			for ns, nsLabels := range equivalenceNamespaces {
				want := legacyDenied(legacyDir, "alice", nil, ns, nsLabels)
				got := libraryDenied(dir, "alice", nil, ns, nsLabels)
				compared++
				if want != got && mismatches < 10 {
					mismatches++
					t.Errorf("pair (%d,%d) in %s: the webhook denied=%v, the library denies=%v\n  a: %+v\n  b: %+v",
						i, j, ns, want, got, all[i], all[j])
				}
			}
		}
	}
	t.Logf("compared %d decisions", compared)
}

// TestLegacyEquivalence_SubjectKinds checks that a subject is found by the same key in both: the
// username for a User, the canonical name for a ServiceAccount, the group name for a Group.
func TestLegacyEquivalence_SubjectKinds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		subject Subject
		user    string
		groups  []string
	}{
		{Subject{Kind: "User", Name: "alice"}, "alice", nil},
		{Subject{Kind: "User", Name: "alice"}, "bob", nil},
		{Subject{Kind: "Group", Name: "devs"}, "anyone", []string{"devs"}},
		{Subject{Kind: "Group", Name: "devs"}, "anyone", []string{"ops"}},
		{Subject{Kind: "ServiceAccount", Name: "bot", Namespace: "ci"}, "system:serviceaccount:ci:bot", nil},
		{Subject{Kind: "ServiceAccount", Name: "bot", Namespace: "ci"}, "system:serviceaccount:other:bot", nil},
	}

	compared := 0
	for _, c := range cases {
		for _, rule := range equivalenceRules(c.subject) {
			legacyDir, ok := legacyBuildDirectory([]Rule{rule})
			if !ok {
				continue
			}
			dir, _ := NewBuilder().Build([]Rule{rule})
			for ns, nsLabels := range equivalenceNamespaces {
				want := legacyDenied(legacyDir, c.user, c.groups, ns, nsLabels)
				got := libraryDenied(dir, c.user, c.groups, ns, nsLabels)
				compared++
				if want != got {
					t.Errorf("%s/%s as %q%v in %s: the webhook denied=%v, the library denies=%v",
						c.subject.Kind, c.subject.Name, c.user, c.groups, ns, want, got)
				}
			}
		}
	}
	t.Logf("compared %d decisions", compared)
}

// TestLegacyEquivalence_DeliberateDifferences records where the library is meant to differ, so that
// a change to either behaviour has to come here and say so.
func TestLegacyEquivalence_DeliberateDifferences(t *testing.T) {
	t.Parallel()
	subject := Subject{Kind: "User", Name: "alice"}

	// A rule with an uncompilable pattern made the old rebuild return early, leaving the previous
	// directory in place: a brand-new rule simply never applied, and its subject stayed
	// unrestricted. The library quarantines the pattern instead, which can only narrow.
	broken := Rule{Name: "broken", Subjects: []Subject{subject}, LimitNamespaces: []string{"[unclosed"}}

	if _, ok := legacyBuildDirectory([]Rule{broken}); ok {
		t.Fatal("the reference implementation is supposed to abandon the build on a bad pattern")
	}

	dir, stats := NewBuilder().Build([]Rule{broken})
	if len(stats.Quarantined) != 1 {
		t.Errorf("the library must quarantine the rule, got %v", stats.Quarantined)
	}
	if !libraryDenied(dir, "alice", nil, "team-a", labels.Set{}) {
		t.Error("the library must deny the subject of a rule it could not compile")
	}

	// A healthy rule alongside the broken one survives in the library; in the old code the whole
	// rebuild was lost.
	healthy := Rule{Name: "healthy", Subjects: []Subject{subject}, LimitNamespaces: []string{"team-a"}}
	dir, _ = NewBuilder().Build([]Rule{broken, healthy})
	if libraryDenied(dir, "alice", nil, "team-a", labels.Set{}) {
		t.Error("a healthy rule must still apply next to a broken one")
	}
	if !libraryDenied(dir, "alice", nil, "team-b", labels.Set{}) {
		t.Error("and it must not open anything beyond itself")
	}
}

// legacyClusterScopedDenied transcribes authorizeClusterScopedRequest from the same commit, in the
// terms the library now uses. The original asked the discovery cache directly; what mattered was
// which of its answers denied:
//
//   - a preferred-version lookup that failed             -> deny (internal error)
//   - the core group, resource not in the core listing   -> no opinion, let RBAC answer
//   - a Get that failed                                  -> deny (internal error)
//   - namespaced, and the subject is limited             -> deny
//   - cluster-scoped                                     -> no opinion
//
// coreGroup says whether the request named the core group, because that is the only place the old
// code distinguished "does not exist" from "could not ask".
func legacyClusterScopedDenied(scope ResourceScope, coreGroup bool) bool {
	if scope.Known {
		return scope.Namespaced
	}
	if coreGroup && scope.Absent {
		// The core listing answered and does not carry the resource.
		return false
	}
	// Anything else was a lookup error, and a lookup error denied.
	return true
}

// TestLegacyEquivalence_AlternationAnchoring records the one namespaced difference: an alternation
// in limitNamespaces is now anchored on both branches.
//
// The old code anchored by concatenation, so "team-.*|kube-system" became
// (^team-.*)|(kube-system$) and opened every namespace whose name ended in "kube-system". The
// cross-product above does not catch this because its fixture patterns contain no alternation -
// which is exactly why it is written out here instead of being left to luck.
//
// This is a narrowing change. A rule written with an alternation stops covering names it was never
// meant to cover; nothing gains access.
func TestLegacyEquivalence_AlternationAnchoring(t *testing.T) {
	t.Parallel()
	const pattern = "team-.*|kube-system"

	legacy, err := regexp.Compile(legacyWrapRegex(pattern))
	if err != nil {
		t.Fatal(err)
	}
	current, err := newCompileCache().compile(pattern)
	if err != nil {
		t.Fatal(err)
	}

	// What both agree on: the branches as written.
	for _, ns := range []string{"team-a", "kube-system"} {
		if !legacy.MatchString(ns) || !current.Matches(ns) {
			t.Errorf("%q must be covered by both", ns)
		}
	}

	// And the difference.
	const leaked = "attacker-kube-system"
	if !legacy.MatchString(leaked) {
		t.Fatalf("the reference implementation is supposed to match %q; if it no longer does, this "+
			"difference has been resolved elsewhere and this test should say so", leaked)
	}
	if current.Matches(leaked) {
		t.Errorf("%q is still covered: the alternation is not anchored on both branches", leaked)
	}
}

// TestLegacyEquivalence_ClusterScoped compares the cluster-scoped half, which the rest of this file
// does not cover - and which is where the two implementations had actually diverged.
//
// Three of the four outcomes are unchanged. The fourth is the fix: a resource in a NAMED group that
// discovery says does not exist used to be a denial, because the old code could not tell that
// answer from a failure to reach discovery, and the user saw Forbidden for something that was never
// there. The library separates the two, so RBAC answers and the API server produces the 404 it owes.
func TestLegacyEquivalence_ClusterScoped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		scope     ResourceScope
		coreGroup bool
		// differs is set on the one cell where the library is meant to disagree.
		differs bool
	}{
		{name: "a cluster-scoped resource", scope: ResourceScope{Known: true, Namespaced: false}},
		{name: "a namespaced resource", scope: ResourceScope{Known: true, Namespaced: true}},
		{name: "a lookup that did not happen", scope: ResourceScope{}},
		{name: "a lookup that did not happen, core group", scope: ResourceScope{}, coreGroup: true},
		{name: "the core group does not carry it", scope: ResourceScope{Absent: true}, coreGroup: true},
		{
			name:    "a named group does not carry it",
			scope:   ResourceScope{Absent: true},
			differs: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacy := legacyClusterScopedDenied(tc.scope, tc.coreGroup)
			library := ClusterScopedDenied(tc.scope)

			if tc.differs {
				if legacy == library {
					t.Fatalf("this cell is supposed to differ, both answered %v; if the behaviour was "+
						"changed back, say so here", library)
				}
				if library {
					t.Error("the library must NOT deny a resource discovery says does not exist: " +
						"RBAC answers and the API server returns 404")
				}
				return
			}
			if legacy != library {
				t.Errorf("legacy denied=%v, library denied=%v", legacy, library)
			}
		})
	}
}
