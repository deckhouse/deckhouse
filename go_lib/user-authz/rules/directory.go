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
	"fmt"
	"maps"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Selector is one compiled spec.namespaceSelector of a rule.
type Selector struct {
	// MatchAny opens every namespace, system ones included.
	MatchAny bool
	// Labels is the compiled label selector; nil when the selector carried none or did not compile
	// (the rule is then quarantined, see Entry.Quarantined).
	Labels labels.Selector
}

// Entry is what the directory knows about one subject: the union of every rule naming it.
type Entry struct {
	// AllowAccessToSystemNamespaces is set when any rule naming the subject sets it.
	AllowAccessToSystemNamespaces bool
	// LimitNamespaces are the patterns of every rule naming the subject that uses limitNamespaces.
	LimitNamespaces []Matcher
	// NamespaceSelectors are the selectors of every rule naming the subject that uses namespaceSelector.
	NamespaceSelectors []Selector
	// NamespaceFiltersAbsent is set when at least one rule naming the subject has neither
	// limitNamespaces nor namespaceSelector: such a rule opens every non-system namespace, and the
	// union of rules can only be wider than that.
	NamespaceFiltersAbsent bool
	// Quarantined is set when a rule naming the subject could not be compiled (an invalid regular
	// expression or label selector). The offending pattern is left out, which can only narrow the
	// entry; the flag lets callers report the rule rather than silently serve the narrower scope.
	Quarantined bool
}

// Restricted is the entry of a subject whose rules are known to exist but are not in the directory
// yet: it names no namespace, so every namespaced request outside CAR-independent RBAC is denied.
// The webhook uses it for a subject bound by a ClusterRoleBinding of user-authz-controller whose rule
// it has not observed, so that the binding, which is created seconds after the rule, never grants
// more than the rule will.
func Restricted() Entry {
	return Entry{}
}

// Note for callers of Restricted: it is the zero entry, and Combine is a union, so combining it
// with an entry that has already been observed leaves that entry unchanged. On its own it denies
// every namespace. That is what the ordering guard wants in both cases - a subject with no observed
// entry is denied, and a subject that already has one keeps exactly the scope its observed rules
// grant - so do not "fix" Combine to let a restricted marker clamp the others.

// HasAnyFilters reports whether the entry limits namespaces at all. An entry without filters is one
// the webhook has no opinion about: a rule with matchAny, or a rule without limits that also opens
// the system namespaces.
func (e *Entry) HasAnyFilters() bool {
	for _, selector := range e.NamespaceSelectors {
		if selector.MatchAny {
			return false
		}
	}
	if e.NamespaceFiltersAbsent {
		// limitNamespaces takes priority over allowAccessToSystemNamespaces: without limits only the
		// system-namespace gate is left, and that gate is open when the flag is set.
		return !e.AllowAccessToSystemNamespaces
	}
	for _, m := range e.LimitNamespaces {
		if m.MatchesEverything() {
			return !e.AllowAccessToSystemNamespaces
		}
	}
	return true
}

// Combine folds the entries of every subject a user matches (its name, its service account
// username, each of its groups) into one: the user may act wherever any of them may.
func Combine(entries []Entry) Entry {
	var out Entry
	for _, e := range entries {
		out.AllowAccessToSystemNamespaces = out.AllowAccessToSystemNamespaces || e.AllowAccessToSystemNamespaces
		out.NamespaceFiltersAbsent = out.NamespaceFiltersAbsent || e.NamespaceFiltersAbsent
		out.Quarantined = out.Quarantined || e.Quarantined
		out.LimitNamespaces = append(out.LimitNamespaces, e.LimitNamespaces...)
		out.NamespaceSelectors = append(out.NamespaceSelectors, e.NamespaceSelectors...)
	}
	return out
}

// Directory is the compiled set of rules of a cluster, keyed by subject. It is immutable once built;
// a Source replaces the whole directory on every change.
type Directory struct {
	users           map[string]Entry
	groups          map[string]Entry
	serviceAccounts map[string]Entry
	// rules are the names of the rules the directory was built from.
	rules map[string]struct{}
	// ruleSubjects maps a rule name to the subjects this directory's copy of it names. The ordering
	// guard needs more than the rule's name: a subject added to an existing rule reaches a consumer
	// as an updated ClusterRoleBinding while the rule keeps its name, so only the subject lists can
	// tell whether the binding is ahead of the rule.
	ruleSubjects map[string]map[string]struct{}
	// maxResourceVersion is the highest resourceVersion among the rules, the watermark a consumer
	// reports so that the lag behind the controller can be measured.
	maxResourceVersion uint64
	// quarantined are the names of the rules that did not compile fully.
	quarantined map[string]error
}

// Stats describes a build.
type Stats struct {
	Rules              int
	Subjects           int
	Quarantined        map[string]error
	MaxResourceVersion uint64
}

// Builder compiles rules into directories, keeping the compiled patterns between builds.
type Builder struct {
	cache *compileCache
}

// NewBuilder returns a Builder with an empty pattern cache.
func NewBuilder() *Builder {
	return &Builder{cache: newCompileCache()}
}

// Build compiles the rules into a Directory. A rule whose pattern or selector does not compile is
// quarantined: the rule still counts, its subjects still get an entry, but the broken filter is left
// out, so the entry is narrower than written, never wider. Build never fails: the consumers must
// keep serving the rules that are valid.
func (b *Builder) Build(rules []Rule) (*Directory, Stats) {
	d := &Directory{
		users:           make(map[string]Entry),
		groups:          make(map[string]Entry),
		serviceAccounts: make(map[string]Entry),
		rules:           make(map[string]struct{}, len(rules)),
		ruleSubjects:    make(map[string]map[string]struct{}, len(rules)),
		quarantined:     make(map[string]error),
	}
	inUse := make(map[string]struct{})

	for _, rule := range rules {
		d.rules[rule.Name] = struct{}{}
		if rv, err := strconv.ParseUint(rule.ResourceVersion, 10, 64); err == nil && rv > d.maxResourceVersion {
			d.maxResourceVersion = rv
		}

		var (
			matchers  []Matcher
			selectors []Selector
			broken    error
		)
		selectorApplied := rule.NamespaceSelector.Applied()
		switch {
		case rule.NamespaceSelector != nil:
			// A namespaceSelector wins over limitNamespaces and allowAccessToSystemNamespaces: the
			// entry opens exactly the namespaces the selector matches, system ones included, and
			// the patterns are dropped.
			//
			// An empty one - namespaceSelector: {} with no labelSelector - still displaces the
			// patterns, and what that leaves depends on whether there were any. With patterns, the
			// entry has a selector that matches nothing and no patterns left, so it opens no
			// namespace at all. Without them, the entry has no filters either way and opens every
			// non-system namespace, because an empty selector is then indistinguishable from an
			// absent one. Both are pinned in TestBuild_NamespaceSelector.
			sel := Selector{MatchAny: rule.NamespaceSelector.MatchAny}
			if selectorApplied {
				compiled, err := metav1.LabelSelectorAsSelector(rule.NamespaceSelector.LabelSelector)
				if err != nil {
					broken = fmt.Errorf("namespaceSelector: %w", err)
				} else {
					sel.Labels = compiled
				}
			}
			selectors = append(selectors, sel)
		default:
			for _, pattern := range rule.LimitNamespaces {
				m, err := b.cache.compile(pattern)
				if err != nil {
					broken = fmt.Errorf("limitNamespaces pattern %q: %w", pattern, err)
					continue
				}
				inUse[m.entry] = struct{}{}
				matchers = append(matchers, m)
			}
		}
		if broken != nil {
			d.quarantined[rule.Name] = broken
		}
		// A rule with neither patterns nor a label selector opens every non-system namespace.
		filtersAbsent := len(rule.LimitNamespaces) == 0 && !selectorApplied

		for _, subject := range rule.Subjects {
			kind, name := SubjectKey(subject)
			bucket := d.bucket(kind)
			if bucket == nil {
				continue
			}
			subjects := d.ruleSubjects[rule.Name]
			if subjects == nil {
				subjects = make(map[string]struct{}, len(rule.Subjects))
				d.ruleSubjects[rule.Name] = subjects
			}
			subjects[kind+"/"+name] = struct{}{}

			entry := bucket[name]
			entry.NamespaceFiltersAbsent = entry.NamespaceFiltersAbsent || filtersAbsent
			entry.Quarantined = entry.Quarantined || broken != nil
			if len(selectors) > 0 {
				entry.NamespaceSelectors = append(entry.NamespaceSelectors, selectors...)
			} else {
				entry.LimitNamespaces = append(entry.LimitNamespaces, matchers...)
				// allowAccessToSystemNamespaces belongs to the limitNamespaces form only.
				entry.AllowAccessToSystemNamespaces = entry.AllowAccessToSystemNamespaces || rule.AllowAccessToSystemNamespaces
			}
			bucket[name] = entry
		}
	}

	b.cache.evict(inUse)

	return d, Stats{
		Rules:    len(rules),
		Subjects: len(d.users) + len(d.groups) + len(d.serviceAccounts),
		// A copy, for the same reason Quarantined() returns one: the directory is immutable
		// once built, and Stats travels to observers and metric callbacks that must not be able
		// to change what the running decision reads.
		Quarantined:        maps.Clone(d.quarantined),
		MaxResourceVersion: d.maxResourceVersion,
	}
}

func (d *Directory) bucket(kind string) map[string]Entry {
	switch kind {
	case "User":
		return d.users
	case "Group":
		return d.groups
	case "ServiceAccount":
		return d.serviceAccounts
	}
	return nil
}

// Lookup returns the entries of every subject the user matches: its username as a User, its
// username as a ServiceAccount, and each of its groups. An empty result means no rule names the
// user. A nil Directory knows nothing.
func (d *Directory) Lookup(username string, groups []string) []Entry {
	if d == nil {
		return nil
	}
	var out []Entry
	if e, ok := d.users[username]; ok {
		out = append(out, e)
	}
	if e, ok := d.serviceAccounts[username]; ok {
		out = append(out, e)
	}
	for _, g := range groups {
		if e, ok := d.groups[g]; ok {
			out = append(out, e)
		}
	}
	return out
}

// KnowsRule reports whether the directory was built from a rule of that name.
func (d *Directory) KnowsRule(name string) bool {
	if d == nil {
		return false
	}
	_, ok := d.rules[name]
	return ok
}

// RuleCovers reports whether this directory's copy of the rule names the subject: the username as
// a User or as a ServiceAccount, or any of the groups. A rule the directory does not have at all
// covers nobody.
//
// This is the question the ordering guard has to ask, and asking only KnowsRule is not enough. The
// bindings of a rule and the rule itself reach a consumer over independent watches, so a consumer
// can see either one first, in two different ways:
//
//   - a whole new rule: its bindings may arrive first, and the rule name is unknown;
//   - a subject added to an existing rule: the controller updates the same bindings, which keep
//     their names, so the rule stays "known" while its observed copy still lacks the subject.
//
// In both cases the cluster-wide binding is in place before the scope that is meant to limit it, so
// the subject must be treated as restricted until the rule's own update lands.
func (d *Directory) RuleCovers(ruleName, username string, groups []string) bool {
	if d == nil {
		return false
	}
	subjects, ok := d.ruleSubjects[ruleName]
	if !ok {
		return false
	}
	if _, ok := subjects["User/"+username]; ok {
		return true
	}
	if _, ok := subjects["ServiceAccount/"+username]; ok {
		return true
	}
	for _, group := range groups {
		if _, ok := subjects["Group/"+group]; ok {
			return true
		}
	}
	return false
}

// Len is the number of rules the directory was built from.
func (d *Directory) Len() int {
	if d == nil {
		return 0
	}
	return len(d.rules)
}

// MaxResourceVersion is the highest resourceVersion of the rules the directory was built from.
func (d *Directory) MaxResourceVersion() uint64 {
	if d == nil {
		return 0
	}
	return d.maxResourceVersion
}

// Quarantined returns the rules that did not compile fully, with the reason.
func (d *Directory) Quarantined() map[string]error {
	if d == nil {
		return nil
	}
	if len(d.quarantined) == 0 {
		return nil
	}
	// A copy: the directory is immutable once built, and callers must not be able to change it.
	out := make(map[string]error, len(d.quarantined))
	for name, err := range d.quarantined {
		out[name] = err
	}
	return out
}
