/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/decision"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"webhook/internal/cache"
)

const (
	// The reasons live in the library, which permission-browser answers with too; the local names
	// keep the tests and the log lines readable.
	internalErrorReason          = decision.InternalErrorReason
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

	// restrictions bounds how often the ordering guard is logged.
	restrictions decision.RestrictionLog

	// logDecisions says which answered reviews are written to the log in full.
	logDecisions decisionLogMode
}

// decisionLogMode selects which SubjectAccessReviews are written to the log with their full body.
//
// Every review used to be written - identity, groups, resource, answer - at the default verbosity,
// with nothing to turn it off: about 125 KB a minute from a 5 rps probe on a stand, and on a
// master every authorization question in the cluster passes through here. Denials are what an
// operator looks for in this log; allowed and no-opinion answers are background, and are opt-in.
type decisionLogMode int

const (
	// logDenied writes the reviews this webhook denied. The default.
	logDenied decisionLogMode = iota
	// logAll writes every review, as before.
	logAll
	// logNone writes no review bodies at all.
	logNone
)

// DecisionLogEnv names the environment variable that selects the mode: none, denied or all.
const DecisionLogEnv = "LOG_DECISIONS"

// DecisionLogModeFrom parses the value of DecisionLogEnv. An empty value is the default; anything
// unknown is the default too, said once in the log so a typo does not pass for a setting.
func DecisionLogModeFrom(value string, logger *log.Logger) decisionLogMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "denied":
		return logDenied
	case "all":
		return logAll
	case "none":
		return logNone
	}
	logger.Printf("%s=%q is not one of none, denied, all; logging denied reviews", DecisionLogEnv, value)
	return logDenied
}

// logs reports whether a review with the given answer is written.
func (m decisionLogMode) logs(denied bool) bool {
	switch m {
	case logAll:
		return true
	case logDenied:
		return denied
	}
	return false
}

func (m decisionLogMode) String() string {
	switch m {
	case logAll:
		return "all"
	case logNone:
		return "none"
	}
	return "denied"
}

// NewHandler wires the handler. rulesProvider and bindings are required: without the rules the
// webhook has no opinion about anybody, and without the bindings it cannot tell a subject nobody
// limits from a subject whose rule it has not observed yet.
func NewHandler(logger *log.Logger, discoveryCache cache.Cache, nsLister corev1listers.NamespaceLister, nsSynced kcache.InformerSynced,
	independentRBAC independentRBACResolver, rulesProvider RulesProvider, bindings RuleBindings, logDecisions decisionLogMode) (*Handler, error) {
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
		logDecisions:    logDecisions,
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

	if h.logDecisions.logs(request.Status.Denied) {
		h.logger.Printf("response body: %s", respData)
	}
}

// authorizeRequest asks the shared decision and writes its answer onto the review.
//
// The sequence lives in go_lib/user-authz/decision, which permission-browser calls with the same
// inputs. What the API server enforces and what the UI reports are the same question; they used to
// be two implementations of it, and they drifted.
func (h *Handler) authorizeRequest(request *WebhookRequest) *WebhookRequest {
	attrs := request.Spec.ResourceAttributes

	result := decision.Authorize(decision.Request{
		User:      request.Spec.User,
		Groups:    request.Spec.Group,
		Namespace: attrs.Namespace,
		Resource:  attrs.Resource,
		APIGroup:  attrs.Group,
	}, decision.Sources{
		Directory:       h.rules.Directory(),
		Bindings:        h.bindings,
		NamespaceLabels: h.namespaceLabels,
		ResourceScope:   h.resourceScope(attrs.Version),
		IndependentRBAC: func() bool { return h.independentRBACAllows(request) },
		Logf:            h.logger.Printf,
		OnRestricted:    h.reportRestricted,
	})

	if result.Denied() {
		request.Status.Denied = true
		request.Status.Reason = result.Reason
	}
	return request
}

// resourceScope asks discovery what a resource is. It returns an error only when the question could
// not be answered; a resource discovery reports as nonexistent comes back as a scope with Absent
// set, so the caller lets RBAC reply and the API server produces the 404 the caller is owed rather
// than a 403 about a resource that was never there.
//
// requestedVersion is the version from the review. The API server always fills it for real traffic;
// a SubjectAccessReview written by hand may leave it out, and then the preferred one is resolved.
func (h *Handler) resourceScope(requestedVersion string) decision.ResourceScopeFunc {
	return func(group, resource string) (rules.ResourceScope, error) {
		apiVersion := requestedVersion
		if apiVersion == "" || apiVersion == "*" {
			if group == "" {
				apiVersion = "v1"
			} else {
				preferred, err := h.cache.GetPreferredVersion(group, resource)
				if err != nil {
					if absent(err) {
						return rules.ResourceScope{Absent: true}, nil
					}
					return rules.ResourceScope{}, err
				}
				apiVersion = preferred
			}
		}

		apiGroup := apiVersion
		if group != "" {
			apiGroup = group + "/" + apiVersion
		}

		namespaced, err := h.cache.Get(apiGroup, resource)
		if err != nil {
			if absent(err) {
				return rules.ResourceScope{Absent: true}, nil
			}
			return rules.ResourceScope{}, err
		}
		return rules.ResourceScope{Known: true, Namespaced: namespaced}, nil
	}
}

// absent reports whether the error means discovery answered and the resource is not there: the API
// server 404'd the group, or a successful listing of it does not carry the resource.
func absent(err error) bool {
	return errors.Is(err, cache.ErrNotFound) || errors.Is(err, cache.ErrResourceAbsent)
}

// reportRestricted logs the ordering guard, as often as it is worth saying and no more. When that
// is - once when a rule starts restricting, hourly while it goes on, again when it happens afresh
// - belongs with the guard itself, so it lives in the library and both consumers share it.
func (h *Handler) reportRestricted(username, rule string) {
	if !h.restrictions.Allow(rule) {
		return
	}
	h.logger.Printf("rule %q binds subjects the webhook has not observed it naming (first seen for %q; rules synced: %v); restricting them until the rule arrives", rule, username, h.rules.HasSynced())
}

// namespaceLabels returns the labels of a namespace for the namespaceSelector check.
//
// It refuses while the namespace cache is still filling. An empty cache answers "no such
// namespace" for every name, which is indistinguishable from a namespace that genuinely has no
// labels - and a selector like DoesNotExist matches that, so a selector would open namespaces it
// was never evaluated against. The error denies instead. permission-browser's engine has always
// checked this; the webhook stored the predicate and never consulted it.
func (h *Handler) namespaceLabels(namespaceName string) (labels.Set, error) {
	if h.nsLister == nil {
		return nil, fmt.Errorf("namespace lister is not initialized")
	}
	if h.nsSynced != nil && !h.nsSynced() {
		return nil, fmt.Errorf("namespace cache is not synced yet")
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
