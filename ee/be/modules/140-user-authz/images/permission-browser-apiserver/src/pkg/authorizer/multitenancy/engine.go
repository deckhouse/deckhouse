/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/discovery"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	noNamespaceAccessReason      = "user has no access to the namespace"
	namespaceLimitedAccessReason = "making cluster-scoped requests for namespaced resources is not allowed"
)

// Engine implements the multi-tenancy authorization logic from user-authz webhook
type Engine struct {
	configPath      string
	lastAppliedStat os.FileInfo

	nsLister        corev1listers.NamespaceLister
	nsSynced        cache.InformerSynced
	discoveryClient discovery.DiscoveryInterface

	mu        sync.RWMutex
	directory map[string]map[string]DirectoryEntry

	// Cache for namespaced resources
	namespacedCache   map[string]bool
	namespacedCacheMu sync.RWMutex
}

// NewEngine creates a new multi-tenancy engine
func NewEngine(configPath string, nsLister corev1listers.NamespaceLister, nsSynced cache.InformerSynced, discoveryClient discovery.DiscoveryInterface) (*Engine, error) {
	e := &Engine{
		configPath:      configPath,
		nsLister:        nsLister,
		nsSynced:        nsSynced,
		discoveryClient: discoveryClient,
		directory:       make(map[string]map[string]DirectoryEntry),
		namespacedCache: make(map[string]bool),
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
		return authorizer.DecisionNoOpinion, "", nil
	}

	combinedDir := e.combineDirEntries(dirEntriesAffected)

	// Check namespaced request
	if attrs.GetNamespace() != "" {
		return e.authorizeNamespacedRequest(attrs, &combinedDir)
	}

	// Check cluster-scoped request for namespaced resource
	if attrs.GetResource() != "" {
		return e.authorizeClusterScopedRequest(attrs, &combinedDir)
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// authorizeNamespacedRequest checks if the user can access the specific namespace
func (e *Engine) authorizeNamespacedRequest(attrs authorizer.Attributes, entry *DirectoryEntry) (authorizer.Decision, string, error) {
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
	if !denied && !entry.AllowAccessToSystemNamespaces {
		for _, pattern := range systemNamespacesRegex {
			if pattern.MatchString(namespace) {
				denied = true
				reason = noNamespaceAccessReason
				break
			}
		}
	}

	// Check namespace selectors
	if denied && len(entry.NamespaceSelectors) > 0 {
		match, err := e.namespaceLabelsMatchSelector(namespace, entry.NamespaceSelectors)
		if err != nil {
			klog.Errorf("Error checking namespace labels: %v", err)
		} else if match {
			denied = false
			reason = ""
		}
	}

	if denied {
		return authorizer.DecisionDeny, reason, nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// authorizeClusterScopedRequest checks if cluster-scoped requests for namespaced resources should be denied
func (e *Engine) authorizeClusterScopedRequest(attrs authorizer.Attributes, entry *DirectoryEntry) (authorizer.Decision, string, error) {
	if !hasAnyFilters(entry) {
		return authorizer.DecisionNoOpinion, "", nil
	}

	group := attrs.GetAPIGroup()
	version := attrs.GetAPIVersion()
	resource := attrs.GetResource()

	// Check if resource is namespaced
	namespaced, err := e.isResourceNamespaced(group, version, resource)
	if err != nil {
		klog.V(4).Infof("Could not determine if resource %s/%s/%s is namespaced: %v", group, version, resource, err)
		return authorizer.DecisionNoOpinion, "", nil
	}

	if namespaced {
		return authorizer.DecisionDeny, namespaceLimitedAccessReason, nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// isResourceNamespaced checks if a resource is namespaced using discovery
func (e *Engine) isResourceNamespaced(group, version, resource string) (bool, error) {
	// Use schema.GroupVersionResource for type-safe cache key
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	cacheKey := gvr.String()

	// Check cache with proper defer unlock
	var namespaced, ok bool
	func() {
		e.namespacedCacheMu.RLock()
		defer e.namespacedCacheMu.RUnlock()
		namespaced, ok = e.namespacedCache[cacheKey]
	}()
	if ok {
		return namespaced, nil
	}

	// Query apiserver for preferred version of this API resource
	gv, err := e.getPreferredGroupVersion(group, version)
	if err != nil {
		return false, err
	}

	resourceList, err := e.discoveryClient.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return false, err
	}

	e.namespacedCacheMu.Lock()
	defer e.namespacedCacheMu.Unlock()
	for _, r := range resourceList.APIResources {
		if r.Name == resource || strings.HasPrefix(r.Name, resource+"/") {
			e.namespacedCache[cacheKey] = r.Namespaced
			return r.Namespaced, nil
		}
	}

	return false, nil
}

// getPreferredGroupVersion returns the preferred GroupVersion for a group.
// Uses schema.GroupVersion for type-safe group/version handling.
func (e *Engine) getPreferredGroupVersion(group, version string) (schema.GroupVersion, error) {
	// If version is specified, use it
	if version != "" {
		return schema.GroupVersion{Group: group, Version: version}, nil
	}

	// For core API group
	if group == "" {
		return schema.GroupVersion{Version: "v1"}, nil
	}

	// Query apiserver for preferred version
	apiGroupList, err := e.discoveryClient.ServerGroups()
	if err != nil {
		// Propagate discovery error. The caller will treat it as best-effort and return NoOpinion.
		return schema.GroupVersion{}, fmt.Errorf("failed to discover server groups while resolving preferred version for group %q: %w", group, err)
	}

	for _, g := range apiGroupList.Groups {
		if g.Name == group {
			// Parse the preferred version string into GroupVersion
			gv, parseErr := schema.ParseGroupVersion(g.PreferredVersion.GroupVersion)
			if parseErr != nil {
				return schema.GroupVersion{}, fmt.Errorf("failed to parse preferred version %q for group %q: %w", g.PreferredVersion.GroupVersion, group, parseErr)
			}
			return gv, nil
		}
	}

	// Group not found - this should not happen in a properly functioning cluster
	return schema.GroupVersion{}, fmt.Errorf("API group %q not found in server groups", group)
}

// combineDirEntries combines multiple directory entries into one
func (e *Engine) combineDirEntries(entries []DirectoryEntry) DirectoryEntry {
	var combined DirectoryEntry

	for _, entry := range entries {
		if !combined.AllowAccessToSystemNamespaces {
			combined.AllowAccessToSystemNamespaces = entry.AllowAccessToSystemNamespaces
		}

		if len(entry.NamespaceSelectors) > 0 {
			combined.NamespaceSelectors = append(combined.NamespaceSelectors, entry.NamespaceSelectors...)
		}
		if len(entry.LimitNamespaces) > 0 {
			combined.LimitNamespaces = append(combined.LimitNamespaces, entry.LimitNamespaces...)
		}
		combined.NamespaceFiltersAbsent = combined.NamespaceFiltersAbsent || entry.NamespaceFiltersAbsent
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

// namespaceLabelsMatchSelector checks if labels of a namespace match provided labelselector
func (e *Engine) namespaceLabelsMatchSelector(namespaceName string, namespaceSelectors []*NamespaceSelector) (bool, error) {
	if e.nsLister == nil {
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

	for _, namespaceSelector := range namespaceSelectors {
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

	e.lastAppliedStat = fileStat

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
			name := subject.Name
			namespace := subject.Namespace
			kind := subject.Kind

			if kind == "ServiceAccount" {
				name = "system:serviceaccount:" + namespace + ":" + name
			}

			dirEntry, ok := directory[kind][name]
			if !ok {
				dirEntry = DirectoryEntry{}
			}

			// If there are neither LimitNamespaces nor NamespaceSelector options, it means all non-system namespaces are allowed
			dirEntry.NamespaceFiltersAbsent = dirEntry.NamespaceFiltersAbsent || (len(crd.Spec.LimitNamespaces) == 0 && !isLabelSelectorApplied(crd.Spec.NamespaceSelector))

			if crd.Spec.NamespaceSelector == nil {
				for _, ln := range crd.Spec.LimitNamespaces {
					r, _ := regexp.Compile(wrapRegex(ln))
					dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
				}

				if !dirEntry.AllowAccessToSystemNamespaces {
					dirEntry.AllowAccessToSystemNamespaces = crd.Spec.AllowAccessToSystemNamespaces
				}
			} else {
				dirEntry.NamespaceSelectors = append(dirEntry.NamespaceSelectors, crd.Spec.NamespaceSelector)
			}

			directory[kind][name] = dirEntry
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.directory = directory
	klog.Info("Multi-tenancy configuration was reloaded successfully")
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
		ln = ln + "$"
	}
	return ln
}
