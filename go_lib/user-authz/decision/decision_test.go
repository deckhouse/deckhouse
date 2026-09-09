/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package decision

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

type staticBindings map[string][]string

func (b staticBindings) RulesFor(username string, groups []string) []string {
	out := append([]string(nil), b[username]...)
	for _, g := range groups {
		out = append(out, b["group:"+g]...)
	}
	return out
}

func dir(rs ...rules.Rule) *rules.Directory {
	d, _ := rules.NewBuilder().Build(rs)
	return d
}

func limited(name, user string, namespaces ...string) rules.Rule {
	return rules.Rule{
		Name:            name,
		Subjects:        []rules.Subject{{Kind: "User", Name: user}},
		LimitNamespaces: namespaces,
	}
}

func scopeOf(scope rules.ResourceScope, err error) ResourceScopeFunc {
	return func(string, string) (rules.ResourceScope, error) { return scope, err }
}

func TestAuthorize_Namespaced(t *testing.T) {
	t.Parallel()
	src := Sources{
		Directory: dir(limited("team-a", "alice", "team-a")),
		Bindings:  staticBindings{},
	}

	if got := Authorize(Request{User: "alice", Namespace: "team-a"}, src); got.Denied() {
		t.Errorf("the rule opens team-a, got %+v", got)
	}
	got := Authorize(Request{User: "alice", Namespace: "team-b"}, src)
	if !got.Denied() || got.Reason != rules.NoNamespaceAccessReason {
		t.Errorf("the rule does not open team-b, got %+v", got)
	}
	// A subject no rule names is not this layer's business.
	if got := Authorize(Request{User: "bob", Namespace: "team-b"}, src); got.Denied() {
		t.Errorf("a subject without a rule gets no opinion, got %+v", got)
	}
}

// A namespaceSelector that cannot be evaluated must deny, not shrug.
//
// This is the "an error must never become no opinion by accident" case, and it is worth a test of
// its own because both wrong answers look the same from outside: a rule whose selector does not
// match denies too. The way to tell them apart is a selector that WOULD match an empty label set -
// DoesNotExist matches a namespace with no labels at all - so if the lookup error is swallowed and
// the empty set is used, the subject is granted a namespace nobody ever evaluated the selector
// against.
func TestAuthorize_NamespaceLabelLookupFailureDenies(t *testing.T) {
	t.Parallel()
	selector := &rules.NamespaceSelector{LabelSelector: &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "quarantine", Operator: metav1.LabelSelectorOpDoesNotExist},
		},
	}}
	src := Sources{
		Directory: dir(rules.Rule{
			Name:              "by-selector",
			Subjects:          []rules.Subject{{Kind: "User", Name: "alice"}},
			NamespaceSelector: selector,
		}),
		Bindings: staticBindings{},
	}
	req := Request{User: "alice", Namespace: "team-a"}

	// The control: the selector does match a namespace with no labels, so a successful lookup
	// opens it. Without this the test below could pass for the wrong reason.
	src.NamespaceLabels = func(string) (labels.Set, error) { return labels.Set{}, nil }
	if got := Authorize(req, src); got.Denied() {
		t.Fatalf("the selector matches an empty label set, so a successful lookup opens the namespace; got %+v", got)
	}

	// And when the labels cannot be read at all, the same request must be denied: an unevaluated
	// selector is not a matching selector.
	src.NamespaceLabels = func(string) (labels.Set, error) { return nil, errors.New("informer cache is not ready") }
	got := Authorize(req, src)
	if !got.Denied() || got.Reason != rules.NoNamespaceAccessReason {
		t.Errorf("a namespace whose labels could not be read was not denied: %+v", got)
	}

	// A grant that exists independently of any rule still survives, as everywhere else.
	src.IndependentRBAC = func() bool { return true }
	if got := Authorize(req, src); got.Denied() {
		t.Errorf("an independent grant must survive a failed lookup too, got %+v", got)
	}
}

func TestAuthorize_IndependentRBACOverridesTheDenial(t *testing.T) {
	t.Parallel()
	src := Sources{
		Directory:       dir(limited("team-a", "alice", "team-a")),
		Bindings:        staticBindings{},
		IndependentRBAC: func() bool { return true },
	}
	// A RoleBinding in the namespace, or a ClusterRoleBinding no rule owns, is a grant that exists
	// on its own; the multi-tenancy layer does not take it away.
	if got := Authorize(Request{User: "alice", Namespace: "team-b"}, src); got.Denied() {
		t.Errorf("a grant independent of any rule must survive, got %+v", got)
	}
}

func TestAuthorize_ClusterScoped(t *testing.T) {
	t.Parallel()
	base := Sources{
		Directory: dir(limited("team-a", "alice", "team-a")),
		Bindings:  staticBindings{},
	}
	req := Request{User: "alice", Resource: "pods", APIGroup: ""}

	cases := []struct {
		name       string
		scope      ResourceScopeFunc
		wantDenied bool
		wantReason string
	}{
		{"a cluster-scoped resource is not limited by namespaces",
			scopeOf(rules.ResourceScope{Known: true, Namespaced: false}, nil), false, ""},
		{"a namespaced resource cannot be listed cluster-wide",
			scopeOf(rules.ResourceScope{Known: true, Namespaced: true}, nil), true, rules.NamespaceLimitedAccessReason},
		{"a resource discovery says does not exist is left to RBAC",
			scopeOf(rules.ResourceScope{Absent: true}, nil), false, ""},
		{"a resource we could not look up stays closed",
			scopeOf(rules.ResourceScope{}, nil), true, rules.NamespaceLimitedAccessReason},
		{"a lookup that failed is an internal error, not a namespace limit",
			scopeOf(rules.ResourceScope{}, errors.New("i/o timeout")), true, InternalErrorReason},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := base
			src.ResourceScope = tc.scope
			got := Authorize(req, src)
			if got.Denied() != tc.wantDenied || got.Reason != tc.wantReason {
				t.Errorf("got %+v, want denied=%v reason=%q", got, tc.wantDenied, tc.wantReason)
			}
		})
	}

	// Without any way to ask, a cluster-scoped request cannot be allowed to proceed.
	src := base
	src.ResourceScope = nil
	if got := Authorize(req, src); !got.Denied() || got.Reason != InternalErrorReason {
		t.Errorf("no scope source must fail closed, got %+v", got)
	}
}

func TestAuthorize_OrderingGuard(t *testing.T) {
	t.Parallel()
	// The binding says alice is bound by a rule the directory has not observed naming her.
	src := Sources{
		Directory: dir(limited("team-a", "someone-else", "team-a")),
		Bindings:  staticBindings{"alice": {"team-a"}},
	}

	got := Authorize(Request{User: "alice", Namespace: "team-a"}, src)
	if !got.Denied() {
		t.Errorf("a binding ahead of its rule must not grant, got %+v", got)
	}

	// Once the observed rule names her, its own scope applies.
	src.Directory = dir(rules.Rule{
		Name: "team-a",
		Subjects: []rules.Subject{
			{Kind: "User", Name: "someone-else"},
			{Kind: "User", Name: "alice"},
		},
		LimitNamespaces: []string{"team-a"},
	})
	if got := Authorize(Request{User: "alice", Namespace: "team-a"}, src); got.Denied() {
		t.Errorf("the updated rule opens team-a, got %+v", got)
	}
	if got := Authorize(Request{User: "alice", Namespace: "team-b"}, src); !got.Denied() {
		t.Errorf("and does not open team-b, got %+v", got)
	}

	// A directory that has not been read yet knows nothing, so a bound subject is restricted.
	src.Directory = nil
	if got := Authorize(Request{User: "alice", Namespace: "team-a"}, src); !got.Denied() {
		t.Errorf("an unread directory must restrict a bound subject, got %+v", got)
	}
	// ...while a subject nobody binds is still none of this layer's business.
	if got := Authorize(Request{User: "nobody", Namespace: "team-a"}, src); got.Denied() {
		t.Errorf("an unbound subject gets no opinion, got %+v", got)
	}
}

func TestAuthorize_NonResourceRequest(t *testing.T) {
	t.Parallel()
	src := Sources{
		Directory: dir(limited("team-a", "alice", "team-a")),
		Bindings:  staticBindings{},
	}
	// No namespace and no resource: the rules are about namespaces, so there is nothing to limit.
	if got := Authorize(Request{User: "alice"}, src); got.Denied() {
		t.Errorf("a non-resource request is not limited, got %+v", got)
	}
}

func TestNamespaceAccess(t *testing.T) {
	t.Parallel()
	src := Sources{
		Directory: dir(
			limited("team-a", "alice", "team-a"),
			rules.Rule{Name: "open", Subjects: []rules.Subject{{Kind: "User", Name: "carol"}}, AllowAccessToSystemNamespaces: true},
		),
		Bindings:        staticBindings{},
		NamespaceLabels: func(string) (labels.Set, error) { return labels.Set{}, nil },
	}

	if access, _ := NamespaceAccess(src, "carol", nil, false); access != AllNamespaces {
		t.Errorf("a rule without filters sees everything, got %v", access)
	}
	if access, _ := NamespaceAccess(src, "nobody", nil, false); access != NoNamespaces {
		t.Errorf("a subject no rule names and no privilege sees nothing, got %v", access)
	}
	if access, _ := NamespaceAccess(src, "nobody", nil, true); access != AllNamespaces {
		t.Errorf("the caller's privilege policy is honoured, got %v", access)
	}

	access, filter := NamespaceAccess(src, "alice", nil, false)
	if access != Filtered {
		t.Fatalf("a limited subject is filtered, got %v", access)
	}
	if !NamespaceAllowed(src, &filter, "team-a") {
		t.Error("the filter opens team-a")
	}
	if NamespaceAllowed(src, &filter, "team-b") {
		t.Error("the filter does not open team-b")
	}
	// A nil filter is the "no filtering" case and must not hide anything.
	if !NamespaceAllowed(src, nil, "anything") {
		t.Error("a nil filter allows")
	}
}
