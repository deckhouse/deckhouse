/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"user-authz-webhook/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	configPath = "/etc/user-authz-webhook/config.json"

	noNamespaceAccessReason      = "user has no access to the namespace"
	namespaceLimitedAccessReason = "making cluster-scoped requests for namespaced resources is not allowed"
	internalErrorReason          = "webhook: kubernetes api request error"
)

var _ http.Handler = (*Handler)(nil)

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger *log.Logger

	lastAppliedStat os.FileInfo

	cache cache.Cache

	kubeclient kubernetes.Interface

	//        [user type] [user name]
	mu        sync.RWMutex
	directory map[string]map[string]DirectoryEntry
}

func NewHandler(logger *log.Logger, discoveryCache cache.Cache) (*Handler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Handler{
		logger:     logger,
		cache:      discoveryCache,
		kubeclient: clientSet,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		return
	}

	var request WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Printf("cannot unmarshal kubernetes request: %v", err)
		http.Error(w, "Invalid json request", http.StatusBadRequest)
		return
	}

	h.authorizeRequest(&request)

	respData, err := json.Marshal(request)
	if err != nil {
		// this case is exceptional
		h.logger.Fatalf("cannot marshal json response: %v", respData)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(respData)

	h.logger.Printf("response body: %s", respData)
}

func (h *Handler) authorizeNamespacedRequest(request *WebhookRequest, entry *DirectoryEntry) *WebhookRequest {
	if !hasAnyFilters(entry) {
		// User has no namespaced restriction.
		return request
	}

	// there are limitNamespaces/namespaceSelectors
	if !entry.NamespaceFiltersAbsent {
		// deny all namespaces
		request.Status.Denied = true
		request.Status.Reason = noNamespaceAccessReason
		// check if the target namespace is in limitNamespaces list
		for _, pattern := range entry.LimitNamespaces {
			if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
				request.Status.Denied = false
				request.Status.Reason = ""
				break
			}
		}
	} else {
		// there is no filters - assume a positive outcome
		request.Status.Denied = false
	}

	if !request.Status.Denied && !entry.AllowAccessToSystemNamespaces {
		// check if the target namespace is a system one and restricted
		for _, pattern := range systemNamespacesRegex {
			if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
				request.Status.Denied = true
				request.Status.Reason = noNamespaceAccessReason
				break
			}
		}
	}

	// if request is still denied - check available namespace selectors if any of them matches the request namespace, doesn't matter a system one or not
	if request.Status.Denied && len(entry.NamespaceSelectors) > 0 {
		match, err := h.namespaceLabelsMatchSelector(request.Spec.ResourceAttributes.Namespace, entry.NamespaceSelectors)
		if err != nil {
			request.Status.Reason = err.Error()
		} else if match {
			request.Status.Denied = false
			request.Status.Reason = ""
		}
	}

	return request
}

func (h *Handler) fillDenyRequest(request *WebhookRequest, reason, logEntry string) *WebhookRequest {
	if logEntry != "" {
		h.logger.Println(logEntry)
	}

	request.Status.Denied = true
	request.Status.Reason = reason

	return request
}

func (h *Handler) authorizeClusterScopedRequest(request *WebhookRequest, entry *DirectoryEntry) *WebhookRequest {
	// if resource is not nil and namespace is nil
	apiGroup := request.Spec.ResourceAttributes.Version
	group := request.Spec.ResourceAttributes.Group

	if apiGroup == "" {
		if group != "" {
			var err error
			apiGroup, err = h.cache.GetPreferredVersion(group)
			if err != nil {
				// could not check whether resource is namespaced or not (from cache) - deny access
				return h.fillDenyRequest(request, internalErrorReason, err.Error())
			}
		} else {
			// apiGroup and group versions both empty, which means that this is a core Kubernetes resource
			apiGroup = "v1"
		}
	}

	if group != "" {
		apiGroup = group + "/" + apiGroup
	}

	namespaced, err := h.cache.Get(apiGroup, request.Spec.ResourceAttributes.Resource)
	if err != nil {
		// could not check whether resource is namespaced or not (from cache) - deny access
		h.fillDenyRequest(request, internalErrorReason, err.Error())

	} else if namespaced && hasAnyFilters(entry) {
		// we should not allow cluster-scoped requests for the namespaced objects if access to the namespaces is limited
		h.fillDenyRequest(request, namespaceLimitedAccessReason, "")
	}

	return request
}

func (h *Handler) authorizeRequest(request *WebhookRequest) *WebhookRequest {
	dirEntriesAffected := h.affectedDirs(request)
	if len(dirEntriesAffected) == 0 {
		return request
	}

	var combinedDir DirectoryEntry

	// Combine dirs for the current request. Users may have more than one rule attached to their groups or usernames.
	for _, dirEntry := range dirEntriesAffected {
		if !combinedDir.AllowAccessToSystemNamespaces {
			combinedDir.AllowAccessToSystemNamespaces = dirEntry.AllowAccessToSystemNamespaces
		}

		// Aggregate namespace selectors and limitNamespaces into a single set of rules
		if len(dirEntry.NamespaceSelectors) > 0 {
			combinedDir.NamespaceSelectors = append(combinedDir.NamespaceSelectors, dirEntry.NamespaceSelectors...)
		}
		if len(dirEntry.LimitNamespaces) > 0 {
			combinedDir.LimitNamespaces = append(combinedDir.LimitNamespaces, dirEntry.LimitNamespaces...)
		}
		combinedDir.NamespaceFiltersAbsent = combinedDir.NamespaceFiltersAbsent || dirEntry.NamespaceFiltersAbsent
	}

	if request.Spec.ResourceAttributes.Namespace != "" {
		return h.authorizeNamespacedRequest(request, &combinedDir)
	}

	if request.Spec.ResourceAttributes.Resource != "" {
		return h.authorizeClusterScopedRequest(request, &combinedDir)
	}

	return request
}

// renewDirectories reads the configuration file (actually it is a json file with all CRs from the cluster) and composes
// rules for users, groups, and service accounts.
func (h *Handler) renewDirectories() {
	fileStat, err := os.Stat(configPath)
	if err != nil {
		h.logger.Printf("cannot reload the config: %v", err)
		return
	}

	if os.SameFile(h.lastAppliedStat, fileStat) {
		return
	}

	h.lastAppliedStat = fileStat

	var config UserAuthzConfig

	configRawData, err := os.ReadFile(configPath)
	if err != nil {
		h.logger.Printf("cannot read the config %s: %v", configPath, err)
		return
	}

	if err := json.Unmarshal(configRawData, &config); err != nil {
		h.logger.Printf("cannot unmarshal the config %s: %v", configPath, err)
		return
	}

	directory := map[string]map[string]DirectoryEntry{
		"User":           make(map[string]DirectoryEntry),
		"Group":          make(map[string]DirectoryEntry),
		"ServiceAccount": make(map[string]DirectoryEntry),
	}

	// fill limited namespaces by subjects kinds/names
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

			// If there are neither LimitNamespaces nor NamespaceSelector options, it means all non-system namespaces are allowed.
			// We need to know whether we have at least one such a CR for the user in a cluster.
			dirEntry.NamespaceFiltersAbsent = dirEntry.NamespaceFiltersAbsent || (len(crd.Spec.LimitNamespaces) == 0 && !isLabelSelectorApplied(crd.Spec.NamespaceSelector))

			// if the NamespaceSelector field is empty - take the limitNamespaces entries and check the allowAccessToSystemNamespaces flag
			if crd.Spec.NamespaceSelector == nil {
				// This is an important thing! All regular expressions is wrapped in the ^...$
				for _, ln := range crd.Spec.LimitNamespaces {
					r, _ := regexp.Compile(wrapRegex(ln))
					dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
				}

				if !dirEntry.AllowAccessToSystemNamespaces {
					dirEntry.AllowAccessToSystemNamespaces = crd.Spec.AllowAccessToSystemNamespaces
				}
				// if the NamespaceSelector field isn't empty - ignore limitNamespaces and allowAccessToSystemNamespaces in this entry
			} else {
				dirEntry.NamespaceSelectors = append(dirEntry.NamespaceSelectors, crd.Spec.NamespaceSelector)
			}

			directory[kind][name] = dirEntry
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.directory = directory
	h.logger.Println("configuration was reloaded successfully")
}

func isLabelSelectorApplied(namespaceSelector *NamespaceSelector) bool {
	if namespaceSelector != nil && namespaceSelector.LabelSelector != nil {
		return true
	}

	return false
}

// StartRenewConfigLoop periodically reads new config file from the file system and composes directories.
func (h *Handler) StartRenewConfigLoop(stopCh <-chan struct{}) {
	h.renewDirectories()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.renewDirectories()
		case <-stopCh:
			h.logger.Println("renew directories stopped")
			return
		}
	}
}

// affectedDirs checks that User/Group/ServiceAccount from the review request has corresponding ClusterAuthorizationRules
func (h *Handler) affectedDirs(r *WebhookRequest) []DirectoryEntry {
	var dirEntriesAffected []DirectoryEntry

	h.mu.RLock()
	defer h.mu.RUnlock()

	if dirEntry, ok := h.directory["User"][r.Spec.User]; ok {
		dirEntriesAffected = append(dirEntriesAffected, dirEntry)
	}

	if dirEntry, ok := h.directory["ServiceAccount"][r.Spec.User]; ok {
		dirEntriesAffected = append(dirEntriesAffected, dirEntry)
	}

	for _, group := range r.Spec.Group {
		if dirEntry, ok := h.directory["Group"][group]; ok {
			dirEntriesAffected = append(dirEntriesAffected, dirEntry)
		}
	}

	return dirEntriesAffected
}

// checks if labels of a namespace match provided labelselector
func (h *Handler) namespaceLabelsMatchSelector(namespaceName string, namespaceSelectors []*NamespaceSelector) (bool, error) {
	var labelsSet labels.Set
	namespace, err := h.kubeclient.CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	labelsSet = namespace.ObjectMeta.GetLabels()

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

// checks if an entry has any namespace-related filters
func hasAnyFilters(entry *DirectoryEntry) bool {
	// check for MatchAny field of any namespace selector which permits literally any namespace
	for _, namespaceSelector := range entry.NamespaceSelectors {
		if namespaceSelector.MatchAny {
			return false
		}
	}

	if entry.NamespaceFiltersAbsent {
		// The limitNamespaces option has a priority over the allowAccessToSystemNamespaces option.
		// If limited namespaces are not specified, check whether access to system namespaces is limited.
		// If it is not - user has no limited namespaces.
		return !entry.AllowAccessToSystemNamespaces
	}

	for _, regex := range entry.LimitNamespaces {
		switch regex.String() {
		// Special regexp cases that allow every namespace. Do not need to forbid cluster scoped requests.
		case "^.*$", "^.+$":
			return !entry.AllowAccessToSystemNamespaces
		}
	}

	return true
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
