/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// Keep in sync with the webhook (webhook/internal/web/hook/handler.go): both
	// answer the same question, and the wording must not let a caller tell a
	// closed namespace from a missing one.
	noNamespaceAccessReason      = "either you have no access to the namespace or the namespace does not exist"
	namespaceLimitedAccessReason = "making cluster-scoped requests for namespaced resources is not allowed"
)

// privilegedGroups contains groups that bypass multi-tenancy restrictions.
// Users in these groups are allowed full access even without ClusterAuthorizationRules.
var privilegedGroups = map[string]struct{}{
	"system:masters":         {},
	"kubeadm:cluster-admins": {},
	"superadmins":            {},
}

// isPrivilegedUser checks if the user belongs to any privileged group
// that should bypass multi-tenancy restrictions.
func isPrivilegedUser(groups []string) bool {
	for _, group := range groups {
		if _, ok := privilegedGroups[group]; ok {
			return true
		}
	}
	return false
}

// IndependentRBACChecker reports whether a request is allowed by RBAC grants
// that exist independently of ClusterAuthorizationRules: RoleBindings in the
// request's namespace and ClusterRoleBindings not generated from a CAR.
type IndependentRBACChecker interface {
	AllowsIndependently(ctx context.Context, attrs authorizer.Attributes) bool
}

// ResourceScope reports whether a resource is namespaced. known is false when
// the snapshot has never seen the group/resource (missing CRD, broken
// APIService, empty cache). Callers that enforce namespace limits treat
// !known like namespaced: failing open would let a CAR ClusterRoleBinding
// report Allow for a cluster-scoped list of a namespaced resource.
//
// HasData separates "the snapshot exists and does not list this resource"
// from "there is no snapshot at all". The webhook makes the same distinction,
// and only the first case, for the core group, lets RBAC answer.
//
// Implemented by resolver.ResourceScopeCache. This package must not import
// resolver (resolver already imports multitenancy).
type ResourceScope interface {
	Scope(group, resource string) (bool, bool)
	HasData() bool
}

// Engine implements the multi-tenancy authorization logic from user-authz webhook
type Engine struct {
	configPath      string
	lastAppliedStat os.FileInfo

	nsLister      corev1listers.NamespaceLister
	nsSynced      cache.InformerSynced
	resourceScope ResourceScope

	// independentRBAC, when set, is consulted before returning Deny: requests
	// explicitly granted by CAR-independent RBAC must not be denied by
	// multi-tenancy filters.
	independentRBAC IndependentRBACChecker

	mu        sync.RWMutex
	directory map[string]map[string]DirectoryEntry
}

// SetIndependentRBACChecker wires the CAR-independent RBAC checker into the
// engine. Must be called before the engine starts serving Authorize calls.
func (e *Engine) SetIndependentRBACChecker(checker IndependentRBACChecker) {
	e.independentRBAC = checker
}

// NewEngine creates a new multi-tenancy engine. resourceScope may be nil: every
// lookup then reports !known, which is fail-closed for filtered cluster-scoped
// requests. Live discovery is never used on Authorize.
func NewEngine(configPath string, nsLister corev1listers.NamespaceLister, nsSynced cache.InformerSynced, resourceScope ResourceScope) (*Engine, error) {
	e := &Engine{
		configPath:    configPath,
		nsLister:      nsLister,
		nsSynced:      nsSynced,
		resourceScope: resourceScope,
		directory:     make(map[string]map[string]DirectoryEntry),
	}

	// Initial config load
	e.renewDirectories()

	return e, nil
}

// Authorize implements authorizer.Authorizer
// This authorizer only denies; it never allows (returns NoOpinion if access is not restricted)
func (e *Engine) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if !attrs.IsResourceRequest() {
		return authorizer.DecisionNoOpinion, "", nil
	}

	user := attrs.GetUser()
	if user == nil {
		return authorizer.DecisionNoOpinion, "", nil
	}

	dirEntriesAffected := e.affectedDirs(user.GetName(), user.GetGroups())
	if len(dirEntriesAffected) == 0 {
		// NOTE: We intentionally return NoOpinion here (not Deny) even for users without CAR.
		// This method is part of the Kubernetes authorizer chain for API requests.
		// NoOpinion means "I have no opinion, let RBAC decide".
		// If we returned Deny here, users without CAR couldn't do ANYTHING,
		// even with valid RBAC permissions (RoleBindings).
		//
		// The deny-by-default logic is applied only in GetNamespaceAccessType /
		// IsNamespaceAllowedWithFilter, which are used for filtering the
		// accessiblenamespaces API response (data filtering), not for API
		// request authorization.
		return authorizer.DecisionNoOpinion, "", nil
	}

	combinedDir := e.combineDirEntries(dirEntriesAffected)

	// Check namespaced request
	if attrs.GetNamespace() != "" {
		return e.authorizeNamespacedRequest(ctx, attrs, &combinedDir)
	}

	// Check cluster-scoped request for namespaced resource
	if attrs.GetResource() != "" {
		return e.authorizeClusterScopedRequest(ctx, attrs, &combinedDir)
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// authorizeNamespacedRequest checks if the user can access the specific namespace.
//
// The multi-tenancy scope here is CAR-only (LimitNamespaces, namespaceSelectors,
// the system-namespace gate): inside that scope the CAR's cluster-wide
// accessLevel binding is meant to apply, so we return NoOpinion and let RBAC
// decide. Outside that scope the request is denied unless CAR-independent RBAC
// explicitly grants it - this both keeps RoleBinding/AuthorizationRule access
// working and prevents the CAR accessLevel from leaking into namespaces not
// listed in limitNamespaces.
func (e *Engine) authorizeNamespacedRequest(ctx context.Context, attrs authorizer.Attributes, entry *DirectoryEntry) (authorizer.Decision, string, error) {
	if !hasAnyFilters(entry) {
		return authorizer.DecisionNoOpinion, "", nil
	}

	namespace := attrs.GetNamespace()
	denied := true
	reason := noNamespaceAccessReason

	// Check limitNamespaces patterns
	if !entry.NamespaceFiltersAbsent {
		for _, pattern := range entry.LimitNamespaces {
			if pattern.MatchString(namespace) {
				denied = false
				reason = ""
				break
			}
		}
	} else {
		denied = false
	}

	// Check system namespaces restriction
	if !denied && isSystemNamespace(namespace) && !systemNamespaceAllowed(entry, namespace) {
		denied = true
		reason = noNamespaceAccessReason
	}

	// Check namespace selectors
	if denied && len(entry.NamespaceSelectors) > 0 {
		match, err := e.namespaceLabelsMatchSelector(namespace, entry)
		if err != nil {
			klog.Errorf("Error checking namespace labels: %v", err)
		} else if match {
			denied = false
			reason = ""
		}
	}

	// The namespace is outside the CAR scope. Requests granted by
	// CAR-independent RBAC (RoleBindings in the namespace, non-CAR
	// ClusterRoleBindings) must not be denied.
	if denied && e.independentRBAC != nil && e.independentRBAC.AllowsIndependently(ctx, attrs) {
		denied = false
		reason = ""
	}

	if denied {
		return authorizer.DecisionDeny, reason, nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// authorizeClusterScopedRequest checks if cluster-scoped requests for namespaced resources should be denied
func (e *Engine) authorizeClusterScopedRequest(ctx context.Context, attrs authorizer.Attributes, entry *DirectoryEntry) (authorizer.Decision, string, error) {
	if !hasAnyFilters(entry) {
		return authorizer.DecisionNoOpinion, "", nil
	}

	namespaced, known, populated := false, false, false
	if e.resourceScope != nil {
		namespaced, known = e.resourceScope.Scope(attrs.GetAPIGroup(), attrs.GetResource())
		populated = e.resourceScope.HasData()
	}

	// Known cluster-scoped resources are not limited by namespace filters.
	// Everything else (namespaced, or unknown) is treated as namespaced so a
	// discovery miss cannot fail-open through a CAR ClusterRoleBinding.
	if known && !namespaced {
		return authorizer.DecisionNoOpinion, "", nil
	}

	// A core resource absent from a populated snapshot does not exist. The
	// webhook lets RBAC answer that case instead of denying it, so BulkSAR
	// must not report Deny where the webhook would not. Groups are excluded:
	// there the webhook denies an unresolvable resource, and so do we.
	if !known && populated && attrs.GetAPIGroup() == "" {
		return authorizer.DecisionNoOpinion, "", nil
	}

	if e.independentRBAC != nil && e.independentRBAC.AllowsIndependently(ctx, attrs) {
		return authorizer.DecisionNoOpinion, "", nil
	}
	return authorizer.DecisionDeny, namespaceLimitedAccessReason, nil
}

// combineDirEntries combines multiple directory entries into one.
// AllowedSystemNamespaces is merged into a fresh map so the combined view does
// not alias (and cannot mutate) the source entries stored in the directory.
func (e *Engine) combineDirEntries(entries []DirectoryEntry) DirectoryEntry {
	var combined DirectoryEntry

	for _, entry := range entries {
		if !combined.AllowAccessToSystemNamespaces {
			combined.AllowAccessToSystemNamespaces = entry.AllowAccessToSystemNamespaces
		}

		if len(entry.NamespaceSelectors) > 0 {
			combined.NamespaceSelectors = append(combined.NamespaceSelectors, entry.NamespaceSelectors...)
		}
		if len(entry.compiledSelectors) > 0 {
			combined.compiledSelectors = append(combined.compiledSelectors, entry.compiledSelectors...)
		}
		if len(entry.LimitNamespaces) > 0 {
			combined.LimitNamespaces = append(combined.LimitNamespaces, entry.LimitNamespaces...)
		}
		combined.NamespaceFiltersAbsent = combined.NamespaceFiltersAbsent || entry.NamespaceFiltersAbsent

		for ns := range entry.AllowedSystemNamespaces {
			if combined.AllowedSystemNamespaces == nil {
				combined.AllowedSystemNamespaces = make(map[string]struct{}, len(entry.AllowedSystemNamespaces))
			}
			combined.AllowedSystemNamespaces[ns] = struct{}{}
		}
	}

	return combined
}

// affectedDirs checks that User/Group/ServiceAccount has corresponding ClusterAuthorizationRules
func (e *Engine) affectedDirs(userName string, groups []string) []DirectoryEntry {
	var dirEntriesAffected []DirectoryEntry

	e.mu.RLock()
	defer e.mu.RUnlock()

	if entry, ok := e.directory["User"][userName]; ok {
		dirEntriesAffected = append(dirEntriesAffected, entry)
	}

	if entry, ok := e.directory["ServiceAccount"][userName]; ok {
		dirEntriesAffected = append(dirEntriesAffected, entry)
	}

	for _, group := range groups {
		if entry, ok := e.directory["Group"][group]; ok {
			dirEntriesAffected = append(dirEntriesAffected, entry)
		}
	}

	return dirEntriesAffected
}

// namespaceLabelsMatchSelector checks if labels of a namespace match the
// entry's selectors. Prefer compiledSelectors (built in renewDirectories).
func (e *Engine) namespaceLabelsMatchSelector(namespaceName string, entry *DirectoryEntry) (bool, error) {
	if e.nsLister == nil || entry == nil {
		return false, nil
	}

	namespace, err := e.nsLister.Get(namespaceName)
	if err != nil {
		return false, err
	}

	labelsSet := labels.Set(namespace.GetLabels())
	if labelsSet == nil {
		labelsSet = labels.Set{}
	}

	if len(entry.compiledSelectors) > 0 {
		for _, selector := range entry.compiledSelectors {
			if selector.Matches(labelsSet) {
				return true, nil
			}
		}
		return false, nil
	}

	for _, namespaceSelector := range entry.NamespaceSelectors {
		if namespaceSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(namespaceSelector.LabelSelector)
			if err != nil {
				return false, err
			}
			if selector.Matches(labelsSet) {
				return true, nil
			}
		}
	}
	return false, nil
}

// renewDirectories reads the configuration file and composes rules
func (e *Engine) renewDirectories() {
	fileStat, err := os.Stat(e.configPath)
	if err != nil {
		klog.V(4).Infof("Cannot read config stat: %v", err)
		return
	}

	if os.SameFile(e.lastAppliedStat, fileStat) {
		return
	}

	var config UserAuthzConfig

	configRawData, err := os.ReadFile(e.configPath)
	if err != nil {
		klog.Errorf("Cannot read config %s: %v", e.configPath, err)
		return
	}

	if err := json.Unmarshal(configRawData, &config); err != nil {
		klog.Errorf("Cannot unmarshal config %s: %v", e.configPath, err)
		return
	}

	directory := map[string]map[string]DirectoryEntry{
		"User":           make(map[string]DirectoryEntry),
		"Group":          make(map[string]DirectoryEntry),
		"ServiceAccount": make(map[string]DirectoryEntry),
	}

	// Fill limited namespaces by subjects kinds/names
	for _, crd := range config.CRDs {
		for _, subject := range crd.Spec.Subjects {
			kind, name := subjectDirectoryKey(subject.Kind, subject.Name, subject.Namespace, "")

			if _, ok := directory[kind]; !ok {
				continue
			}

			dirEntry := directory[kind][name]

			// If there are neither LimitNamespaces nor NamespaceSelector options, it means all non-system namespaces are allowed
			dirEntry.NamespaceFiltersAbsent = dirEntry.NamespaceFiltersAbsent || (len(crd.Spec.LimitNamespaces) == 0 && !isLabelSelectorApplied(crd.Spec.NamespaceSelector))

			if crd.Spec.NamespaceSelector == nil {
				for _, ln := range crd.Spec.LimitNamespaces {
					r, err := regexp.Compile(wrapRegex(ln))
					if err != nil {
						klog.Errorf("Cannot compile limitNamespaces pattern %q from ClusterAuthorizationRule %q: %v", ln, crd.Name, err)
						return
					}
					dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
				}

				if !dirEntry.AllowAccessToSystemNamespaces {
					dirEntry.AllowAccessToSystemNamespaces = crd.Spec.AllowAccessToSystemNamespaces
				}
			} else {
				dirEntry.NamespaceSelectors = append(dirEntry.NamespaceSelectors, crd.Spec.NamespaceSelector)
				if crd.Spec.NamespaceSelector.LabelSelector != nil {
					selector, err := metav1.LabelSelectorAsSelector(crd.Spec.NamespaceSelector.LabelSelector)
					if err != nil {
						klog.Errorf("Cannot compile namespaceSelector from ClusterAuthorizationRule %q: %v", crd.Name, err)
						return
					}
					dirEntry.compiledSelectors = append(dirEntry.compiledSelectors, selector)
				}
			}

			directory[kind][name] = dirEntry
		}
	}

	// NOTE: AuthorizationRules (config.ARs) are intentionally NOT applied here.
	// The directory is built from ClusterAuthorizationRules (CARs) ONLY, mirroring
	// the real kube-apiserver user-authz webhook authorizer (images/webhook), whose
	// config struct parses "crds" and ignores "ars" entirely. AR-derived access is
	// surfaced through the RBAC path instead: each AR creates a RoleBinding that the
	// RBAC authorizer (BulkSubjectAccessReview) and the namespace resolver
	// (AccessibleNamespaces) pick up. Feeding ARs into this deny-only engine would
	// turn them into spurious namespace deny-filters, making the reported view
	// inconsistent with real authorization. For users that ALSO have a CAR (whose
	// filters deny outside their scope), AR namespaces are rescued in Authorize by
	// the CAR-independent RBAC check, which finds the AR's RoleBinding.

	e.mu.Lock()
	e.directory = directory
	e.lastAppliedStat = fileStat
	e.mu.Unlock()
	klog.Info("Multi-tenancy configuration was reloaded successfully")
}

// subjectDirectoryKey returns the directory map key for a subject.
// ServiceAccounts are canonicalized to "system:serviceaccount:<ns>:<name>";
// defaultNamespace fills in subject.Namespace when it's empty (RBAC fallback).
func subjectDirectoryKey(kind, name, subjectNamespace, defaultNamespace string) (string, string) {
	if kind != "ServiceAccount" {
		return kind, name
	}
	ns := subjectNamespace
	if ns == "" {
		ns = defaultNamespace
	}
	return kind, "system:serviceaccount:" + ns + ":" + name
}

// StartRenewConfigLoop periodically reads new config file
func (e *Engine) StartRenewConfigLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.renewDirectories()
		case <-stopCh:
			klog.Info("Renew directories stopped")
			return
		}
	}
}

// hasAnyFilters checks if an entry has any namespace-related filters
func hasAnyFilters(entry *DirectoryEntry) bool {
	// Check for MatchAny field of any namespace selector which permits literally any namespace
	for _, namespaceSelector := range entry.NamespaceSelectors {
		if namespaceSelector.MatchAny {
			return false
		}
	}

	if entry.NamespaceFiltersAbsent {
		return !entry.AllowAccessToSystemNamespaces
	}

	for _, regex := range entry.LimitNamespaces {
		switch regex.String() {
		case "^.*$", "^.+$":
			return !entry.AllowAccessToSystemNamespaces
		}
	}

	return true
}

func isLabelSelectorApplied(namespaceSelector *NamespaceSelector) bool {
	return namespaceSelector != nil && namespaceSelector.LabelSelector != nil
}

func wrapRegex(ln string) string {
	if !strings.HasPrefix(ln, "^") {
		ln = "^" + ln
	}
	if !strings.HasSuffix(ln, "$") {
		ln += "$"
	}
	return ln
}

// GetNamespaceAccessType evaluates the user's namespace access and returns:
//   - accessType: AllNamespacesAllowed, NoNamespacesAllowed, or FilteredAccess
//   - filter: the combined directory entry for filtering (only valid when accessType == FilteredAccess)
//
// This method computes affectedDirs once and returns the filter for reuse,
// avoiding redundant lookups when filtering multiple namespaces.
func (e *Engine) GetNamespaceAccessType(userInfo user.Info) (NamespaceAccessType, *DirectoryEntry) {
	if userInfo == nil {
		return AllNamespacesAllowed, nil
	}

	dirEntriesAffected := e.affectedDirs(userInfo.GetName(), userInfo.GetGroups())
	if len(dirEntriesAffected) == 0 {
		// No ClusterAuthorizationRules apply to this user.
		// Privileged users (system:masters, etc.) bypass MT restrictions.
		if isPrivilegedUser(userInfo.GetGroups()) {
			klog.V(4).Infof("GetNamespaceAccessType: user=%s is privileged, all namespaces allowed", userInfo.GetName())
			return AllNamespacesAllowed, nil
		}
		// Non-privileged users without CAR get no access (deny-by-default).
		klog.V(4).Infof("GetNamespaceAccessType: user=%s has no CAR and is not privileged (deny-by-default)", userInfo.GetName())
		return NoNamespacesAllowed, nil
	}

	combinedDir := e.combineDirEntries(dirEntriesAffected)
	if !hasAnyFilters(&combinedDir) {
		return AllNamespacesAllowed, nil
	}

	// User has restrictions - caller must filter each namespace
	return FilteredAccess, &combinedDir
}

// IsNamespaceAllowedWithFilter checks if a namespace is allowed using a pre-computed filter.
// Use this with the filter returned by GetNamespaceAccessType to avoid redundant affectedDirs lookups.
func (e *Engine) IsNamespaceAllowedWithFilter(namespace string, filter *DirectoryEntry) bool {
	if filter == nil {
		return true
	}

	allowed := true

	// Check limitNamespaces patterns
	if !filter.NamespaceFiltersAbsent {
		allowed = false
		for _, pattern := range filter.LimitNamespaces {
			if pattern.MatchString(namespace) {
				allowed = true
				break
			}
		}
	}

	// Check system namespaces restriction
	if allowed && isSystemNamespace(namespace) && !systemNamespaceAllowed(filter, namespace) {
		allowed = false
	}

	// Check namespace selectors if denied by patterns
	if !allowed && len(filter.NamespaceSelectors) > 0 {
		match, err := e.namespaceLabelsMatchSelector(namespace, filter)
		if err == nil && match {
			allowed = true
		}
	}

	return allowed
}
