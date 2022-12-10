/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"user-authz-webhook/cache"
)

const (
	configPath = "/etc/user-authz-webhook/config.json"

	noNamespaceAccessReason      = "user has no access to the namespace"
	namespaceLimitedAccessReason = "making cluster scoped requests for namespaced resources are not allowed"
	internalErrorReason          = "webhook: kubernetes api request error"
)

var _ http.Handler = (*Handler)(nil)

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger *log.Logger

	lastAppliedStat os.FileInfo

	cache cache.Cache

	//        [user type] [user name]
	mu        sync.RWMutex
	directory map[string]map[string]DirectoryEntry
}

func NewHandler(logger *log.Logger, discoveryCache cache.Cache) *Handler {
	return &Handler{
		logger: logger,
		cache:  discoveryCache,
	}
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
		// User has no namespaces restriction.
		return request
	}

	// If the limit namespaces option missed at least for one directory, requests for all namespaces are allowed
	// except the system namespaces.
	if entry.LimitNamespacesAbsent {
		request.Status.Denied = false

		if !entry.AllowAccessToSystemNamespaces {
			for _, pattern := range systemNamespacesRegex {
				// Deny if matching one of system namespaces regexps
				if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
					request.Status.Denied = true
					request.Status.Reason = noNamespaceAccessReason
					break
				}
			}
		}

		return request
	}

	// If the limit namespaces option is enabled, we must check that user requests only affect these namespaces.
	// If the system namespaces access option is enabled, we should also count system namespaces as the allowed ones.
	if entry.AllowAccessToSystemNamespaces {
		entry.LimitNamespaces = append(entry.LimitNamespaces, systemNamespacesRegex...)
	}

	// All namespaces are denied except namespaces from the limit namespaces list.
	request.Status.Denied = true
	request.Status.Reason = noNamespaceAccessReason

	for _, pattern := range entry.LimitNamespaces {
		if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
			request.Status.Denied = false
			request.Status.Reason = ""
			break
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

		combinedDir.LimitNamespaces = append(combinedDir.LimitNamespaces, dirEntry.LimitNamespaces...)
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

			// If there is no LimitNamespaces option, it means all namespaces are allowed except system namespaces.
			// We need to know whether we have at least one such CR for the user in a cluster.
			dirEntry.LimitNamespacesAbsent = dirEntry.LimitNamespacesAbsent || len(crd.Spec.LimitNamespaces) == 0

			// This is an important thing! All regular expressions is wrapped in the ^...$
			for _, ln := range crd.Spec.LimitNamespaces {
				r, _ := regexp.Compile(wrapRegex(ln))
				dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
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

func hasLimitedNamespaces(entry *DirectoryEntry) bool {
	if len(entry.LimitNamespaces) == 0 || entry.LimitNamespacesAbsent {
		// The limitNamespaces option has a priority over the allowAccessToSystemNamespaces option.
		// If limited namespaces are not specified, check whether access to system namespaces is limited.
		// If it is not - user has no limited namespaces.
		return !entry.AllowAccessToSystemNamespaces
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
