/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// The two role models a report can cover.
const (
	// RoleModelPrimary is the scope-based model: d8:namespace:*, d8:project:*,
	// d8:subsystem:<subsystem>:*, d8:system:*.
	RoleModelPrimary = "primary"
	// RoleModelLegacy is the access levels of ClusterAuthorizationRule.
	RoleModelLegacy = "legacy"
)

// Where a role says what it is.
const (
	roleKindRole       = "role"
	roleKindCustomRole = "custom-role"
	// roleKindCapability marks the building blocks a role aggregates. A
	// capability belongs to a module and names that module's resources.
	roleKindCapability = "capability"
	// labelModule names the module an object belongs to; every Deckhouse
	// object carries it.
	labelModule = "module"

	annotationReplacedBy  = "rbac.deckhouse.io/deprecated-replaced-by"
	annotationAccessLevel = "user-authz.deckhouse.io/access-level"
)

// legacyAccessLevels are the levels of ClusterAuthorizationRule, in the order
// they include one another. A ClusterRole annotated for one level is bound to
// every level after it -- the fan-out hook walks this same order through a
// fallthrough -- so the order here is not cosmetic: it is what makes the legacy
// report say that Admin also carries everything Editor carries.
var legacyAccessLevels = []string{"User", "PrivilegedUser", "Editor", "Admin", "ClusterEditor", "ClusterAdmin", "SuperAdmin"}

// legacyBaseRole is the ClusterRole a ClusterAuthorizationRule binds for a
// level, alongside the annotated ones. The template derives it with kebabcase,
// which for these seven names is a plain lowercase-with-dashes.
var legacyBaseRole = map[string]string{
	"User":           "user-authz:user",
	"PrivilegedUser": "user-authz:privileged-user",
	"Editor":         "user-authz:editor",
	"Admin":          "user-authz:admin",
	"ClusterEditor":  "user-authz:cluster-editor",
	"ClusterAdmin":   "user-authz:cluster-admin",
	"SuperAdmin":     "user-authz:super-admin",
}

// namespacedScopes are the scopes whose roles only ever apply inside a
// namespace: the binding webhook refuses a ClusterRoleBinding carrying one.
var namespacedScopes = map[string]struct{}{
	"namespace": {},
	"project":   {},
}

// RoleAccessResolver answers "what does this role grant".
//
// It is the catalogue counterpart of SubjectAccessResolver and deliberately
// shares its expansion: the rows of both reports are produced by the same
// reportBuilder, so a wildcard reads the same way in the document about a role
// and in the document about a person. Two implementations of that would drift,
// and a compliance export that contradicts the access report is worse than no
// export at all.
type RoleAccessResolver struct {
	clusterRoleLister rbaclisters.ClusterRoleLister
	scopeCache        *ResourceScopeCache
	moduleIndex       *ModuleIndex
	limits            ReportLimits
	// now is the clock, replaced in tests so a report has a fixed timestamp.
	now func() time.Time
}

// NewRoleAccessResolver creates a resolver. scopeCache may be nil: without a
// discovery snapshot wildcards are reported as written instead of expanded.
func NewRoleAccessResolver(clusterRoleLister rbaclisters.ClusterRoleLister, scopeCache *ResourceScopeCache) *RoleAccessResolver {
	return &RoleAccessResolver{
		clusterRoleLister: clusterRoleLister,
		scopeCache:        scopeCache,
		limits:            DefaultReportLimits,
		now:               time.Now,
	}
}

// WithModuleIndex attributes the inventory to the modules that ship it. Without
// the index the inventory is still reported, just without module names: the CRDs
// may be unreadable, and a coverage report without grouping beats none.
func (r *RoleAccessResolver) WithModuleIndex(index *ModuleIndex) *RoleAccessResolver {
	r.moduleIndex = index

	return r
}

// RoleAccessRequest is the resolved input of a role report.
type RoleAccessRequest struct {
	// Model is primary or legacy.
	Model string
	// Names restricts the report to the named roles (or access levels).
	Names []string
	// Scopes restricts the primary model to these scopes.
	Scopes []string
	// AccessLevels restricts the legacy model to these levels.
	AccessLevels []string
	// ExcludeCustom leaves out the roles created in this cluster.
	ExcludeCustom bool
	// ExpandWildcards expands wildcard rules against discovery.
	ExpandWildcards bool
	// IncludeComposition reports what each role is assembled from.
	IncludeComposition bool
	// IncludeInventory adds every resource of the cluster to the report.
	IncludeInventory bool
}

// Report builds the catalogue for the requested roles.
func (r *RoleAccessResolver) Report(_ context.Context, req RoleAccessRequest) (v1alpha1.RoleAccessReportStatus, error) {
	roles, err := r.clusterRoleLister.List(labels.Everything())
	if err != nil {
		return v1alpha1.RoleAccessReportStatus{}, fmt.Errorf("list cluster roles: %w", err)
	}

	index := newRoleIndex(roles)

	var (
		reported []v1alpha1.RoleAccess
		notes    []string
	)

	switch req.Model {
	case RoleModelLegacy:
		reported = r.legacyRoles(index, req)
	default:
		var skippedCustom int

		reported, skippedCustom = r.primaryRoles(index, req)
		if skippedCustom > 0 {
			notes = append(notes, fmt.Sprintf(
				"%d custom roles are left out: the catalogue describes the platform role model. Name a custom role explicitly to include it",
				skippedCustom,
			))
		}
	}

	truncated := false
	for i := range reported {
		if reported[i].Truncated {
			truncated = true
		}
	}

	if r.scopeCache != nil && r.scopeCache.Partial() {
		notes = append(notes, "the discovery snapshot is incomplete: an aggregated API was unavailable, so some resources may be missing from the expanded rows")
	}
	if req.ExpandWildcards && (r.scopeCache == nil || !r.scopeCache.HasData()) {
		notes = append(notes, "wildcard expansion was requested but no discovery snapshot is available: wildcard rules are reported as written")
	}

	status := v1alpha1.RoleAccessReportStatus{
		Snapshot: v1alpha1.ReportSnapshot{
			Time:               metav1.NewTime(r.now()),
			Model:              modelOf(req.Model),
			ExpandedWildcards:  req.ExpandWildcards,
			DiscoveryResources: r.discoveryResourceCount(),
		},
		Roles:     stripInternal(reported),
		Notes:     notes,
		Truncated: truncated,
	}

	if req.IncludeInventory {
		status.Inventory = r.inventory(index)
		if len(status.Inventory) == 0 {
			notes = append(notes, "the cluster inventory was requested but no discovery snapshot is available: coverage cannot be measured")
			status.Notes = notes
		}
	}

	status.Snapshot.Digest = digestOf(status.Roles)

	return status, nil
}

// inventory is every resource of the cluster, so that a coverage review can
// name the ones no role reaches. The roles alone cannot answer that: a resource
// nobody grants leaves no trace in them.
func (r *RoleAccessResolver) inventory(index *roleIndex) []v1alpha1.InventoryResource {
	if r.scopeCache == nil {
		return nil
	}

	discovered := r.scopeCache.Inventory()
	byCapability := capabilityModules(index)

	inventory := make([]v1alpha1.InventoryResource, 0, len(discovered))
	for _, resource := range discovered {
		entry := v1alpha1.InventoryResource{
			Group:      resource.Group,
			Resource:   resource.Resource,
			Kind:       resource.Kind,
			Namespaced: resource.Namespaced,
			Verbs:      resource.Verbs,
		}

		defined := false
		if r.moduleIndex != nil {
			if origin, known := r.moduleIndex.Origin(resource.Group, resource.Resource); known {
				entry.Module = origin.Module
				entry.Custom = origin.Custom
				defined = true
			}
		}

		// Not every CRD the platform installs says which module installed it --
		// the older ones carry only the heritage label. The roles do know: a
		// role of a module names the resources of that module, which is the
		// same statement of ownership, made for the same reason.
		//
		// Only a resource that has a definition of its own is attributed this
		// way. Built-in Kubernetes resources belong to Kubernetes, and roles
		// name them constantly -- the capabilities that grant pods are shipped
		// by user-authz, and reading that as ownership would file half of
		// Kubernetes under it.
		if defined && entry.Module == "" {
			entry.Module = byCapability[capabilityKey(resource.Group, resource.Resource)]
		}

		inventory = append(inventory, entry)
	}

	return inventory
}

// ambiguousModule marks a resource claimed by capabilities of more than one
// module. Picking one of them would be a guess, and the report says "we do not
// know" by leaving the module empty.
const ambiguousModule = "\x00ambiguous"

func capabilityKey(group, resource string) string {
	// A subresource belongs to its parent: a capability granting "pods" owns
	// "pods/log" too.
	base, _, _ := strings.Cut(resource, "/")

	return group + "/" + base
}

// capabilityModules reads resource ownership out of the roles a module ships.
//
// Both models say it, in their own words. In the primary model that is a
// capability: it carries the module it belongs to and names that module's
// resources. In the legacy model it is the module's own access-level role,
// d8:user-authz:<module>:<level> -- the same statement, and the only one
// available on a cluster that never adopted capabilities.
//
// Wildcard rules are skipped. A role written as "*/*" grants everything the
// cluster has, and reading that as ownership would file the whole cluster under
// one module.
func capabilityModules(index *roleIndex) map[string]string {
	if index == nil {
		return nil
	}

	owners := make(map[string]string)

	for _, role := range index.all {
		module := role.Labels[labelModule]
		if module == "" {
			continue
		}

		// The seven roles of the levels themselves carry no access-level
		// annotation -- they are what the annotation points at -- so they never
		// reach here. They belong to user-authz and name the resources of the
		// whole cluster.
		isCapability := role.Labels[labelRoleKind] == roleKindCapability
		isModuleLevelRole := role.Annotations[annotationAccessLevel] != ""

		if !isCapability && !isModuleLevelRole {
			continue
		}

		for _, rule := range role.Rules {
			if slices.Contains(rule.APIGroups, "*") || slices.Contains(rule.Resources, "*") {
				continue
			}

			for _, group := range rule.APIGroups {
				for _, resource := range rule.Resources {
					key := capabilityKey(group, resource)

					switch owner := owners[key]; owner {
					case "", module:
						owners[key] = module
					default:
						owners[key] = ambiguousModule
					}
				}
			}
		}
	}

	for key, owner := range owners {
		if owner == ambiguousModule {
			delete(owners, key)
		}
	}

	return owners
}

func modelOf(model string) string {
	if model == RoleModelLegacy {
		return RoleModelLegacy
	}

	return RoleModelPrimary
}

// discoveryResourceCount is how many resources a wildcard row was expanded
// against. Reported so a wildcard can be read a year later, when the cluster
// has a different set of CRDs installed.
func (r *RoleAccessResolver) discoveryResourceCount() int {
	if r.scopeCache == nil {
		return 0
	}

	return len(r.scopeCache.ResourcesMatching([]string{"*"}, []string{"*"}))
}

// roleIndex is one pass over the ClusterRoles, arranged the three ways the
// report needs to read them.
type roleIndex struct {
	byName map[string]*rbacv1.ClusterRole
	all    []*rbacv1.ClusterRole
	// legacyNames maps a role to the names it had in the previous model. More
	// than one alias can point at the same role -- the rename folded the
	// kubernetes-suffixed variants together -- and an export that keeps only
	// one of them cannot be lined up with a document issued before the rename.
	legacyNames map[string][]string
}

func newRoleIndex(roles []*rbacv1.ClusterRole) *roleIndex {
	index := &roleIndex{
		byName:      make(map[string]*rbacv1.ClusterRole, len(roles)),
		all:         make([]*rbacv1.ClusterRole, 0, len(roles)),
		legacyNames: map[string][]string{},
	}

	for _, role := range roles {
		index.byName[role.Name] = role
		index.all = append(index.all, role)

		if replaced := role.Annotations[annotationReplacedBy]; replaced != "" {
			index.legacyNames[replaced] = append(index.legacyNames[replaced], role.Name)
		}
	}

	slices.SortFunc(index.all, func(a, b *rbacv1.ClusterRole) int { return strings.Compare(a.Name, b.Name) })
	for name := range index.legacyNames {
		slices.Sort(index.legacyNames[name])
	}

	return index
}

// primaryRoles reports the scope-based model: every ClusterRole the model calls
// a role, assembled from the capabilities it aggregates.
//
// req.ExcludeCustom leaves out the roles created in this cluster -- a catalogue
// mixing them with the model reads as if the platform shipped them. A role named
// explicitly is reported either way, and the count of the skipped ones is
// returned so that the omission stays visible in the document.
func (r *RoleAccessResolver) primaryRoles(index *roleIndex, req RoleAccessRequest) ([]v1alpha1.RoleAccess, int) {
	wantName := setOf(req.Names)
	wantScope := setOf(req.Scopes)

	skippedCustom := 0

	reported := make([]v1alpha1.RoleAccess, 0, len(index.all))
	for _, role := range index.all {
		kind := role.Labels[labelRoleKind]
		if kind != roleKindRole && kind != roleKindCustomRole {
			continue
		}

		_, named := wantName[role.Name]
		if req.ExcludeCustom && kind == roleKindCustomRole && !named {
			skippedCustom++

			continue
		}
		if len(wantName) > 0 && !named {
			continue
		}

		descriptor := DescribeRole(&role.ObjectMeta, role.Name)
		if len(wantScope) > 0 {
			if _, ok := wantScope[descriptor.Scope]; !ok {
				continue
			}
		}

		reported = append(reported, r.describeRole(index, role, descriptor, req))
	}

	slices.SortFunc(reported, compareRoles)

	return reported, skippedCustom
}

// The catalogue is read from the narrowest access to the widest, so that is how
// it is ordered. By name it would open with the administrator and close with the
// viewer, and the reader has to hold the model in their head to see that.
var (
	scopeOrder = map[string]int{"namespace": 0, "project": 1, "subsystem": 2, "system": 3}
	levelOrder = map[string]int{"viewer": 0, "user": 1, "manager": 2, "admin": 3, "superadmin": 4}
)

// rankOf places what the order does not know after what it does: a custom role
// with a level of its own belongs at the end of its scope, not in the middle of
// the model.
func rankOf(order map[string]int, key string) int {
	if rank, ok := order[key]; ok {
		return rank
	}

	return len(order)
}

func compareRoles(a, b v1alpha1.RoleAccess) int {
	if by := rankOf(scopeOrder, a.Role.Scope) - rankOf(scopeOrder, b.Role.Scope); by != 0 {
		return by
	}
	// Subsystems are peers: they are ordered by name, and the levels inside
	// each of them still run from the narrowest access to the widest.
	if by := strings.Compare(a.Role.Subsystem, b.Role.Subsystem); by != 0 {
		return by
	}
	if by := rankOf(levelOrder, a.Role.Level) - rankOf(levelOrder, b.Role.Level); by != 0 {
		return by
	}

	return strings.Compare(a.Name, b.Name)
}

// describeRole expands one role of the primary model.
func (r *RoleAccessResolver) describeRole(
	index *roleIndex,
	role *rbacv1.ClusterRole,
	descriptor v1alpha1.RoleDescriptor,
	req RoleAccessRequest,
) v1alpha1.RoleAccess {
	_, namespaced := namespacedScopes[descriptor.Scope]

	entry := v1alpha1.RoleAccess{
		Name:        role.Name,
		Role:        descriptor,
		LegacyNames: index.legacyNames[role.Name],
		Namespaced:  namespaced,
	}

	components := r.componentsOf(index, role)

	builder := newReportBuilder(r.limits, req.ExpandWildcards, r.scopeCache)
	scope := newScopeAccumulator()

	if len(components) == 0 {
		// A role with no aggregationRule carries its rules itself. The model
		// does not ship such roles, but a custom one may.
		builder.expand(scope, roleOrigin(role.Name, descriptor), role.Rules, namespaced)
	}
	for _, component := range components {
		builder.expand(scope, roleOrigin(component.Name, component.descriptor), component.role.Rules, namespaced)
	}

	entry.Resources = renderResources(scope, false)
	entry.NonResourceRules = renderNonResources(scope)
	entry.Truncated = builder.truncated

	if req.IncludeComposition {
		entry.Composition = componentDescriptors(components)
	} else {
		entry.Resources = dropSources(entry.Resources)
		entry.NonResourceRules = dropNonResourceSources(entry.NonResourceRules)
	}

	// The aggregation controller writes the union of the capabilities into the
	// role itself. Comparing row counts is not a proof of equality, but an
	// empty role whose capabilities are not empty is the shape that matters:
	// the controller has not caught up, and a report taken now understates
	// access.
	if len(components) > 0 && len(role.Rules) == 0 && len(entry.Resources) > 0 {
		entry.Notes = append(entry.Notes, "the aggregation controller has not filled this role yet; the rows come from the capabilities it selects")
	}

	return entry
}

// roleComponent is one capability (or bound role) with its metadata.
type roleComponent struct {
	Name       string
	role       *rbacv1.ClusterRole
	descriptor v1alpha1.RoleDescriptor
}

// componentsOf resolves what a role is assembled from, following the
// aggregation to the capabilities at the bottom of it.
//
// Two reasons it recurses. The levels include one another by aggregating the
// role below -- admin selects manager -- so stopping at the first level would
// attribute half of admin's rows to "manager", which answers "what does admin
// include" but not the question the export asks: which capability covers this
// resource. And expanding both a role and the capabilities under it would count
// the same rules twice, since the aggregation controller has already copied them
// upwards.
//
// The rules are read from the capabilities rather than from the role's own
// rules field, although the controller fills the latter: it concatenates
// without attribution, so expanding them one at a time is the only way to say
// where a row came from.
func (r *RoleAccessResolver) componentsOf(index *roleIndex, role *rbacv1.ClusterRole) []roleComponent {
	var (
		components []roleComponent
		seen       = map[string]struct{}{role.Name: {}}
	)

	r.collectComponents(index, role, seen, &components)

	slices.SortFunc(components, func(a, b roleComponent) int { return strings.Compare(a.Name, b.Name) })

	return components
}

func (r *RoleAccessResolver) collectComponents(
	index *roleIndex,
	role *rbacv1.ClusterRole,
	seen map[string]struct{},
	components *[]roleComponent,
) {
	if role.AggregationRule == nil {
		return
	}

	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		parsed, err := metav1.LabelSelectorAsSelector(&selector)
		if err != nil {
			continue
		}

		for _, candidate := range index.all {
			if _, ok := seen[candidate.Name]; ok {
				continue
			}
			if !parsed.Matches(labels.Set(candidate.Labels)) {
				continue
			}
			seen[candidate.Name] = struct{}{}

			if candidate.AggregationRule != nil {
				// An included role: its rules are a copy of what its own
				// components grant, so follow it instead of reading them.
				r.collectComponents(index, candidate, seen, components)

				continue
			}

			*components = append(*components, roleComponent{
				Name:       candidate.Name,
				role:       candidate,
				descriptor: DescribeRole(&candidate.ObjectMeta, candidate.Name),
			})
		}
	}
}

// legacyRoles reports the access levels of ClusterAuthorizationRule.
//
// A level is not a role: the fan-out binds the level's own ClusterRole plus
// every ClusterRole annotated for that level or a lower one. The report says
// what the level grants, which is the union of those.
func (r *RoleAccessResolver) legacyRoles(index *roleIndex, req RoleAccessRequest) []v1alpha1.RoleAccess {
	wanted := setOf(req.AccessLevels)
	for _, name := range req.Names {
		wanted[name] = struct{}{}
	}

	reported := make([]v1alpha1.RoleAccess, 0, len(legacyAccessLevels))
	for position, level := range legacyAccessLevels {
		if len(wanted) > 0 {
			if _, ok := wanted[level]; !ok {
				continue
			}
		}

		reported = append(reported, r.describeLegacyLevel(index, level, position, req))
	}

	return reported
}

func (r *RoleAccessResolver) describeLegacyLevel(index *roleIndex, level string, position int, req RoleAccessRequest) v1alpha1.RoleAccess {
	entry := v1alpha1.RoleAccess{
		Name: level,
		Role: v1alpha1.RoleDescriptor{Level: strings.ToLower(level)},
	}

	components := legacyComponents(index, level, position)

	builder := newReportBuilder(r.limits, req.ExpandWildcards, r.scopeCache)
	scope := newScopeAccumulator()
	for _, component := range components {
		// Not narrowed to namespaced resources: a ClusterAuthorizationRule
		// without namespace limits binds the level cluster-wide, so a
		// cluster-scoped rule of the level is exercisable.
		builder.expand(scope, roleOrigin(component.Name, component.descriptor), component.role.Rules, false)
	}

	entry.Resources = renderResources(scope, false)
	entry.NonResourceRules = renderNonResources(scope)
	entry.Truncated = builder.truncated

	if req.IncludeComposition {
		entry.Composition = componentDescriptors(components)
	} else {
		entry.Resources = dropSources(entry.Resources)
		entry.NonResourceRules = dropNonResourceSources(entry.NonResourceRules)
	}

	if len(components) == 0 {
		entry.Notes = append(entry.Notes, "no ClusterRole of this access level is present in the cluster")
	}

	return entry
}

// legacyComponents collects what a ClusterAuthorizationRule binds for a level:
// the level's own ClusterRole and every ClusterRole annotated for this level or
// a lower one.
func legacyComponents(index *roleIndex, level string, position int) []roleComponent {
	var (
		components []roleComponent
		seen       = map[string]struct{}{}
	)

	add := func(role *rbacv1.ClusterRole) {
		if role == nil {
			return
		}
		if _, ok := seen[role.Name]; ok {
			return
		}
		seen[role.Name] = struct{}{}
		components = append(components, roleComponent{
			Name:       role.Name,
			role:       role,
			descriptor: DescribeRole(&role.ObjectMeta, role.Name),
		})
	}

	add(index.byName[legacyBaseRole[level]])

	for _, role := range index.all {
		annotated := role.Annotations[annotationAccessLevel]
		if annotated == "" {
			continue
		}
		at := slices.Index(legacyAccessLevels, annotated)
		if at < 0 || at > position {
			continue
		}
		add(role)
	}

	slices.SortFunc(components, func(a, b roleComponent) int { return strings.Compare(a.Name, b.Name) })

	return components
}

// roleOrigin is the provenance of a row in a role report: the role or
// capability that carries the rule. The binding fields of grantOrigin stay
// empty on purpose -- a role grants nothing until something binds it, and the
// report is about the role.
func roleOrigin(name string, descriptor v1alpha1.RoleDescriptor) grantOrigin {
	return grantOrigin{
		roleKind: "ClusterRole",
		roleName: name,
		role:     descriptor,
		// One match, so the shared builder emits exactly one source per role.
		matches: []v1alpha1.SubjectMatch{{}},
	}
}

func componentDescriptors(components []roleComponent) []v1alpha1.RoleComponent {
	if len(components) == 0 {
		return nil
	}

	described := make([]v1alpha1.RoleComponent, 0, len(components))
	for _, component := range components {
		described = append(described, v1alpha1.RoleComponent{Name: component.Name, Role: component.descriptor})
	}

	return described
}

// dropSources removes the per-source detail from the plain matrix. The simple
// mode answers "role, resource, verbs" and nothing else; the sources exist to
// answer "which capability", which is what the detailed mode is for.
func dropSources(rows []v1alpha1.ResourceAccess) []v1alpha1.ResourceAccess {
	for i := range rows {
		rows[i].Sources = nil
	}

	return rows
}

func dropNonResourceSources(rows []v1alpha1.NonResourceAccess) []v1alpha1.NonResourceAccess {
	for i := range rows {
		rows[i].Sources = nil
	}

	return rows
}

// stripInternal clears the fields of the shared row type that a role report
// cannot fill: a role has no binding, so naming one would be a lie, and the
// subject match is meaningless outside a report about a subject.
func stripInternal(roles []v1alpha1.RoleAccess) []v1alpha1.RoleAccess {
	for i := range roles {
		for j := range roles[i].Resources {
			for k := range roles[i].Resources[j].Sources {
				source := &roles[i].Resources[j].Sources[k]
				source.BindingKind = ""
				source.BindingName = ""
				source.BindingNamespace = ""
				source.MatchedBy = v1alpha1.SubjectMatch{}
			}
		}
		for j := range roles[i].NonResourceRules {
			for k := range roles[i].NonResourceRules[j].Sources {
				source := &roles[i].NonResourceRules[j].Sources[k]
				source.BindingKind = ""
				source.BindingName = ""
				source.BindingNamespace = ""
				source.MatchedBy = v1alpha1.SubjectMatch{}
			}
		}
	}

	return roles
}

// digestOf hashes the canonical form of the reported roles.
//
// It is what makes two exports comparable: an unchanged cluster produces the
// same digest, so "nothing changed since last quarter" is something a reader
// can verify instead of having to diff two documents by eye. The snapshot
// itself is not hashed -- the timestamp would make every digest unique, which
// is the opposite of the point.
// digestOf hashes the reported roles.
//
// The JSON is streamed into the hash rather than marshalled into memory first:
// a report of a large cluster is tens of megabytes, and holding a second copy
// of it only to throw it away is the most expensive thing this function could
// do. The digest itself is opaque -- it is compared between two reports, never
// to a constant -- so the framing of the stream does not matter.
func digestOf(roles []v1alpha1.RoleAccess) string {
	hasher := sha256.New()
	if err := json.NewEncoder(hasher).Encode(roles); err != nil {
		return ""
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

func setOf(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}
