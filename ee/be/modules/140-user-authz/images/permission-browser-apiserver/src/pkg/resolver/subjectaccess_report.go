/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// reportBuilder accumulates the expanded grants of a subject, one bucket for
// the cluster-wide scope and one per namespace, and turns them into the API
// representation.
type reportBuilder struct {
	limits          ReportLimits
	expandWildcards bool
	scopeCache      *ResourceScopeCache

	cluster     *scopeAccumulator
	namespaces  map[string]*scopeAccumulator
	assignments []v1alpha1.RoleAssignment

	rowCount  int
	truncated bool
}

func newReportBuilder(limits ReportLimits, expandWildcards bool, scopeCache *ResourceScopeCache) *reportBuilder {
	return &reportBuilder{
		limits:          limits,
		expandWildcards: expandWildcards,
		scopeCache:      scopeCache,
		cluster:         newScopeAccumulator(),
		namespaces:      map[string]*scopeAccumulator{},
	}
}

type scopeAccumulator struct {
	resources    map[resourceKey]*resourceAccumulator
	nonResources map[string]*nonResourceAccumulator
	// carOnly is true while every grant in this scope came from a
	// ClusterAuthorizationRule-managed binding, which multi-tenancy may narrow.
	carOnly bool
	// anyCAR is true when at least one such binding contributed.
	anyCAR bool
	seeded bool
}

func newScopeAccumulator() *scopeAccumulator {
	return &scopeAccumulator{
		resources:    map[resourceKey]*resourceAccumulator{},
		nonResources: map[string]*nonResourceAccumulator{},
	}
}

func (s *scopeAccumulator) noteOrigin(origin grantOrigin) {
	s.anyCAR = s.anyCAR || origin.carManagedGlobal

	if !s.seeded {
		s.carOnly = origin.carManagedGlobal
		s.seeded = true
		return
	}
	s.carOnly = s.carOnly && origin.carManagedGlobal
}

func (s *scopeAccumulator) empty() bool {
	return len(s.resources) == 0 && len(s.nonResources) == 0
}

type resourceKey struct {
	group         string
	resource      string
	resourceNames string
}

type resourceAccumulator struct {
	group           string
	resource        string
	resourceNames   []string
	viaWildcard     bool
	viaVerbWildcard bool
	verbs           map[string]struct{}
	sources         map[sourceKey]*sourceAccumulator
}

type nonResourceAccumulator struct {
	path            string
	viaVerbWildcard bool
	verbs           map[string]struct{}
	sources         map[sourceKey]*sourceAccumulator
}

// sourceKey identifies a provenance entry. The binding namespace is left out on
// purpose: identical bindings fanned out into several namespaces (as project
// role bindings are) must collapse into one source so the namespaces can be
// reported together.
type sourceKey struct {
	bindingKind string
	bindingName string
	roleKind    string
	roleName    string
	matchKind   string
	matchName   string
}

type sourceAccumulator struct {
	source          v1alpha1.AccessSource
	verbs           map[string]struct{}
	viaVerbWildcard bool
}

func (b *reportBuilder) roleAssignments() []v1alpha1.RoleAssignment {
	assignments := slices.Clone(b.assignments)
	slices.SortFunc(assignments, func(x, y v1alpha1.RoleAssignment) int {
		if cmp := strings.Compare(x.Namespace, y.Namespace); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(x.BindingKind, y.BindingKind); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(x.BindingName, y.BindingName); cmp != 0 {
			return cmp
		}
		return strings.Compare(x.RoleName, y.RoleName)
	})

	return assignments
}

// addClusterGrant records a ClusterRoleBinding grant. Cluster-wide grants apply
// in every namespace, so they form a single scope of their own rather than
// being repeated for each namespace.
func (b *reportBuilder) addClusterGrant(origin grantOrigin, rules []rbacv1.PolicyRule) {
	b.recordAssignment(origin, "")
	b.cluster.noteOrigin(origin)
	b.expand(b.cluster, origin, rules, false)
}

// addNamespaceGrant records a RoleBinding grant in one namespace.
func (b *reportBuilder) addNamespaceGrant(namespace string, origin grantOrigin, rules []rbacv1.PolicyRule) {
	b.recordAssignment(origin, namespace)

	scope, ok := b.namespaces[namespace]
	if !ok {
		if len(b.namespaces) >= b.limits.MaxNamespaces {
			b.truncated = true
			return
		}
		scope = newScopeAccumulator()
		b.namespaces[namespace] = scope
	}

	scope.noteOrigin(origin)
	b.expand(scope, origin, rules, true)
}

func (b *reportBuilder) recordAssignment(origin grantOrigin, namespace string) {
	for _, match := range origin.matches {
		if b.limits.MaxRoleAssignments > 0 && len(b.assignments) >= b.limits.MaxRoleAssignments {
			b.truncated = true

			return
		}
		b.assignments = append(b.assignments, v1alpha1.RoleAssignment{
			BindingKind: origin.bindingKind,
			BindingName: origin.bindingName,
			Namespace:   namespace,
			RoleKind:    origin.roleKind,
			RoleName:    origin.roleName,
			MatchedBy:   match,
			Role:        origin.role,
		})
	}
}

// expand turns policy rules into resource rows. namespaced restricts the
// expansion to namespaced resources, since a RoleBinding cannot grant access to
// cluster-scoped objects.
func (b *reportBuilder) expand(scope *scopeAccumulator, origin grantOrigin, rules []rbacv1.PolicyRule, namespaced bool) {
	for _, rule := range rules {
		verbs, viaVerbWildcard := expandVerbs(rule.Verbs)
		if len(verbs) == 0 {
			continue
		}

		// Non-resource URLs are cluster-scoped by nature: a RoleBinding never
		// grants them, matching upstream RBAC.
		if !namespaced && len(rule.NonResourceURLs) > 0 {
			for _, path := range rule.NonResourceURLs {
				b.addNonResourceRow(scope, origin, path, verbs, viaVerbWildcard)
			}
		}

		if len(rule.Resources) == 0 {
			continue
		}

		for _, target := range b.targetsOf(rule, namespaced) {
			b.addResourceRow(scope, origin, target, rule.ResourceNames, verbs, viaVerbWildcard)
		}
	}
}

// isKnownClusterScoped reports whether discovery has seen this resource and says
// it is cluster-scoped. An unknown resource answers false.
func (b *reportBuilder) isKnownClusterScoped(group, resource string) bool {
	namespaced, known := b.scopeCache.Scope(group, resource)

	return known && !namespaced
}

// resourceTarget is one row a rule expands to.
type resourceTarget struct {
	group       string
	resource    string
	viaWildcard bool
}

// targetsOf resolves the (group, resource) pairs a rule covers. Wildcards are
// expanded against the discovery snapshot so that a "*" rule reads like the
// concrete access it grants; without expansion (or without discovery data) the
// wildcard is reported as-is.
func (b *reportBuilder) targetsOf(rule rbacv1.PolicyRule, namespaced bool) []resourceTarget {
	hasWildcard := slices.Contains(rule.APIGroups, "*") || slices.Contains(rule.Resources, "*")

	if hasWildcard && b.expandWildcards && b.scopeCache != nil {
		matched := b.scopeCache.ResourcesMatching(rule.APIGroups, rule.Resources)
		targets := make([]resourceTarget, 0, len(matched))
		for _, resource := range matched {
			if namespaced && !resource.Namespaced {
				continue
			}
			targets = append(targets, resourceTarget{group: resource.Group, resource: resource.Resource, viaWildcard: true})
		}
		return targets
	}

	targets := make([]resourceTarget, 0, len(rule.APIGroups)*len(rule.Resources))
	for _, group := range rule.APIGroups {
		for _, resource := range rule.Resources {
			if namespaced && !hasWildcard && b.scopeCache != nil {
				// Skip cluster-scoped resources named by a RoleBinding: the
				// grant exists in RBAC but can never be exercised.
				//
				// Only a resource discovery positively calls cluster-scoped is
				// dropped. One the snapshot has never heard of is kept: the
				// snapshot goes incomplete whenever an aggregated APIService
				// flaps, and an audit report that omits a grant is a worse
				// answer than one that lists a grant nobody can exercise.
				//
				// A subresource is judged by its base. Discovery never lists
				// subresources -- ServerPreferredResources skips every name
				// holding a slash -- so asking the snapshot about "nodes/proxy"
				// answers "unknown" forever, and the row would survive although
				// it can no more be exercised in a namespace than "nodes".
				base, _, _ := strings.Cut(resource, "/")
				if b.isKnownClusterScoped(group, base) {
					continue
				}
			}
			targets = append(targets, resourceTarget{
				group:       group,
				resource:    resource,
				viaWildcard: group == "*" || resource == "*",
			})
		}
	}

	return targets
}

func (b *reportBuilder) addResourceRow(scope *scopeAccumulator, origin grantOrigin, target resourceTarget, resourceNames, verbs []string, viaVerbWildcard bool) {
	key := resourceKey{
		group:         target.group,
		resource:      target.resource,
		resourceNames: joinResourceNames(resourceNames),
	}

	row, ok := scope.resources[key]
	if !ok {
		if b.rowCount >= b.limits.MaxResourceRows {
			b.truncated = true
			return
		}
		b.rowCount++
		row = &resourceAccumulator{
			group:         target.group,
			resource:      target.resource,
			resourceNames: slices.Clone(resourceNames),
			verbs:         map[string]struct{}{},
			sources:       map[sourceKey]*sourceAccumulator{},
		}
		scope.resources[key] = row
	}

	row.viaWildcard = row.viaWildcard || target.viaWildcard
	row.viaVerbWildcard = row.viaVerbWildcard || viaVerbWildcard
	for _, verb := range verbs {
		row.verbs[verb] = struct{}{}
	}

	b.addSources(row.sources, origin, verbs, viaVerbWildcard)
}

func (b *reportBuilder) addNonResourceRow(scope *scopeAccumulator, origin grantOrigin, path string, verbs []string, viaVerbWildcard bool) {
	row, ok := scope.nonResources[path]
	if !ok {
		if b.rowCount >= b.limits.MaxResourceRows {
			b.truncated = true
			return
		}
		b.rowCount++
		row = &nonResourceAccumulator{
			path:    path,
			verbs:   map[string]struct{}{},
			sources: map[sourceKey]*sourceAccumulator{},
		}
		scope.nonResources[path] = row
	}

	row.viaVerbWildcard = row.viaVerbWildcard || viaVerbWildcard
	for _, verb := range verbs {
		row.verbs[verb] = struct{}{}
	}

	b.addSources(row.sources, origin, verbs, viaVerbWildcard)
}

func (b *reportBuilder) addSources(sources map[sourceKey]*sourceAccumulator, origin grantOrigin, verbs []string, viaVerbWildcard bool) {
	for _, source := range origin.sources(nil) {
		key := sourceKey{
			bindingKind: source.BindingKind,
			bindingName: source.BindingName,
			roleKind:    source.RoleKind,
			roleName:    source.RoleName,
			matchKind:   source.MatchedBy.Kind,
			matchName:   source.MatchedBy.Name,
		}

		accumulated, ok := sources[key]
		if !ok {
			if len(sources) >= b.limits.MaxSourcesPerRow {
				b.truncated = true
				return
			}
			accumulated = &sourceAccumulator{source: source, verbs: map[string]struct{}{}}
			sources[key] = accumulated
		}

		accumulated.viaVerbWildcard = accumulated.viaVerbWildcard || viaVerbWildcard
		for _, verb := range verbs {
			accumulated.verbs[verb] = struct{}{}
		}
	}
}

// scopes renders the accumulated grants: the cluster-wide scope first, then one
// scope per set of namespaces that share identical local access.
func (b *reportBuilder) scopes(superadmin superadminScopes) []v1alpha1.AccessScope {
	var scopes []v1alpha1.AccessScope

	if !b.cluster.empty() {
		scope := v1alpha1.AccessScope{
			Cluster:          true,
			Resources:        renderResources(b.cluster, true),
			NonResourceRules: renderNonResources(b.cluster),
		}
		scope.Caveat = v1alpha1.AccessCaveat{
			ProtectedVerbs: scopeHasProtectedVerb(scope),
			Superadmin:     superadmin.cluster,
		}
		scopes = append(scopes, scope)
	}

	for _, group := range b.groupNamespaces() {
		representative := b.namespaces[group.namespaces[0]]
		// Binding namespaces are only meaningful when the scope is a single
		// namespace; otherwise the scope's namespace list already says where
		// the identical bindings live.
		scope := v1alpha1.AccessScope{
			Namespaces:       group.namespaces,
			Resources:        renderResources(representative, len(group.namespaces) == 1),
			NonResourceRules: renderNonResources(representative),
		}
		scope.Caveat = namespaceCaveat(scope, group.namespaces, superadmin)
		scopes = append(scopes, scope)
	}

	return scopes
}

type namespaceGroup struct {
	signature  string
	namespaces []string
}

// groupNamespaces merges namespaces whose access is identical, so a project
// with a dozen namespaces reads as one section instead of a dozen copies.
func (b *reportBuilder) groupNamespaces() []namespaceGroup {
	bySignature := map[string][]string{}

	for namespace, scope := range b.namespaces {
		if scope.empty() {
			continue
		}
		signature := scopeSignature(scope)
		bySignature[signature] = append(bySignature[signature], namespace)
	}

	groups := make([]namespaceGroup, 0, len(bySignature))
	for signature, namespaces := range bySignature {
		slices.Sort(namespaces)
		groups = append(groups, namespaceGroup{signature: signature, namespaces: namespaces})
	}

	slices.SortFunc(groups, func(x, y namespaceGroup) int {
		return strings.Compare(x.namespaces[0], y.namespaces[0])
	})

	return groups
}

// scopeSignature describes the access of a scope well enough to tell whether
// two namespaces can be reported together: same rows, same verbs and the same
// roles behind them.
func scopeSignature(scope *scopeAccumulator) string {
	parts := make([]string, 0, len(scope.resources)+len(scope.nonResources))

	for key, row := range scope.resources {
		parts = append(parts, strings.Join([]string{
			"r", key.group, key.resource, key.resourceNames,
			strings.Join(sortVerbs(keysOf(row.verbs)), ","),
			fmt.Sprintf("%t/%t", row.viaWildcard, row.viaVerbWildcard),
			sourcesSignature(row.sources),
		}, "|"))
	}
	for path, row := range scope.nonResources {
		parts = append(parts, strings.Join([]string{
			"n", path,
			strings.Join(sortVerbs(keysOf(row.verbs)), ","),
			fmt.Sprintf("%t", row.viaVerbWildcard),
			sourcesSignature(row.sources),
		}, "|"))
	}

	slices.Sort(parts)

	return strings.Join(parts, ";")
}

func sourcesSignature(sources map[sourceKey]*sourceAccumulator) string {
	parts := make([]string, 0, len(sources))
	for key := range sources {
		parts = append(parts, strings.Join([]string{
			key.bindingKind, key.bindingName, key.roleKind, key.roleName, key.matchKind, key.matchName,
		}, "/"))
	}
	slices.Sort(parts)

	return strings.Join(parts, "+")
}

func renderResources(scope *scopeAccumulator, keepBindingNamespace bool) []v1alpha1.ResourceAccess {
	rows := make([]v1alpha1.ResourceAccess, 0, len(scope.resources))

	for _, row := range scope.resources {
		rows = append(rows, v1alpha1.ResourceAccess{
			Group:           row.group,
			Resource:        row.resource,
			Verbs:           sortVerbs(keysOf(row.verbs)),
			ViaWildcard:     row.viaWildcard,
			ViaVerbWildcard: row.viaVerbWildcard,
			ResourceNames:   sortedCopy(row.resourceNames),
			Sources:         renderSources(row.sources, keepBindingNamespace),
		})
	}

	slices.SortFunc(rows, func(x, y v1alpha1.ResourceAccess) int {
		if cmp := strings.Compare(x.Group, y.Group); cmp != 0 {
			return cmp
		}
		return strings.Compare(x.Resource, y.Resource)
	})

	return rows
}

func renderNonResources(scope *scopeAccumulator) []v1alpha1.NonResourceAccess {
	if len(scope.nonResources) == 0 {
		return nil
	}

	rows := make([]v1alpha1.NonResourceAccess, 0, len(scope.nonResources))
	for _, row := range scope.nonResources {
		rows = append(rows, v1alpha1.NonResourceAccess{
			Path:            row.path,
			Verbs:           sortVerbs(keysOf(row.verbs)),
			ViaVerbWildcard: row.viaVerbWildcard,
			Sources:         renderSources(row.sources, false),
		})
	}

	slices.SortFunc(rows, func(x, y v1alpha1.NonResourceAccess) int {
		return strings.Compare(x.Path, y.Path)
	})

	return rows
}

func renderSources(sources map[sourceKey]*sourceAccumulator, keepBindingNamespace bool) []v1alpha1.AccessSource {
	rendered := make([]v1alpha1.AccessSource, 0, len(sources))

	for _, accumulated := range sources {
		source := accumulated.source
		source.Verbs = sortVerbs(keysOf(accumulated.verbs))
		source.ViaVerbWildcard = accumulated.viaVerbWildcard
		if !keepBindingNamespace {
			source.BindingNamespace = ""
		}
		rendered = append(rendered, source)
	}

	slices.SortFunc(rendered, func(x, y v1alpha1.AccessSource) int {
		if cmp := strings.Compare(x.BindingKind, y.BindingKind); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(x.RoleName, y.RoleName); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(x.BindingName, y.BindingName); cmp != 0 {
			return cmp
		}
		return strings.Compare(x.MatchedBy.Name, y.MatchedBy.Name)
	})

	return rendered
}

func scopeHasProtectedVerb(scope v1alpha1.AccessScope) bool {
	for _, row := range scope.Resources {
		if hasProtectedVerb(row.Verbs) {
			return true
		}
	}

	return false
}

// namespaceCaveat splits the scope's namespaces into those where the subject is
// superadmin (and therefore bypasses the system-resource webhook) and those
// where the webhook narrows what RBAC seems to allow.
func namespaceCaveat(scope v1alpha1.AccessScope, namespaces []string, superadmin superadminScopes) v1alpha1.AccessCaveat {
	caveat := v1alpha1.AccessCaveat{ProtectedVerbs: scopeHasProtectedVerb(scope)}
	if !caveat.ProtectedVerbs {
		return caveat
	}

	for _, namespace := range namespaces {
		if superadmin.covers(namespace) {
			caveat.SuperadminNamespaces = append(caveat.SuperadminNamespaces, namespace)
			continue
		}
		caveat.RestrictedNamespaces = append(caveat.RestrictedNamespaces, namespace)
	}

	caveat.Superadmin = len(caveat.RestrictedNamespaces) == 0

	return caveat
}

func keysOf(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	return keys
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)

	return sorted
}
