/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
)

// standardVerbs is what a "*" verb rule expands to. It matches the verb set the
// console shows, so an expanded wildcard reads like an explicit grant.
var standardVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}

// protectedVerbs are the write verbs that the system-resource admission webhook
// restricts for non-superadmin subjects. RBAC alone cannot express that, so a
// report flags scopes where the difference matters.
var protectedVerbs = map[string]struct{}{
	"update":           {},
	"patch":            {},
	"delete":           {},
	"deletecollection": {},
}

// ReportLimits bounds the size of a report. A subject bound to cluster-admin
// matches every resource in discovery, so the output has to be capped to keep
// both the apiserver and the client responsive.
type ReportLimits struct {
	MaxNamespaces    int
	MaxResourceRows  int
	MaxSourcesPerRow int
}

// DefaultReportLimits are sized to fit the widest realistic grant (a wildcard
// rule over a full DKP discovery set) without truncating it.
var DefaultReportLimits = ReportLimits{
	MaxNamespaces:    500,
	MaxResourceRows:  5000,
	MaxSourcesPerRow: 50,
}

// SubjectAccessResolver answers "what is this subject allowed to do" by
// expanding the subject's bindings, instead of probing every
// resource/verb/namespace combination through the authorizer.
//
// Everything is served from informer caches, so a report costs one pass over
// ClusterRoleBindings plus one over RoleBindings, regardless of how many
// resources the cluster has.
type SubjectAccessResolver struct {
	roleLister               rbaclisters.RoleLister
	roleBindingLister        rbaclisters.RoleBindingLister
	clusterRoleLister        rbaclisters.ClusterRoleLister
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister
	scopeCache               *ResourceScopeCache
	mtEngine                 *multitenancy.Engine
	groupCatalog             *GroupCatalog
	limits                   ReportLimits
}

// NewSubjectAccessResolver creates a resolver. groupCatalog and mtEngine may be
// nil: without a catalog the report only uses the groups it was given, without
// the multi-tenancy engine no ClusterAuthorizationRule filtering is applied.
func NewSubjectAccessResolver(
	roleLister rbaclisters.RoleLister,
	roleBindingLister rbaclisters.RoleBindingLister,
	clusterRoleLister rbaclisters.ClusterRoleLister,
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister,
	scopeCache *ResourceScopeCache,
	mtEngine *multitenancy.Engine,
	groupCatalog *GroupCatalog,
) *SubjectAccessResolver {
	return &SubjectAccessResolver{
		roleLister:               roleLister,
		roleBindingLister:        roleBindingLister,
		clusterRoleLister:        clusterRoleLister,
		clusterRoleBindingLister: clusterRoleBindingLister,
		scopeCache:               scopeCache,
		mtEngine:                 mtEngine,
		groupCatalog:             groupCatalog,
		limits:                   DefaultReportLimits,
	}
}

// SubjectAccessRequest is the resolved input of a report.
type SubjectAccessRequest struct {
	// Subject is the identity to report on.
	Subject v1alpha1.SubjectReference
	// CallerGroups are the groups of the authenticated caller, used in self
	// mode where the token is the source of truth for group membership.
	CallerGroups []string
	// ExtraGroups are the groups explicitly requested in the spec.
	ExtraGroups []string
	// ResolveGroups enables Group catalog lookup for User subjects.
	ResolveGroups bool
	// Namespaces restricts the namespaced part of the report.
	Namespaces []string
	// ExpandWildcards expands wildcard rules against discovery.
	ExpandWildcards bool
}

// subjectIdentity is how the subject appears to RBAC: a user name (empty for a
// Group subject) plus the groups it carries.
type subjectIdentity struct {
	user   string
	groups []string
}

// matchingSubjects returns every binding subject entry that refers to the
// identity, deduplicated.
//
// All matches matter, not just the first: a binding may list both the user and
// one of its groups, and clients filter grants by how the subject matched to
// show access with and without group membership.
func (i subjectIdentity) matchingSubjects(subjects []rbacv1.Subject, defaultNamespace string) []v1alpha1.SubjectMatch {
	var matches []v1alpha1.SubjectMatch
	seen := make(map[v1alpha1.SubjectMatch]struct{}, len(subjects))

	for _, subject := range subjects {
		// A Group subject has no user name; an empty name must not match a
		// malformed binding entry that also has none.
		if i.user == "" && subject.Kind != rbacv1.GroupKind {
			continue
		}
		if !rbacadapter.SubjectMatches(subject, i.user, i.groups, defaultNamespace) {
			continue
		}

		match := v1alpha1.SubjectMatch{Kind: subject.Kind, Name: subject.Name}
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		matches = append(matches, match)
	}

	return matches
}

func (i subjectIdentity) userInfo() user.Info {
	return &user.DefaultInfo{Name: i.user, Groups: i.groups}
}

// Report builds the access report for the requested subject.
//
// A non-nil error is fatal (the report cannot be trusted); recoverable problems
// are reported through status.EvaluationError and status.Notes so a partial
// report is still useful.
func (r *SubjectAccessResolver) Report(ctx context.Context, req SubjectAccessRequest) (v1alpha1.SubjectAccessReportStatus, error) {
	if err := ctx.Err(); err != nil {
		return v1alpha1.SubjectAccessReportStatus{}, err
	}

	identity, resolvedGroups, notes, groupErr := r.resolveIdentity(ctx, req)

	status := v1alpha1.SubjectAccessReportStatus{
		Subject: v1alpha1.ResolvedSubject{
			Kind:      req.Subject.Kind,
			Name:      req.Subject.Name,
			Namespace: req.Subject.Namespace,
			Groups:    resolvedGroups,
		},
		Notes: notes,
	}

	var errs []error
	if groupErr != nil {
		errs = append(errs, groupErr)
	}

	report := newReportBuilder(r.limits, req.ExpandWildcards, r.scopeCache)

	if err := r.collectClusterRoleBindings(ctx, identity, report); err != nil {
		errs = append(errs, err)
	}
	if err := r.collectRoleBindings(ctx, identity, req.Namespaces, report); err != nil {
		errs = append(errs, err)
	}

	status.RoleAssignments = report.roleAssignments()

	mtNotes := r.applyMultitenancy(identity, report)
	status.Notes = append(status.Notes, mtNotes...)

	status.Scopes = report.scopes(r.superadminIndex(report))
	status.Truncated = report.truncated

	if err := errors.Join(errs...); err != nil {
		status.EvaluationError = err.Error()
		klog.Warningf("SubjectAccessReport: partial result for %s %q: %v", req.Subject.Kind, req.Subject.Name, err)
	}

	return status, nil
}

// resolveIdentity turns the requested subject into the identity RBAC matches
// against: the user name plus every group the subject carries.
func (r *SubjectAccessResolver) resolveIdentity(ctx context.Context, req SubjectAccessRequest) (subjectIdentity, []v1alpha1.ResolvedGroup, []string, error) {
	var (
		identity subjectIdentity
		groups   []v1alpha1.ResolvedGroup
		notes    []string
		groupErr error
	)

	addGroup := func(name, source string, via []string) {
		if name == "" {
			return
		}
		for _, existing := range groups {
			if existing.Name == name {
				return
			}
		}
		groups = append(groups, v1alpha1.ResolvedGroup{Name: name, Source: source, Via: via})
	}

	switch req.Subject.Kind {
	case v1alpha1.SubjectKindGroup:
		// A Group report answers "what does this group grant", so only the
		// group itself counts: pseudo-groups belong to the users in it.
		identity.user = ""
		addGroup(req.Subject.Name, v1alpha1.GroupSourceExplicit, nil)

	case v1alpha1.SubjectKindServiceAccount:
		identity.user = fmt.Sprintf("system:serviceaccount:%s:%s", req.Subject.Namespace, req.Subject.Name)
		addGroup("system:serviceaccounts", v1alpha1.GroupSourceImplicit, nil)
		addGroup("system:serviceaccounts:"+req.Subject.Namespace, v1alpha1.GroupSourceImplicit, nil)
		addGroup(authenticatedGroup, v1alpha1.GroupSourceImplicit, nil)

	default:
		identity.user = req.Subject.Name
		addGroup(authenticatedGroup, v1alpha1.GroupSourceImplicit, nil)
	}

	for _, group := range req.CallerGroups {
		addGroup(group, v1alpha1.GroupSourceCaller, nil)
	}
	for _, group := range req.ExtraGroups {
		addGroup(group, v1alpha1.GroupSourceExplicit, nil)
	}

	if req.ResolveGroups && req.Subject.Kind == v1alpha1.SubjectKindUser {
		switch {
		case !r.groupCatalog.Available():
			notes = append(notes, "group catalog is unavailable: only the groups passed in the request were taken into account")
		default:
			resolved, err := r.groupCatalog.ResolveUserGroups(ctx, req.Subject.Name)
			if err != nil {
				groupErr = fmt.Errorf("resolving groups of user %q: %w", req.Subject.Name, err)
				notes = append(notes, "group membership could not be resolved: the report may miss grants made to groups")
			}
			for _, group := range resolved {
				addGroup(group.Name, group.Source, group.Via)
			}
		}
	}

	identity.groups = make([]string, 0, len(groups))
	for _, group := range groups {
		identity.groups = append(identity.groups, group.Name)
	}

	return identity, groups, notes, groupErr
}

// authenticatedGroup is attached to every authenticated request, so a report
// that ignored it would miss grants made to all logged-in users.
const authenticatedGroup = "system:authenticated"

// collectClusterRoleBindings expands every ClusterRoleBinding that applies to
// the subject into the cluster-wide scope.
func (r *SubjectAccessResolver) collectClusterRoleBindings(ctx context.Context, identity subjectIdentity, report *reportBuilder) error {
	bindings, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("listing ClusterRoleBindings: %w", err)
	}

	var errs []error
	for _, binding := range bindings {
		if err := ctx.Err(); err != nil {
			return err
		}

		matches := identity.matchingSubjects(binding.Subjects, "")
		if len(matches) == 0 {
			continue
		}

		role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("getting ClusterRole %q: %w", binding.RoleRef.Name, err))
			}
			klog.V(5).Infof("SubjectAccessReport: failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
			continue
		}

		grant := grantOrigin{
			bindingKind:      "ClusterRoleBinding",
			bindingName:      binding.Name,
			roleKind:         "ClusterRole",
			roleName:         role.Name,
			role:             DescribeRole(&role.ObjectMeta, role.Name),
			matches:          matches,
			carManagedGlobal: rbacadapter.IsCARManagedClusterRoleBinding(binding),
		}

		report.addClusterGrant(grant, role.Rules)
	}

	return errors.Join(errs...)
}

// collectRoleBindings expands the RoleBindings that apply to the subject into
// per-namespace scopes.
func (r *SubjectAccessResolver) collectRoleBindings(ctx context.Context, identity subjectIdentity, onlyNamespaces []string, report *reportBuilder) error {
	bindings, err := r.roleBindingLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("listing RoleBindings: %w", err)
	}

	var wanted map[string]struct{}
	if len(onlyNamespaces) > 0 {
		wanted = make(map[string]struct{}, len(onlyNamespaces))
		for _, namespace := range onlyNamespaces {
			wanted[namespace] = struct{}{}
		}
	}

	var errs []error
	for _, binding := range bindings {
		if err := ctx.Err(); err != nil {
			return err
		}

		if wanted != nil {
			if _, ok := wanted[binding.Namespace]; !ok {
				continue
			}
		}

		matches := identity.matchingSubjects(binding.Subjects, binding.Namespace)
		if len(matches) == 0 {
			continue
		}

		rules, roleMeta, err := r.rulesOf(binding)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, err)
			}
			klog.V(5).Infof("SubjectAccessReport: failed to resolve %s %q for RoleBinding %s/%s: %v",
				binding.RoleRef.Kind, binding.RoleRef.Name, binding.Namespace, binding.Name, err)
			continue
		}

		grant := grantOrigin{
			bindingKind:      "RoleBinding",
			bindingName:      binding.Name,
			bindingNamespace: binding.Namespace,
			roleKind:         binding.RoleRef.Kind,
			roleName:         binding.RoleRef.Name,
			role:             DescribeRole(roleMeta, binding.RoleRef.Name),
			matches:          matches,
		}

		report.addNamespaceGrant(binding.Namespace, grant, rules)
	}

	return errors.Join(errs...)
}

// rulesOf resolves the rules a RoleBinding grants, following either a
// namespaced Role or a ClusterRole.
func (r *SubjectAccessResolver) rulesOf(binding *rbacv1.RoleBinding) ([]rbacv1.PolicyRule, *metav1.ObjectMeta, error) {
	if binding.RoleRef.Kind == "ClusterRole" {
		role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("getting ClusterRole %q: %w", binding.RoleRef.Name, err)
		}
		return role.Rules, &role.ObjectMeta, nil
	}

	role, err := r.roleLister.Roles(binding.Namespace).Get(binding.RoleRef.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("getting Role %q/%q: %w", binding.Namespace, binding.RoleRef.Name, err)
	}

	return role.Rules, &role.ObjectMeta, nil
}

// superadminIndex reports where the subject holds a superadmin role, which the
// system-resource admission webhook lets through.
func (r *SubjectAccessResolver) superadminIndex(report *reportBuilder) superadminScopes {
	index := superadminScopes{namespaces: map[string]struct{}{}}

	for _, assignment := range report.assignments {
		if !IsSuperadminRole(assignment.Role) {
			continue
		}
		if assignment.BindingKind == "ClusterRoleBinding" {
			index.cluster = true
			continue
		}
		index.namespaces[assignment.Namespace] = struct{}{}
	}

	return index
}

type superadminScopes struct {
	cluster    bool
	namespaces map[string]struct{}
}

func (s superadminScopes) covers(namespace string) bool {
	if s.cluster {
		return true
	}
	_, ok := s.namespaces[namespace]

	return ok
}

// grantOrigin is the provenance shared by every row a single binding produces.
type grantOrigin struct {
	bindingKind      string
	bindingName      string
	bindingNamespace string
	roleKind         string
	roleName         string
	role             v1alpha1.RoleDescriptor
	matches          []v1alpha1.SubjectMatch
	// carManagedGlobal marks a ClusterRoleBinding rendered from a
	// ClusterAuthorizationRule: cluster-wide by construction, but scoped by the
	// rule's namespace limits.
	carManagedGlobal bool
}

func (g grantOrigin) sources(verbs []string) []v1alpha1.AccessSource {
	sources := make([]v1alpha1.AccessSource, 0, len(g.matches))
	for _, match := range g.matches {
		sources = append(sources, v1alpha1.AccessSource{
			Verbs:            verbs,
			BindingKind:      g.bindingKind,
			BindingName:      g.bindingName,
			BindingNamespace: g.bindingNamespace,
			RoleKind:         g.roleKind,
			RoleName:         g.roleName,
			MatchedBy:        match,
			Role:             g.role,
		})
	}

	return sources
}

// expandVerbs turns a rule's verbs into concrete ones, replacing "*" with the
// standard set while keeping non-standard verbs (impersonate, escalate, bind,
// use) that a rule names explicitly.
func expandVerbs(ruleVerbs []string) []string {
	verbs := make([]string, 0, len(ruleVerbs))
	for _, verb := range ruleVerbs {
		if verb == "*" {
			verbs = append(verbs, standardVerbs...)
			continue
		}
		verbs = append(verbs, verb)
	}

	return sortVerbs(verbs)
}

// sortVerbs deduplicates and orders verbs by the usual read-then-write reading
// order, with any extra verbs appended alphabetically.
func sortVerbs(verbs []string) []string {
	seen := make(map[string]struct{}, len(verbs))
	ordered := make([]string, 0, len(verbs))

	for _, verb := range verbs {
		if _, ok := seen[verb]; ok {
			continue
		}
		seen[verb] = struct{}{}
	}

	for _, verb := range standardVerbs {
		if _, ok := seen[verb]; ok {
			ordered = append(ordered, verb)
			delete(seen, verb)
		}
	}
	extra := make([]string, 0, len(seen))
	for verb := range seen {
		extra = append(extra, verb)
	}
	slices.Sort(extra)

	return append(ordered, extra...)
}

func hasProtectedVerb(verbs []string) bool {
	for _, verb := range verbs {
		if _, ok := protectedVerbs[verb]; ok {
			return true
		}
	}

	return false
}

func joinResourceNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	sorted := slices.Clone(names)
	slices.Sort(sorted)

	return strings.Join(sorted, ",")
}
