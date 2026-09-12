/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"io"
	"log"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"webhook/internal/cache"
)

// staticRules is a RulesProvider over a directory built once from a fixed set of rules.
type staticRules struct {
	dir *rules.Directory
}

func (s staticRules) Directory() *rules.Directory { return s.dir }
func (s staticRules) HasSynced() bool             { return s.dir != nil }

func rulesFor(rs ...rules.Rule) staticRules {
	dir, _ := rules.NewBuilder().Build(rs)
	return staticRules{dir: dir}
}

func groupRule(name, group string, limit []string, system bool) rules.Rule {
	return rules.Rule{Name: name, Subjects: []rules.Subject{{Kind: "Group", Name: group}}, LimitNamespaces: limit, AllowAccessToSystemNamespaces: system}
}

// fixtureRules mirror the directory the tests used to assemble by hand: one rule per group, named
// after the scope it grants.
func fixtureRules() staticRules {
	selector := &rules.NamespaceSelector{LabelSelector: &metav1.LabelSelector{
		MatchLabels: map[string]string{"match": "true"},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "expression", Operator: "In", Values: []string{"match", "allow"}},
		},
	}}
	return rulesFor(
		groupRule("normal", "normal", nil, false),
		groupRule("system-allowed", "system-allowed", nil, true),
		groupRule("limited", "limited", []string{"test-.*"}, false),
		groupRule("limited-with-system-regex", "limited-with-system-regex", []string{"d8-.*"}, false),
		groupRule("limited-with-unlimited-regex", "limited-with-unlimited-regex", []string{".*"}, false),
		groupRule("limited-and-system-allowed", "limited-and-system-allowed", []string{"test-.*"}, true),
		groupRule("limited-with-unlimited-regex-and-system-allowed", "limited-with-unlimited-regex-and-system-allowed", []string{".*"}, true),
		rules.Rule{Name: "limited-namespace-selector", Subjects: []rules.Subject{{Kind: "Group", Name: "limited-namespace-selector"}}, NamespaceSelector: selector},
		rules.Rule{Name: "limited-with-match-any-namespace-selector", Subjects: []rules.Subject{{Kind: "Group", Name: "limited-with-match-any-namespace-selector"}}, NamespaceSelector: &rules.NamespaceSelector{MatchAny: true}},
	)
}

func fixtureCache() *dummyCache {
	return &dummyCache{
		data: map[string]map[string]bool{
			"test/v1": {
				"object1": true,
				"object2": false,
			},
			"v1": {
				"namespaces": false,
				"services":   true,
			},
		},
		preferredVersions: map[string]string{
			"object2.test": "v1",
			"object1.test": "v1",
		},
		coreResources: cache.CoreResourcesDict{
			"pods":       struct{}{},
			"namespaces": struct{}{},
			"services":   struct{}{},
		},
	}
}

func TestAuthorizeRequest(t *testing.T) {
	tc := []struct {
		Name         string
		Group        []string
		Attributes   WebhookResourceAttributes
		ResultStatus WebhookRequestStatus
		Namespaces   []runtime.Object
	}{
		{
			Name:  "Namespaced",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Non-existent Namespaced",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Resource:  "faketest",
				Namespace: "test",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Non-existent Clusterscoped",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Resource: "faketest",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Restricted",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Namespaced Restricted System Allowed",
			Group: []string{"system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced One Not Limited Group And One Restricted",
			Group: []string{"limited", "system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "not-allowed-by-default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Limited",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test-abc-def",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Limited and denied namespace",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "any-other-namespace",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Namespaced Limited with unlimited namespace regex",
			Group: []string{"limited-with-unlimited-regex"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "any-other-namespace",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced limited with system",
			Group: []string{"limited-and-system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Cluster scoped. Group and version are empty, search in the v1 apiVersion. Allowed.",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "",
				Version:   "",
				Resource:  "namespaces",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Group and version are empty, search in the v1 apiVersion. Denied.",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "",
				Version:   "",
				Resource:  "services",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object2",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Without version. Version exists",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "",
				Resource:  "object2",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Without version. Version does not exists",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "not.exists",
				Version:   "",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "webhook: kubernetes api request error",
			},
		},
		{
			Name:  "ClusterScoped but namespaced",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped but namespaced and all namespaces are allowed",
			Group: []string{"system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "ClusterScoped but namespaced and limited namespaces",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped One Not Limited Group And One Restricted",
			Group: []string{"limited", "system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:         "Non Resource allowed",
			Group:        []string{"normal"},
			Attributes:   WebhookResourceAttributes{},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Limited with NamespaceSelectors and namespace doesn't exist",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and labels match",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match":      "true",
							"expression": "allow",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and labels don't match",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match": "false",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, and matches limitNamespaces",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test-abc-def",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, and matches NamespaceSelectors",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match":      "true",
							"expression": "match",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces (system regex), wants d8-system namespace without AllowAccessToSystemNamespaces",
			Group: []string{"limited-with-system-regex", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "d8-system",
						Labels: map[string]string{},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system namespace without AllowAccessToSystemNamespaces",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "d8-system",
						Labels: map[string]string{},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system namespace without AllowAccessToSystemNamespaces but the namespace has the labels",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match":      "true",
							"expression": "match",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system with AllowAccessToSystemNamespaces",
			Group: []string{"limited-and-system-allowed", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match":      "true",
							"expression": "allow",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system with AllowAccessToSystemNamespaces but labels don't match",
			Group: []string{"limited-and-system-allowed", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match": "true",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: noNamespaceAccessReason,
			},
		},
		{
			Name:  "Limited with MatchAny NamespaceSelector and limitNamespaces, wants d8-system",
			Group: []string{"limited", "limited-with-match-any-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with MatchAny NamespaceSelector and limitNamespaces, wants across all namespaces",
			Group: []string{"limited", "limited-with-match-any-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelector, wants across all namespaces",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "Limited with NamespaceSelector and unlimited limitNamespaces, wants across all namespaces",
			Group: []string{"limited-namespace-selector", "limited-with-unlimited-regex"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "Limited with NamespaceSelector and unlimited limitNamespaces plus system, wants across all namespaces",
			Group: []string{"limited-namespace-selector", "limited-with-unlimited-regex-and-system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			handler := &Handler{
				logger:   log.New(io.Discard, "", 0),
				cache:    fixtureCache(),
				rules:    fixtureRules(),
				bindings: binding.NewIndex(),
				nsLister: newFakeNamespaceLister(testCase.Namespaces),
				nsSynced: func() bool { return true },
			}

			req := &WebhookRequest{
				Spec: WebhookResourceSpec{
					User:               "test",
					Group:              testCase.Group,
					ResourceAttributes: testCase.Attributes,
				},
				Status: WebhookRequestStatus{},
			}

			req = handler.authorizeRequest(req)
			if req.Status.Denied != testCase.ResultStatus.Denied {
				t.Errorf("denied: got %v | expected %v", req.Status.Denied, testCase.ResultStatus.Denied)
			}

			if req.Status.Reason != testCase.ResultStatus.Reason {
				t.Errorf("reason: got %q | expected %q", req.Status.Reason, testCase.ResultStatus.Reason)
			}
		})
	}
}

type dummyCache struct {
	data              map[string]map[string]bool
	preferredVersions map[string]string
	coreResources     cache.CoreResourcesDict
}

func (d *dummyCache) Get(api, key string) (bool, error) {
	return d.data[api][key], nil
}

func (d *dummyCache) GetCoreResources() (cache.CoreResourcesDict, error) {
	return d.coreResources, nil
}

func (d *dummyCache) GetPreferredVersion(group, resource string) (string, error) {
	if v, ok := d.preferredVersions[fmt.Sprintf("%s.%s", resource, group)]; ok {
		return v, nil
	}

	return "", fmt.Errorf("not found")
}

func (d *dummyCache) Check() error {
	return nil
}

type fakeNamespaceLister struct {
	items map[string]*corev1.Namespace
}

func newFakeNamespaceLister(objects []runtime.Object) *fakeNamespaceLister {
	items := make(map[string]*corev1.Namespace)

	for _, obj := range objects {
		if ns, ok := obj.(*corev1.Namespace); ok {
			nsCopy := ns.DeepCopy()
			items[nsCopy.Name] = nsCopy
		}
	}

	return &fakeNamespaceLister{items: items}
}

func (l *fakeNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	result := make([]*corev1.Namespace, 0, len(l.items))
	for _, ns := range l.items {
		if selector.Matches(labels.Set(ns.Labels)) {
			result = append(result, ns)
		}
	}

	return result, nil
}

func (l *fakeNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if ns, ok := l.items[name]; ok {
		return ns, nil
	}

	return nil, fmt.Errorf("namespaces %q not found", name)
}

// ruleBinding is a ClusterRoleBinding as user-authz-controller creates it for a rule.
func ruleBinding(name, username string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
			binding.LabelHeritage: binding.HeritageValue, binding.LabelModule: binding.ModuleName, binding.LabelManagedBy: binding.ManagedByValue,
		}},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: username}},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "user-authz:admin"},
	}
}

// A subject bound by a rule binding whose rule is not in the directory is restricted until the rule
// arrives: the binding is created seconds after the rule, and this webhook may see it first.
func TestAuthorizeRequest_UnknownRuleBindingRestricts(t *testing.T) {
	idx := binding.NewIndex()
	idx.Upsert(ruleBinding("user-authz:late:admin", "carol"))

	handler := &Handler{
		logger:   log.New(io.Discard, "", 0),
		cache:    fixtureCache(),
		rules:    fixtureRules(), // knows nothing about the rule "late"
		bindings: idx,
		nsLister: newFakeNamespaceLister(nil),
		nsSynced: func() bool { return true },
	}

	namespaced := &WebhookRequest{Spec: WebhookResourceSpec{User: "carol", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "team-a"}}}
	if got := handler.authorizeRequest(namespaced); !got.Status.Denied || got.Status.Reason != rules.NoNamespaceAccessReason {
		t.Errorf("namespaced request of a subject with an unobserved rule must be denied, got %+v", got.Status)
	}
	clusterScoped := &WebhookRequest{Spec: WebhookResourceSpec{User: "carol", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1"}}}
	if got := handler.authorizeRequest(clusterScoped); !got.Status.Denied || got.Status.Reason != rules.NamespaceLimitedAccessReason {
		t.Errorf("cluster-scoped request of a subject with an unobserved rule must be denied, got %+v", got.Status)
	}

	// once the rule is in the directory, its own scope applies
	handler.rules = rulesFor(rules.Rule{Name: "late", Subjects: []rules.Subject{{Kind: "User", Name: "carol"}}, LimitNamespaces: []string{"team-a"}})
	if got := handler.authorizeRequest(&WebhookRequest{Spec: namespaced.Spec}); got.Status.Denied {
		t.Errorf("the rule opens team-a, got %+v", got.Status)
	}
	other := &WebhookRequest{Spec: WebhookResourceSpec{User: "carol", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "team-b"}}}
	if got := handler.authorizeRequest(other); !got.Status.Denied {
		t.Errorf("the rule does not open team-b, got %+v", got.Status)
	}

	// a directory that has not been listed yet restricts too
	handler.rules = staticRules{}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: namespaced.Spec}); !got.Status.Denied {
		t.Errorf("an unsynced directory must restrict a subject bound by a rule, got %+v", got.Status)
	}
	// but a subject nobody binds gets no opinion, as before
	nobody := &WebhookRequest{Spec: WebhookResourceSpec{User: "dave", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "team-a"}}}
	if got := handler.authorizeRequest(nobody); got.Status.Denied {
		t.Errorf("a subject without rule bindings is not restricted, got %+v", got.Status)
	}
}

// The controller updates the bindings of a rule when a subject is added to it, and the binding can
// reach the webhook before the rule's own update. The rule keeps its name throughout, so the guard
// has to notice that the observed copy of the rule does not name the new subject yet - otherwise
// the cluster-wide binding grants that subject every namespace.
func TestAuthorizeRequest_SubjectAddedToKnownRuleRestricts(t *testing.T) {
	// The directory's copy of "team-a" still names only alice.
	observed := rulesFor(rules.Rule{
		Name:            "team-a",
		Subjects:        []rules.Subject{{Kind: "User", Name: "alice"}},
		LimitNamespaces: []string{"dev"},
	})

	// The controller has already added bob to the bindings of the same rule.
	idx := binding.NewIndex()
	idx.Upsert(ruleBinding("user-authz:team-a:admin", "bob"))

	handler := &Handler{
		logger:   log.New(io.Discard, "", 0),
		cache:    fixtureCache(),
		rules:    observed,
		bindings: idx,
		nsLister: newFakeNamespaceLister(nil),
		nsSynced: func() bool { return true },
	}

	inDev := WebhookResourceSpec{User: "bob", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "dev"}}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: inDev}); !got.Status.Denied {
		t.Errorf("bob is bound by team-a but the observed rule does not name him; must be denied, got %+v", got.Status)
	}
	elsewhere := WebhookResourceSpec{User: "bob", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "kube-system"}}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: elsewhere}); !got.Status.Denied {
		t.Errorf("the same holds for a system namespace, got %+v", got.Status)
	}

	// alice, whom the observed rule does name, keeps exactly the rule's scope.
	aliceDev := WebhookResourceSpec{User: "alice", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "dev"}}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: aliceDev}); got.Status.Denied {
		t.Errorf("alice is named by the rule and dev is in its scope, got %+v", got.Status)
	}

	// Once the rule's own update lands, bob gets the rule's scope and nothing more.
	handler.rules = rulesFor(rules.Rule{
		Name:            "team-a",
		Subjects:        []rules.Subject{{Kind: "User", Name: "alice"}, {Kind: "User", Name: "bob"}},
		LimitNamespaces: []string{"dev"},
	})
	if got := handler.authorizeRequest(&WebhookRequest{Spec: inDev}); got.Status.Denied {
		t.Errorf("the updated rule opens dev for bob, got %+v", got.Status)
	}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: elsewhere}); !got.Status.Denied {
		t.Errorf("the updated rule does not open kube-system, got %+v", got.Status)
	}
}

// A subject that already has an observed rule keeps its scope when the guard fires for another
// rule: rules union, so an unobserved one may only widen, and clamping the observed scope would
// deny access the observed rule legitimately grants.
func TestAuthorizeRequest_GuardDoesNotNarrowObservedScope(t *testing.T) {
	observed := rulesFor(rules.Rule{
		Name:            "known",
		Subjects:        []rules.Subject{{Kind: "User", Name: "carol"}},
		LimitNamespaces: []string{"team-a"},
	})

	idx := binding.NewIndex()
	idx.Upsert(ruleBinding("user-authz:unobserved:admin", "carol"))

	handler := &Handler{
		logger:   log.New(io.Discard, "", 0),
		cache:    fixtureCache(),
		rules:    observed,
		bindings: idx,
		nsLister: newFakeNamespaceLister(nil),
		nsSynced: func() bool { return true },
	}

	inScope := WebhookResourceSpec{User: "carol", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "team-a"}}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: inScope}); got.Status.Denied {
		t.Errorf("the observed rule opens team-a and the guard must not take that away, got %+v", got.Status)
	}
	outOfScope := WebhookResourceSpec{User: "carol", ResourceAttributes: WebhookResourceAttributes{Group: "test", Version: "v1", Resource: "object1", Namespace: "team-b"}}
	if got := handler.authorizeRequest(&WebhookRequest{Spec: outOfScope}); !got.Status.Denied {
		t.Errorf("neither rule opens team-b, got %+v", got.Status)
	}
}
