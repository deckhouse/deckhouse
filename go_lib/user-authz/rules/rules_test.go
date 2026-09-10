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

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func user(name string) Subject  { return Subject{Kind: "User", Name: name} }
func group(name string) Subject { return Subject{Kind: "Group", Name: name} }

var noLabels NamespaceLabels = func(string) (labels.Set, error) { return nil, errors.New("no lister") }

func namespaces(all map[string]map[string]string) NamespaceLabels {
	return func(name string) (labels.Set, error) {
		l, ok := all[name]
		if !ok {
			return nil, errors.New("not found")
		}
		return labels.Set(l), nil
	}
}

func build(t *testing.T, rs ...Rule) (*Directory, Stats) {
	t.Helper()
	return NewBuilder().Build(rs)
}

func entryOf(t *testing.T, d *Directory, username string, groups ...string) Entry {
	t.Helper()
	entries := d.Lookup(username, groups)
	if len(entries) == 0 {
		t.Fatalf("no entry for %s %v", username, groups)
	}
	return Combine(entries)
}

func TestMatcher_LiteralAndRegex(t *testing.T) {
	t.Parallel()
	c := newCompileCache()

	literal, err := c.compile("team-a")
	if err != nil {
		t.Fatal(err)
	}
	if literal.re != nil {
		t.Errorf("a namespace name must be matched as a literal, not compiled: %+v", literal)
	}
	if !literal.Matches("team-a") || literal.Matches("team-a-2") || literal.Matches("xteam-a") {
		t.Errorf("literal matching is wrong")
	}

	re, err := c.compile("team-.*")
	if err != nil {
		t.Fatal(err)
	}
	if re.re == nil {
		t.Errorf("a pattern with metacharacters must be compiled")
	}
	if !re.Matches("team-a") || re.Matches("xteam-a") || !re.Matches("team-") {
		t.Errorf("regex matching is wrong or not anchored")
	}

	again, _ := c.compile("team-.*")
	if again.re != re.re {
		t.Errorf("the same pattern must be compiled once")
	}

	if _, err := c.compile("team-("); err == nil {
		t.Errorf("an invalid pattern must fail to compile")
	}

	for pattern, everything := range map[string]bool{".*": true, ".+": true, "^.*$": true, "team-.*": false} {
		m, _ := c.compile(pattern)
		if m.MatchesEverything() != everything {
			t.Errorf("%q: MatchesEverything = %v, want %v", pattern, m.MatchesEverything(), everything)
		}
	}
}

func TestWrapRegex(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"a": "^a$", "^a": "^a$", "a$": "^a$", "^a$": "^a$", ".*": "^.*$",
		// An alternation gets a group, so that both branches are anchored. Written without one,
		// "^team-.*|kube-system$" parses as (^team-.*)|(kube-system$).
		"team-.*|kube-system":   "^(?:team-.*|kube-system)$",
		"^team-.*|kube-system$": "^(?:team-.*|kube-system)$",
		// A "|" that is not a top-level alternation is left where it is, so an ordinary pattern
		// keeps the byte-identical form the literal fast path and MatchesEverything read back.
		"(a|b)-ns":    "^(a|b)-ns$",
		"[a|b]-ns":    "^[a|b]-ns$",
		`a\|b`:        `^a\|b$`,
		"team-[0-9]+": "^team-[0-9]+$",
	} {
		if got := WrapRegex(in); got != want {
			t.Errorf("WrapRegex(%q) = %q, want %q", in, got, want)
		}
	}
}

// The anchoring bug, stated as the access it granted: a rule limited to "team-.*|kube-system" used
// to open every namespace whose name merely ended in "kube-system".
func TestWrapRegex_AlternationDoesNotLeakASuffixMatch(t *testing.T) {
	t.Parallel()
	c := newCompileCache()
	m, err := c.compile("team-.*|kube-system")
	if err != nil {
		t.Fatal(err)
	}

	for _, ns := range []string{"team-a", "team-", "kube-system"} {
		if !m.Matches(ns) {
			t.Errorf("%q must be covered", ns)
		}
	}
	for _, ns := range []string{"attacker-kube-system", "d8-kube-system", "other"} {
		if m.Matches(ns) {
			t.Errorf("%q must NOT be covered: only the two branches as written are", ns)
		}
	}
}

// The quoted-run hole, stated as the access it granted: a rule limited to `\Q(\E|x` used to open
// every namespace whose name ended in "x".
func TestWrapRegex_QuotedRunDoesNotHideAnAlternation(t *testing.T) {
	t.Parallel()
	c := newCompileCache()
	m, err := c.compile(`\Q(\E|x`)
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range []string{"(", "x"} {
		if !m.Matches(ns) {
			t.Errorf("%q must be covered: it is one of the branches as written", ns)
		}
	}
	if m.Matches("attacker-x") {
		t.Error(`"attacker-x" must NOT be covered: the alternation inside a quoted run is still an alternation`)
	}
}

func TestHasTopLevelAlternation(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{
		"a|b":     true,
		"^a|b$":   true,
		"a":       false,
		"(a|b)":   false,
		"(a|b)|c": true,
		"[a|b]":   false,
		"[a|b]|c": true,
		// RE2 supports \Q...\E, and everything inside is literal - including a "(" that opens no
		// group. A scanner that misses this believes it is inside a group at the "|", leaves the
		// pattern anchored by concatenation, and the second branch then matches any name that
		// merely ENDS in it.
		`\Q(\E|x`:     true,
		`\Q|\E`:       false,
		`\Qa|b\E`:     false,
		`\Qa|b\E|c`:   true,
		`\Q[\E|x`:     true,
		`\Q\E|x`:      true,
		`a\Q(\Eb`:     false,
		`a\|b`:        false,
		`\[a|b`:       true,
		"((a|b)|c)":   false,
		"team-[0-9]+": false,
	} {
		if got := hasTopLevelAlternation(in); got != want {
			t.Errorf("hasTopLevelAlternation(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuild_LimitNamespaces(t *testing.T) {
	t.Parallel()
	d, stats := build(t,
		Rule{Name: "team-a", Subjects: []Subject{user("alice"), group("devs")}, LimitNamespaces: []string{"team-a", "team-a-.*"}},
		Rule{Name: "ops", Subjects: []Subject{group("devs")}, LimitNamespaces: []string{"ops"}, AllowAccessToSystemNamespaces: true},
	)
	if stats.Rules != 2 || stats.Subjects != 2 || len(stats.Quarantined) != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if !d.KnowsRule("team-a") || !d.KnowsRule("ops") || d.KnowsRule("other") {
		t.Errorf("KnowsRule is wrong")
	}

	alice := entryOf(t, d, "alice")
	if alice.NamespaceFiltersAbsent || alice.AllowAccessToSystemNamespaces || len(alice.LimitNamespaces) != 2 || !alice.HasAnyFilters() {
		t.Errorf("alice = %+v", alice)
	}

	// alice through the devs group gets the union: both rules, and the system-namespace flag of ops
	both := entryOf(t, d, "alice", "devs")
	if len(both.LimitNamespaces) != 5 || !both.AllowAccessToSystemNamespaces {
		t.Errorf("alice+devs = %+v", both)
	}
	for ns, want := range map[string]bool{"team-a": true, "team-a-1": true, "ops": true, "team-b": false, "kube-system": false} {
		if got, _ := NamespaceAllowed(&both, ns, noLabels); got != want {
			t.Errorf("alice+devs in %s = %v, want %v", ns, got, want)
		}
	}
	// the system-namespace flag does not add namespaces outside the patterns
	if got, _ := NamespaceAllowed(&both, "d8-system", noLabels); got {
		t.Errorf("allowAccessToSystemNamespaces must not open a system namespace the patterns do not name")
	}
}

func TestBuild_NoFilters(t *testing.T) {
	t.Parallel()
	d, _ := build(t, Rule{Name: "open", Subjects: []Subject{user("bob")}})
	bob := entryOf(t, d, "bob")
	if !bob.NamespaceFiltersAbsent || !bob.HasAnyFilters() {
		t.Fatalf("a rule without filters still gates the system namespaces: %+v", bob)
	}
	for ns, want := range map[string]bool{"team-a": true, "kube-system": false, "d8-system": false, "default": false, "antiopa": false} {
		if got, _ := NamespaceAllowed(&bob, ns, noLabels); got != want {
			t.Errorf("bob in %s = %v, want %v", ns, got, want)
		}
	}

	d, _ = build(t, Rule{Name: "open-all", Subjects: []Subject{user("bob")}, AllowAccessToSystemNamespaces: true})
	bob = entryOf(t, d, "bob")
	if bob.HasAnyFilters() {
		t.Errorf("no filters and system namespaces allowed means no opinion: %+v", bob)
	}

	d, _ = build(t, Rule{Name: "star", Subjects: []Subject{user("bob")}, LimitNamespaces: []string{".*"}, AllowAccessToSystemNamespaces: true})
	if bob = entryOf(t, d, "bob"); bob.HasAnyFilters() {
		t.Errorf("a pattern matching everything plus system namespaces means no opinion: %+v", bob)
	}
}

func TestBuild_NamespaceSelector(t *testing.T) {
	t.Parallel()
	selector := &NamespaceSelector{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}}}
	d, _ := build(t,
		// limitNamespaces and the system flag are ignored when a selector is present
		Rule{Name: "sel", Subjects: []Subject{user("carol")}, NamespaceSelector: selector, LimitNamespaces: []string{"ignored"}, AllowAccessToSystemNamespaces: true},
	)
	carol := entryOf(t, d, "carol")
	if len(carol.LimitNamespaces) != 0 || carol.AllowAccessToSystemNamespaces || carol.NamespaceFiltersAbsent || len(carol.NamespaceSelectors) != 1 || !carol.HasAnyFilters() {
		t.Fatalf("carol = %+v", carol)
	}
	ns := namespaces(map[string]map[string]string{
		"team-a":      {"team": "a"},
		"kube-system": {"team": "a"},
		"other":       {"team": "b"},
	})
	for name, want := range map[string]bool{"team-a": true, "kube-system": true, "other": false, "ignored": false} {
		if got, _ := NamespaceAllowed(&carol, name, ns); got != want {
			t.Errorf("carol in %s = %v, want %v", name, got, want)
		}
	}
	// a namespace the lister does not know is not opened and the error is reported to the caller
	if got, err := NamespaceAllowed(&carol, "missing", ns); got || err == nil {
		t.Errorf("missing namespace: allowed=%v err=%v", got, err)
	}

	d, _ = build(t, Rule{Name: "any", Subjects: []Subject{user("carol")}, NamespaceSelector: &NamespaceSelector{MatchAny: true}})
	if carol = entryOf(t, d, "carol"); carol.HasAnyFilters() {
		t.Errorf("matchAny means no opinion: %+v", carol)
	}

	// an empty selector object still wins over the patterns and opens nothing
	d, _ = build(t, Rule{Name: "empty", Subjects: []Subject{user("carol")}, NamespaceSelector: &NamespaceSelector{}, LimitNamespaces: []string{"team-a"}})
	carol = entryOf(t, d, "carol")
	if got, _ := NamespaceAllowed(&carol, "team-a", ns); got || !carol.HasAnyFilters() {
		t.Errorf("an empty namespaceSelector must override limitNamespaces: %+v", carol)
	}
}

func TestBuild_Quarantine(t *testing.T) {
	t.Parallel()
	d, stats := build(t,
		Rule{Name: "broken", Subjects: []Subject{user("dave")}, LimitNamespaces: []string{"team-(", "team-b"}},
		Rule{Name: "fine", Subjects: []Subject{user("erin")}, LimitNamespaces: []string{"team-c"}},
	)
	if len(stats.Quarantined) != 1 || stats.Quarantined["broken"] == nil {
		t.Fatalf("quarantined = %v", stats.Quarantined)
	}
	if !d.KnowsRule("broken") {
		t.Errorf("a quarantined rule is still a known rule")
	}
	dave := entryOf(t, d, "dave")
	if !dave.Quarantined || dave.NamespaceFiltersAbsent || len(dave.LimitNamespaces) != 1 {
		t.Fatalf("dave = %+v", dave)
	}
	// the valid pattern of the broken rule still applies, the broken one opens nothing
	for ns, want := range map[string]bool{"team-b": true, "team-a": false, "team-x": false} {
		if got, _ := NamespaceAllowed(&dave, ns, noLabels); got != want {
			t.Errorf("dave in %s = %v, want %v", ns, got, want)
		}
	}
	erin := entryOf(t, d, "erin")
	if erin.Quarantined {
		t.Errorf("a broken rule must not taint other subjects")
	}

	// a rule whose only pattern is broken denies everything for its subjects
	d, _ = build(t, Rule{Name: "broken", Subjects: []Subject{user("dave")}, LimitNamespaces: []string{"team-("}})
	dave = entryOf(t, d, "dave")
	if got, _ := NamespaceAllowed(&dave, "team-a", noLabels); got || !dave.HasAnyFilters() {
		t.Errorf("a rule with only broken patterns must deny: %+v", dave)
	}
}

func TestRestricted(t *testing.T) {
	t.Parallel()
	r := Restricted()
	if !r.HasAnyFilters() {
		t.Fatal("the restricted entry must have filters")
	}
	for _, ns := range []string{"team-a", "kube-system", "default"} {
		if got, _ := NamespaceAllowed(&r, ns, noLabels); got {
			t.Errorf("the restricted entry must not open %s", ns)
		}
	}
	if !ClusterScopedDenied(ResourceScope{Known: true, Namespaced: true}) {
		t.Errorf("a namespaced resource is denied cluster-wide for a filtered entry")
	}
}

func TestClusterScopedDenied(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		scope ResourceScope
		want  bool
	}{
		{"cluster-scoped resource", ResourceScope{Known: true, Namespaced: false}, false},
		{"namespaced resource", ResourceScope{Known: true, Namespaced: true}, true},
		{"could not look the resource up: stay closed", ResourceScope{Known: false}, true},
		{"discovery answered and the resource is absent: RBAC answers, the API server 404s", ResourceScope{Known: false, Absent: true}, false},
		{"absent is only consulted when the resource is unknown", ResourceScope{Known: true, Namespaced: true, Absent: true}, true},
	}
	for _, tc := range cases {
		if got := ClusterScopedDenied(tc.scope); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLookup_ServiceAccountsAndNil(t *testing.T) {
	t.Parallel()
	d, _ := build(t, Rule{Name: "sa", Subjects: []Subject{{Kind: "ServiceAccount", Name: "bot", Namespace: "ci"}}, LimitNamespaces: []string{"ci"}})
	if len(d.Lookup("system:serviceaccount:ci:bot", nil)) != 1 {
		t.Errorf("a service account is looked up by its username")
	}
	if len(d.Lookup("bot", nil)) != 0 {
		t.Errorf("a service account is not a user")
	}
	var nilDir *Directory
	if nilDir.Lookup("anyone", nil) != nil || nilDir.KnowsRule("x") || nilDir.Len() != 0 {
		t.Errorf("a nil directory knows nothing")
	}
}

func TestFromUnstructured_AndProject(t *testing.T) {
	t.Parallel()
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "ClusterAuthorizationRule",
		"metadata": map[string]interface{}{
			"name":            "team-a",
			"resourceVersion": "42",
			"generation":      int64(3),
			"annotations":     map[string]interface{}{"drop": "me"},
			"managedFields":   []interface{}{map[string]interface{}{"manager": "x"}},
		},
		"spec": map[string]interface{}{
			"accessLevel":                   "Admin",
			"additionalRoles":               []interface{}{map[string]interface{}{"name": "x"}},
			"allowAccessToSystemNamespaces": true,
			"limitNamespaces":               []interface{}{"team-a", "team-a-.*"},
			"subjects": []interface{}{
				map[string]interface{}{"kind": "User", "name": "alice"},
				map[string]interface{}{"kind": "ServiceAccount", "name": "bot", "namespace": "ci"},
			},
		},
		"status": map[string]interface{}{"conditions": []interface{}{}},
	}}

	projected, err := Project(u)
	if err != nil {
		t.Fatal(err)
	}
	pu := projected.(*unstructured.Unstructured)
	if _, found, _ := unstructured.NestedFieldNoCopy(pu.Object, "spec", "additionalRoles"); found {
		t.Errorf("projection must drop additionalRoles")
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(pu.Object, "status"); found {
		t.Errorf("projection must drop status")
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(pu.Object, "metadata", "managedFields"); found {
		t.Errorf("projection must drop managedFields")
	}

	rule, err := FromUnstructured(pu)
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name != "team-a" || rule.ResourceVersion != "42" || rule.Generation != 3 || !rule.AllowAccessToSystemNamespaces {
		t.Errorf("rule = %+v", rule)
	}
	if len(rule.LimitNamespaces) != 2 || len(rule.Subjects) != 2 || rule.Subjects[1].Namespace != "ci" {
		t.Errorf("rule = %+v", rule)
	}

	d, stats := build(t, rule)
	if stats.MaxResourceVersion != 42 || d.MaxResourceVersion() != 42 {
		t.Errorf("resourceVersion watermark = %d", d.MaxResourceVersion())
	}
}

func TestIsSystemNamespace(t *testing.T) {
	t.Parallel()
	for ns, want := range map[string]bool{"kube-system": true, "d8-monitoring": true, "default": true, "antiopa": true, "loghouse": true, "team-a": false, "kube": false, "d8": false, "defaults": false} {
		if got := IsSystemNamespace(ns); got != want {
			t.Errorf("IsSystemNamespace(%q) = %v, want %v", ns, got, want)
		}
	}
}

// A directory answers RuleCovers only for the subjects its own copy of the rule names. The ordering
// guard depends on this: a subject added to an existing rule keeps the rule's name, so a name-only
// check would think the rule already accounted for it.
func TestDirectoryRuleCovers(t *testing.T) {
	t.Parallel()
	dir, _ := NewBuilder().Build([]Rule{{
		Name: "team-a",
		Subjects: []Subject{
			{Kind: "User", Name: "alice"},
			{Kind: "Group", Name: "devs"},
			{Kind: "ServiceAccount", Name: "bot", Namespace: "ci"},
		},
		LimitNamespaces: []string{"dev"},
	}})

	if !dir.RuleCovers("team-a", "alice", nil) {
		t.Error("the rule names alice")
	}
	if !dir.RuleCovers("team-a", "someone", []string{"devs"}) {
		t.Error("the rule names the group devs")
	}
	if !dir.RuleCovers("team-a", "system:serviceaccount:ci:bot", nil) {
		t.Error("the rule names the service account")
	}
	if dir.RuleCovers("team-a", "bob", []string{"ops"}) {
		t.Error("the rule does not name bob, so the guard must fire")
	}
	if dir.RuleCovers("team-b", "alice", nil) {
		t.Error("an unknown rule covers nobody")
	}
	var nilDir *Directory
	if nilDir.RuleCovers("team-a", "alice", nil) {
		t.Error("a directory that has not been built covers nobody")
	}
}
