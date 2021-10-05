/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"user-authz-webhook/cache"
)

const (
	configPath = "/etc/user-authz-webhook/config.json"

	noNamespaceAccessReason      = "user has no access to the namespace"
	namespaceLimitedAccessReason = "making cluster scoped requests for namespaced resources is not allowed"
	internalErrorReason          = "webhook: kubernetes api request error"
)

var _ http.Handler = (*Handler)(nil)

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger *log.Logger

	appliedConfigMtime int64

	cache cache.Cache

	//        [user type] [user name]
	mu        sync.RWMutex
	directory map[string]map[string]DirectoryEntry
}

func NewHandler(logger *log.Logger) *Handler {
	return &Handler{
		logger: logger,
		cache:  cache.NewNamespacedDiscoveryCache(logger),
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

func (h *Handler) authorizeNamespacedRequest(request *WebhookRequest, limitNamespaces []*regexp.Regexp, systemAccess bool) *WebhookRequest {
	if len(limitNamespaces) == 0 {
		if systemAccess {
			// User has no namespaces restriction.
			return request
		}

		// If there is no restriction for system namespaces,
		// we need to check that a user doesn't request resources from the system namespace.

		// All namespaces are allowed except system namespaces.
		request.Status.Denied = false

		for _, pattern := range systemNamespacesRegex {
			if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
				request.Status.Denied = true
				request.Status.Reason = noNamespaceAccessReason
				break
			}
		}

		return request
	}

	// If the limit namespaces option is enabled, we must check that user requests only affect these namespaces.
	// If the system namespaces access option is enabled, we should also count system namespaces as the allowed ones.
	if systemAccess {
		limitNamespaces = append(limitNamespaces, systemNamespacesRegex...)
	}

	// All namespaces are denied except namespaces from the limit namespaces list.
	request.Status.Denied = true
	request.Status.Reason = noNamespaceAccessReason

	for _, pattern := range limitNamespaces {
		if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
			request.Status.Denied = false
			request.Status.Reason = ""
			break
		}
	}

	return request
}

func (h *Handler) authorizeClusterScopedRequest(request *WebhookRequest, hasLimitedNamespaces bool) *WebhookRequest {
	// if resource is not nil and namespace is nil
	apiGroup := request.Spec.ResourceAttributes.Version
	group := request.Spec.ResourceAttributes.Group

	if group != "" {
		apiGroup = group + "/" + apiGroup
	}

	namespaced, err := h.cache.Get(apiGroup, request.Spec.ResourceAttributes.Resource)
	if err != nil {
		// could not check whether resource is namespaced or not (from cache) - deny access
		h.logger.Println(err)

		request.Status.Denied = true
		request.Status.Reason = internalErrorReason

	} else if namespaced && hasLimitedNamespaces {
		// we should not allow cluster scoped requests for namespaced objects if namespaces access is limited
		request.Status.Denied = true
		request.Status.Reason = namespaceLimitedAccessReason
	}

	return request
}

func (h *Handler) authorizeRequest(request *WebhookRequest) *WebhookRequest {
	dirEntriesAffected := h.affectedDirs(request)
	if len(dirEntriesAffected) == 0 {
		return request
	}

	var (
		limitNamespaces          []*regexp.Regexp
		accessToSystemNamespaces bool
	)

	for _, dirEntry := range dirEntriesAffected {
		if !accessToSystemNamespaces {
			accessToSystemNamespaces = dirEntry.AllowAccessToSystemNamespaces
		}

		limitNamespaces = append(limitNamespaces, dirEntry.LimitNamespaces...)
	}

	if request.Spec.ResourceAttributes.Namespace != "" {
		return h.authorizeNamespacedRequest(request, limitNamespaces, accessToSystemNamespaces)
	}

	if request.Spec.ResourceAttributes.Resource != "" {
		hasLimitedNamespaces := !accessToSystemNamespaces || emptyOrContainsAllMatchingRegex(limitNamespaces)
		return h.authorizeClusterScopedRequest(request, hasLimitedNamespaces)
	}

	return request
}

func (h *Handler) renewDirectories() {
	fStat, err := os.Stat(configPath)
	if err != nil {
		h.logger.Printf("cannot reload the config: %v", err)
		return
	}

	mtime := fStat.ModTime().Unix()
	if mtime == h.appliedConfigMtime {
		return
	}

	h.appliedConfigMtime = mtime
	var config UserAuthzConfig

	configRawData, err := ioutil.ReadFile(configPath)
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

			for _, ln := range crd.Spec.LimitNamespaces {
				r, _ := regexp.Compile("^" + ln + "$")
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
}

func (h *Handler) StartRenewConfigLoop(stopCh <-chan struct{}) {
	h.renewDirectories()

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				h.renewDirectories()
			case <-stopCh:
				return
			}
		}
	}()
}

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

func emptyOrContainsAllMatchingRegex(regexes []*regexp.Regexp) bool {
	if len(regexes) == 0 {
		return false
	}
	for _, regex := range regexes {
		switch regex.String() {
		case ".*", ".+":
			return false
		}
	}

	return true
}
