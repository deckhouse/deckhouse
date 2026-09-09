/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"webhook/internal/cache"
)

const (
	internalErrorReason = "webhook: kubernetes api request error"

	// The reasons are shared with permission-browser through the library; the local names keep the
	// existing tests and log lines readable.
	noNamespaceAccessReason      = rules.NoNamespaceAccessReason
	namespaceLimitedAccessReason = rules.NamespaceLimitedAccessReason
)

var _ http.Handler = (*Handler)(nil)

// RulesProvider hands out the current directory of ClusterAuthorizationRules. Directory is nil until
// the rules have been listed once; the handler treats that as "the rules are not known", never as
// "there are no rules".
type RulesProvider interface {
	Directory() *rules.Directory
	HasSynced() bool
}

// RuleBindings answers which ClusterAuthorizationRules bind a user through the ClusterRoleBindings
// user-authz-controller created for them. It is the evidence the handler uses to notice that its
// directory lags behind the controller.
type RuleBindings interface {
	RulesFor(username string, groups []string) []string
}

// Handler is a main entrypoint for the webhook
type Handler struct {
	logger *log.Logger

	cache cache.Cache

	nsLister corev1listers.NamespaceLister
	nsSynced kcache.InformerSynced

	// independentRBAC, when set, is consulted before denying a request:
	// requests explicitly granted by CAR-independent RBAC (RoleBindings,
	// non-CAR ClusterRoleBindings) must not be denied by multi-tenancy
	// filters. See RBACEvaluator for details.
	independentRBAC independentRBACResolver

	rules    RulesProvider
	bindings RuleBindings

	// reported remembers the (subject, rule) pairs the ordering guard has already logged.
	reported sync.Map
}

// NewHandler wires the handler. rulesProvider and bindings are required: without the rules the
// webhook has no opinion about anybody, and without the bindings it cannot tell a subject nobody
// limits from a subject whose rule it has not observed yet.
func NewHandler(logger *log.Logger, discoveryCache cache.Cache, nsLister corev1listers.NamespaceLister, nsSynced kcache.InformerSynced,
	independentRBAC independentRBACResolver, rulesProvider RulesProvider, bindings RuleBindings) (*Handler, error) {
	if rulesProvider == nil {
		return nil, fmt.Errorf("rules provider is required")
	}
	if bindings == nil {
		return nil, fmt.Errorf("rule bindings index is required")
	}
	return &Handler{
		logger:          logger,
		cache:           discoveryCache,
		nsLister:        nsLister,
		nsSynced:        nsSynced,
		independentRBAC: independentRBAC,
		rules:           rulesProvider,
		bindings:        bindings,
	}, nil
}

// independentRBACAllows reports whether CAR-independent RBAC explicitly
// grants the request.
func (h *Handler) independentRBACAllows(request *WebhookRequest) bool {
	if h.independentRBAC == nil {
		return false
	}
	return h.independentRBAC.AllowsIndependently(&request.Spec)
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
	if _, err := w.Write(respData); err != nil {
		h.logger.Printf("failed to write response: %v", err)
	}

	h.logger.Printf("response body: %s", respData)
}

func (h *Handler) authorizeNamespacedRequest(request *WebhookRequest, entry *rules.Entry) *WebhookRequest {
	if !entry.HasAnyFilters() {
		// User has no namespaced restriction.
		return request
	}

	allowed, err := rules.NamespaceAllowed(entry, request.Spec.ResourceAttributes.Namespace, h.namespaceLabels)
	if err != nil {
		// The lookup fails for a namespace that does not exist as well as for a
		// genuine cache problem. Neither may reach the user: `namespaces "x" not
		// found` as the denial reason disclosed exactly what the deny is meant to
		// hide. The operator still gets the detail from the log.
		h.logger.Printf("namespace selector check for %q failed: %v", request.Spec.ResourceAttributes.Namespace, err)
	}
	if allowed {
		return request
	}

	// The namespace is outside the user's CAR scope, but the request may be
	// explicitly granted by CAR-independent RBAC: a RoleBinding in the
	// namespace (including RoleBindings rendered from AuthorizationRules) or
	// a ClusterRoleBinding not generated from a CAR. Such grants exist
	// regardless of any CAR and must not be denied here; the final decision
	// is still made by the RBAC authorizer after this webhook.
	if h.independentRBACAllows(request) {
		return request
	}

	request.Status.Denied = true
	request.Status.Reason = rules.NoNamespaceAccessReason
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

func (h *Handler) authorizeClusterScopedRequest(request *WebhookRequest, entry *rules.Entry) *WebhookRequest {
	if !entry.HasAnyFilters() {
		return request
	}

	// if resource is not nil and namespace is nil
	apiVersion := request.Spec.ResourceAttributes.Version
	group := request.Spec.ResourceAttributes.Group
	resource := request.Spec.ResourceAttributes.Resource
	var apiGroup string

	if apiVersion == "" || apiVersion == "*" {
		if group != "" {
			var err error
			apiVersion, err = h.cache.GetPreferredVersion(group, resource)
			if err != nil {
				// could not check whether resource is namespaced or not (from cache) - deny access
				return h.fillDenyRequest(request, internalErrorReason, err.Error())
			}
		} else {
			k8sCoreResources, err := h.cache.GetCoreResources()
			if err != nil {
				return h.fillDenyRequest(request, internalErrorReason, err.Error())
			}
			// A core resource absent from the core snapshot does not exist: let RBAC decide.
			if _, found := k8sCoreResources[resource]; !found {
				return request
			}

			// apiVersion and group both empty, which means that this is a core Kubernetes resource
			apiVersion = "v1"
		}
	}

	if group != "" {
		apiGroup = group + "/" + apiVersion
	} else {
		apiGroup = apiVersion
	}

	namespaced, err := h.cache.Get(apiGroup, resource)
	if err != nil {
		// could not check whether resource is namespaced or not (from cache) - deny access
		return h.fillDenyRequest(request, internalErrorReason, err.Error())
	}

	if !rules.ClusterScopedDenied(rules.ResourceScope{Known: true, Namespaced: namespaced}) {
		return request
	}
	// Cluster-scoped access to a namespaced resource granted by a non-CAR
	// ClusterRoleBinding is a deliberate cluster-wide grant; do not deny it.
	if h.independentRBACAllows(request) {
		return request
	}
	// we should not allow cluster-scoped requests for the namespaced objects if access to the namespaces is limited
	return h.fillDenyRequest(request, rules.NamespaceLimitedAccessReason, "")
}

func (h *Handler) authorizeRequest(request *WebhookRequest) *WebhookRequest {
	entries := h.affectedEntries(request)
	if len(entries) == 0 {
		return request
	}

	// Combine dirs for the current request. Users may have more than one rule attached to their groups or usernames.
	combined := rules.Combine(entries)

	if request.Spec.ResourceAttributes.Namespace != "" {
		return h.authorizeNamespacedRequest(request, &combined)
	}

	if request.Spec.ResourceAttributes.Resource != "" {
		return h.authorizeClusterScopedRequest(request, &combined)
	}

	return request
}

// affectedEntries collects the directory entries of the User/ServiceAccount/Groups of the request.
//
// It also applies the ordering guard: user-authz-controller creates and updates the
// ClusterRoleBindings of a rule around the same time as the rule, and the rule and its bindings
// reach this webhook over independent watches, so the bindings can be ahead. When a binding says it
// binds the subject to a rule whose observed copy does not name that subject (the rule is unknown,
// or known but not yet updated), the subject gets a maximally restricted entry, so the cluster-wide
// binding never grants more than the rule will. The restriction lifts by itself when the rule
// arrives.
//
// The restricted entry only bites a subject that has no observed entry of its own: rules union, so
// an entry the webhook has already observed stays as wide as it is (see rules.Combine). That is
// deliberate - an unobserved rule may only widen a subject's scope, so clamping what is already
// known would deny access the observed rules legitimately grant.
func (h *Handler) affectedEntries(r *WebhookRequest) []rules.Entry {
	dir := h.rules.Directory()
	entries := dir.Lookup(r.Spec.User, r.Spec.Group)

	for _, rule := range h.bindings.RulesFor(r.Spec.User, r.Spec.Group) {
		if dir.RuleCovers(rule, r.Spec.User, r.Spec.Group) {
			continue
		}
		h.reportRestricted(r.Spec.User, rule)
		entries = append(entries, rules.Restricted())
		break
	}

	return entries
}

// reportRestricted logs the ordering guard once per subject and rule. The guard is evaluated on
// every request, and an orphaned rule binding would otherwise log on every request of its subject,
// forever - on the authorization path, behind the logger's process-wide lock.
func (h *Handler) reportRestricted(username, rule string) {
	key := username + "\x00" + rule
	if _, seen := h.reported.Load(key); seen {
		return
	}
	if _, loaded := h.reported.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	h.logger.Printf("user %q is bound by rule %q the webhook has not observed binding it (rules synced: %v); restricting until the rule arrives", username, rule, h.rules.HasSynced())
}

// namespaceLabels returns the labels of a namespace for the namespaceSelector check.
func (h *Handler) namespaceLabels(namespaceName string) (labels.Set, error) {
	if h.nsLister == nil {
		return nil, fmt.Errorf("namespace lister is not initialized")
	}

	namespace, err := h.nsLister.Get(namespaceName)
	if err != nil {
		return nil, err
	}

	set := labels.Set(namespace.GetLabels())
	if set == nil {
		set = labels.Set{}
	}
	return set, nil
}
