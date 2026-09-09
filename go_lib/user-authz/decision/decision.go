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

// Package decision is the multi-tenancy answer to a request: the order in which the rules of a
// subject are found, folded and consulted, and the point at which CAR-independent RBAC may override
// the result.
//
// The rules package holds the facts - what a rule says, which namespaces an entry opens, whether a
// cluster-scoped request may proceed. This package holds the sequence. Both existed twice, once in
// the authorization webhook and once in permission-browser, and the two drifted: a resource that
// does not exist was reported as permitted by one and forbidden by the other, because the carve-out
// for it had been written into only one of them. What the API server enforces and what the UI
// reports are the same question, so they are now the same code, and a consumer supplies only what
// it alone can know: the labels of a namespace, what discovery says about a resource, and whether
// RBAC grants the request independently of any rule.
package decision

import (
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

// InternalErrorReason is the answer when the decision needed a fact that could not be fetched. It
// is deliberately vague: the operator gets the detail from the log, the caller does not.
const InternalErrorReason = "webhook: kubernetes api request error"

// Outcome is what the multi-tenancy layer has to say. It never allows: a rule limits where an
// access level applies, and whether the level grants the verb at all is RBAC's question.
type Outcome int

const (
	// NoOpinion leaves the request to the authorizers that follow.
	NoOpinion Outcome = iota
	// Deny ends the request.
	Deny
)

// Result is an Outcome and, when it is Deny, why.
type Result struct {
	Outcome Outcome
	Reason  string
}

// Denied reports whether the result ends the request.
func (r Result) Denied() bool { return r.Outcome == Deny }

// Request is the part of a request the multi-tenancy layer reads.
type Request struct {
	User      string
	Groups    []string
	Namespace string
	// Resource and APIGroup describe a cluster-scoped request; Resource is empty for one that is
	// not about a resource at all.
	Resource string
	APIGroup string
}

// RuleBindings answers which rules bind a subject through the ClusterRoleBindings
// user-authz-controller writes for them.
type RuleBindings interface {
	RulesFor(username string, groups []string) []string
}

// ResourceScopeFunc answers what discovery knows about a resource. An error means the question
// could not be asked, which is denied; a resource discovery reports as nonexistent is not an error,
// it is a scope with Absent set.
type ResourceScopeFunc func(apiGroup, resource string) (rules.ResourceScope, error)

// Sources are the facts the decision needs and the consumer holds.
type Sources struct {
	// Directory is the observed rules. Nil means they have not been read yet, which is not the
	// same as there being none: see Entries.
	Directory *rules.Directory
	// Bindings is required. Without it a subject nobody limits cannot be told from one whose rule
	// has not been observed.
	Bindings RuleBindings
	// NamespaceLabels resolves a namespace's labels for a namespaceSelector. Nil means selectors
	// match nothing.
	NamespaceLabels rules.NamespaceLabels
	// ResourceScope is consulted for cluster-scoped requests only. Nil denies them, which is the
	// fail-closed reading of "we cannot tell whether this resource is namespaced".
	ResourceScope ResourceScopeFunc
	// IndependentRBAC reports whether RBAC grants this request regardless of any rule: a
	// RoleBinding in the namespace, or a ClusterRoleBinding no rule owns. Such a grant exists on
	// its own and must not be denied here. Nil means no such grant.
	IndependentRBAC func() bool
	// Logf receives diagnostics; nil discards them.
	Logf func(format string, args ...interface{})
	// OnRestricted is called with the rule name when the ordering guard fires, so a consumer can
	// count or rate-limit its own reporting. Optional.
	OnRestricted func(username, rule string)
}

func (s Sources) logf(format string, args ...interface{}) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// Entries are the directory entries that apply to a subject, with the ordering guard applied.
//
// The guard: user-authz-controller writes the ClusterRoleBindings of a rule around the same time as
// the rule, and the rule and its bindings reach a consumer over independent watches, so the
// bindings can be ahead. When a binding says it binds the subject to a rule whose observed copy
// does not name that subject - the rule is unknown, or known but not yet updated - the subject gets
// a maximally restricted entry, so the cluster-wide binding never grants more than the rule will.
// It lifts by itself when the rule arrives.
//
// The restricted entry only bites a subject with no observed entry of its own: rules union, so an
// entry already observed stays as wide as it is. That is deliberate. An unobserved rule can only
// widen a subject's scope, and clamping what is already known would deny access the observed rules
// legitimately grant.
func Entries(src Sources, username string, groups []string) []rules.Entry {
	entries := src.Directory.Lookup(username, groups)

	if src.Bindings == nil {
		return entries
	}
	for _, rule := range src.Bindings.RulesFor(username, groups) {
		if src.Directory.RuleCovers(rule, username, groups) {
			continue
		}
		if src.OnRestricted != nil {
			src.OnRestricted(username, rule)
		}
		entries = append(entries, rules.Restricted())
		break
	}
	return entries
}

// Authorize answers a request.
func Authorize(req Request, src Sources) Result {
	entries := Entries(src, req.User, req.Groups)
	if len(entries) == 0 {
		// No rule names this subject and none is on its way, so multi-tenancy has nothing to say.
		return Result{Outcome: NoOpinion}
	}

	entry := rules.Combine(entries)

	switch {
	case req.Namespace != "":
		return namespaced(req, src, &entry)
	case req.Resource != "":
		return clusterScoped(req, src, &entry)
	default:
		// Not a resource request. The rules are about namespaces, so there is nothing to limit.
		return Result{Outcome: NoOpinion}
	}
}

func namespaced(req Request, src Sources, entry *rules.Entry) Result {
	if !entry.HasAnyFilters() {
		return Result{Outcome: NoOpinion}
	}

	allowed, err := rules.NamespaceAllowed(entry, req.Namespace, src.NamespaceLabels)
	if err != nil {
		// The lookup fails for a namespace that does not exist as well as for a genuine cache
		// problem. Neither may reach the caller: naming the missing namespace in the denial turned
		// the authorizer into an existence oracle for anyone a rule limits.
		src.logf("namespace selector check for %q failed: %v", req.Namespace, err)
	}
	if allowed {
		return Result{Outcome: NoOpinion}
	}

	if src.IndependentRBAC != nil && src.IndependentRBAC() {
		return Result{Outcome: NoOpinion}
	}
	return Result{Outcome: Deny, Reason: rules.NoNamespaceAccessReason}
}

func clusterScoped(req Request, src Sources, entry *rules.Entry) Result {
	if !entry.HasAnyFilters() {
		return Result{Outcome: NoOpinion}
	}

	if src.ResourceScope == nil {
		return Result{Outcome: Deny, Reason: InternalErrorReason}
	}
	scope, err := src.ResourceScope(req.APIGroup, req.Resource)
	if err != nil {
		src.logf("resource scope lookup for %s/%s failed: %v", req.APIGroup, req.Resource, err)
		return Result{Outcome: Deny, Reason: InternalErrorReason}
	}

	if !rules.ClusterScopedDenied(scope) {
		return Result{Outcome: NoOpinion}
	}
	// A cluster-wide grant that no rule owns is deliberate, and this layer does not take it away.
	if src.IndependentRBAC != nil && src.IndependentRBAC() {
		return Result{Outcome: NoOpinion}
	}
	return Result{Outcome: Deny, Reason: rules.NamespaceLimitedAccessReason}
}

// AccessType says how much of the cluster a subject can see, for callers that filter a list of
// namespaces rather than answer one request.
type AccessType int

const (
	// AllNamespaces: nothing limits the subject.
	AllNamespaces AccessType = iota
	// NoNamespaces: no rule names the subject and the caller does not treat it as privileged.
	NoNamespaces
	// Filtered: the returned entry decides each namespace.
	Filtered
)

// NamespaceAccess folds a subject's entries into the question "which namespaces may they see".
//
// privileged is the caller's own policy for a subject no rule names. An authorizer must answer
// NoOpinion there and let RBAC decide, but a report that filters a list has to choose, and the
// choice belongs to the caller rather than to this package.
func NamespaceAccess(src Sources, username string, groups []string, privileged bool) (AccessType, rules.Entry) {
	entries := Entries(src, username, groups)
	if len(entries) == 0 {
		if privileged {
			return AllNamespaces, rules.Entry{}
		}
		return NoNamespaces, rules.Entry{}
	}

	entry := rules.Combine(entries)
	if !entry.HasAnyFilters() {
		return AllNamespaces, entry
	}
	return Filtered, entry
}

// NamespaceAllowed reports whether a filter from NamespaceAccess opens the namespace.
func NamespaceAllowed(src Sources, entry *rules.Entry, namespace string) bool {
	if entry == nil {
		return true
	}
	allowed, err := rules.NamespaceAllowed(entry, namespace, src.NamespaceLabels)
	if err != nil {
		src.logf("namespace selector check for %q failed: %v", namespace, err)
	}
	return allowed
}
