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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"user-authz-webhook/cache"
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
		// this case is exceptional
		h.logger.Fatalf("cannot unmarshal kubernetes request: %v", err)
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
	if !hasLimitedNamespaces(entry) {
		// User has no namespaced restriction.
		return request
	}

	// Check if the request Namespace is in the system namespaces list
	if namespaceIsSystem(request.Spec.ResourceAttributes.Namespace, systemNamespacesRegex) {
		// Deny request if there is no AllowAccessToSystemNamespaces
		if !entry.AllowAccessToSystemNamespaces {
			request.Status.Denied = true
			request.Status.Reason = noNamespaceAccessReason
			// Allow request if there is AllowAccessToSystemNamespaces
		} else {
			request.Status.Denied = false
			request.Status.Reason = ""
		}
		return request
	}

	// If the limit namespaces option missed at least for one directory, requests for all remaining (non-system) namespaces are allowed
	if entry.LimitNamespacesAbsent {
		request.Status.Denied = false
		return request
	}

	// All namespaces are denied
	request.Status.Denied = true
	request.Status.Reason = noNamespaceAccessReason

	// Firstly, we check if the target namespace is in limitNamespaces list (plus system namespaces if AllowAccessToSystemNamespace == true) because it's cheaper/faster in terms of requests to API
	for _, pattern := range entry.LimitNamespaces {
		if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
			request.Status.Denied = false
			request.Status.Reason = ""
			break
		}
	}

	// Secondly, one namespace selector at a time, we get the lists of namespaces matching namespaceSelectors labels and check against them
	if request.Status.Denied && len(entry.NamespaceSelectors) > 0 {
		for _, namespaceSelector := range entry.NamespaceSelectors {
			namespaces, err := h.getNamespacesByLabelSelector(*namespaceSelector)
			if err != nil {
				request.Status.Reason = err.Error()
				// not sure if we should stop processing the request in case there are some api's client-related issues
				continue
			}
			// check if the requested namespace is in the map of namespaces by labels
			if _, ok := namespaces[request.Spec.ResourceAttributes.Namespace]; ok {
				request.Status.Denied = false
				request.Status.Reason = ""
				break
			}
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

	} else if namespaced && hasLimitedNamespaces(entry) {
		// we should not allow cluster scoped requests for namespaced objects if namespaces access is limited
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
		combinedDir.LimitNamespacesAbsent = combinedDir.LimitNamespacesAbsent || dirEntry.LimitNamespacesAbsent
	}

	if request.Spec.ResourceAttributes.Namespace != "" {
		return h.authorizeNamespacedRequest(request, &combinedDir)
	}

	if request.Spec.ResourceAttributes.Resource != "" {
		return h.authorizeClusterScopedRequest(request, &combinedDir)
	}

	return request
}

// renewDirectories reads a configuration file (actually it is a json file with all CRs from the cluster) and composes
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

			// If there are neither LimitNamespaces nor NamespaceSelector options, it means all namespaces are allowed except system namespaces.
			// We need to know whether we have at least one such a CR for the user in a cluster.
			dirEntry.LimitNamespacesAbsent = dirEntry.LimitNamespacesAbsent || (len(crd.Spec.LimitNamespaces) == 0 && crd.Spec.NamespaceSelector == nil)

			// if NamespaceSelector is empty - take limitNamespaces entries
			if crd.Spec.NamespaceSelector == nil {
				// This is an important thing! All regular expressions is wrapped in the ^...$
				for _, ln := range crd.Spec.LimitNamespaces {
					r, _ := regexp.Compile(wrapRegex(ln))
					dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
				}
				// if NamespaceSelector is not empty - drop limitNamespaces entries
			} else {
				dirEntry.NamespaceSelectors = append(dirEntry.NamespaceSelectors, crd.Spec.NamespaceSelector)
			}

			if !dirEntry.AllowAccessToSystemNamespaces {
				dirEntry.AllowAccessToSystemNamespaces = crd.Spec.AllowAccessToSystemNamespaces
			}

			directory[kind][name] = dirEntry
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.directory = directory
	h.logger.Println("configuration was reloaded successfully")
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

// getsthe map of namespaces matching specific label selector from k8s api client
func (h *Handler) getNamespacesByLabelSelector(namespaceSelector NamespaceSelector) (map[string]struct{}, error) {
	namespaces := make(map[string]struct{})

	getNamespaces, err := h.kubeclient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(namespaceSelector.LabelSelector)})
	if err != nil {
		return nil, err
	}

	for _, namespace := range getNamespaces.Items {
		namespaces[namespace.Name] = struct{}{}
	}
	return namespaces, nil
}

// checks if a namespace name matches any system namespace regex
func namespaceIsSystem(namespace string, systemNamespacesRegex []*regexp.Regexp) bool {
	for _, pattern := range systemNamespacesRegex {
		if pattern.MatchString(namespace) {
			return true
		}
	}
	return false
}

func hasLimitedNamespaces(entry *DirectoryEntry) bool {
	if (len(entry.LimitNamespaces) == 0 && len(entry.NamespaceSelectors) == 0) || entry.LimitNamespacesAbsent {
		// The limitNamespaces option has a priority over the allowAccessToSystemNamespaces option.
		// If limited namespaces are not specified, check whether access to system namespaces is limited.
		// If it is not - user has no limited namespaces.
		return !entry.AllowAccessToSystemNamespaces
	}

	// if entry has NamespaceSelectors list, it's limited.
	if len(entry.NamespaceSelectors) > 0 {
		return true
	}

	for _, regex := range entry.LimitNamespaces {
		switch regex.String() {
		// Special regexp cases that allow every namespace. Do not need to forbid cluster scoped requests.
		case "^.*$", "^.+$":
			return false
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
