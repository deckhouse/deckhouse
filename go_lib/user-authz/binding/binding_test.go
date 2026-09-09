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

package binding

import (
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

var moduleLabels = map[string]string{LabelHeritage: HeritageValue, LabelModule: ModuleName, LabelManagedBy: ManagedByValue}

func crb(name string, labels map[string]string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Subjects:   subjects,
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "user-authz:admin"},
	}
}

func TestRuleNameOf(t *testing.T) {
	cases := map[string]struct {
		rule string
		ok   bool
	}{
		"user-authz:team-a:admin":                        {"team-a", true},
		"user-authz:team-a:admin:custom":                 {"team-a", true},
		"user-authz:team-a:additional-role:cluster-view": {"team-a", true},
		"user-authz:team-a":                              {"", false},
		"user-authz::admin":                              {"", false},
		"d8:user-authz:controller":                       {"", false},
		"team-a:admin":                                   {"", false},
	}
	for name, want := range cases {
		rule, ok := RuleNameOf(name)
		if rule != want.rule || ok != want.ok {
			t.Errorf("RuleNameOf(%q) = %q,%v; want %q,%v", name, rule, ok, want.rule, want.ok)
		}
	}
	if RulePrefix("team-a") != "user-authz:team-a:" {
		t.Errorf("RulePrefix is wrong")
	}
}

func TestIsRuleBinding(t *testing.T) {
	if !IsRuleBinding("user-authz:team-a:admin", moduleLabels) {
		t.Errorf("a module binding of a rule must be recognised")
	}
	// the chart of earlier releases labelled without managed-by
	if !IsRuleBinding("user-authz:team-a:admin", map[string]string{LabelHeritage: HeritageValue, LabelModule: ModuleName}) {
		t.Errorf("a chart-rendered binding of a rule must be recognised")
	}
	if IsRuleBinding("user-authz:team-a:admin", nil) {
		t.Errorf("a user binding that only borrows the name is not a rule binding")
	}
	if IsRuleBinding("d8:user-authz:controller", moduleLabels) {
		t.Errorf("a module binding outside the rule names is not a rule binding")
	}
}

func TestIndex(t *testing.T) {
	idx := NewIndex()
	alice := rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}
	devs := rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "devs"}
	bot := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "bot", Namespace: "ci"}

	idx.Upsert(crb("user-authz:team-a:admin", moduleLabels, alice, devs))
	idx.Upsert(crb("user-authz:team-a:admin:custom", moduleLabels, alice, devs))
	idx.Upsert(crb("user-authz:ci:user", moduleLabels, bot))
	idx.Upsert(crb("my-own-binding", nil, alice))                    // not a rule binding
	idx.Upsert(crb("user-authz:forged:admin", nil, alice))           // borrowed name, no module labels
	idx.Upsert(crb("d8:user-authz:controller", moduleLabels, alice)) // module binding, not a rule

	if idx.Len() != 3 {
		t.Fatalf("indexed %d bindings, want 3", idx.Len())
	}
	if got := idx.RulesFor("alice", nil); !slices.Equal(got, []string{"team-a"}) {
		t.Errorf("alice -> %v", got)
	}
	if got := idx.RulesFor("nobody", []string{"devs"}); !slices.Equal(got, []string{"team-a"}) {
		t.Errorf("devs -> %v", got)
	}
	if got := idx.RulesFor("system:serviceaccount:ci:bot", nil); !slices.Equal(got, []string{"ci"}) {
		t.Errorf("bot -> %v", got)
	}
	if got := idx.RulesFor("bot", nil); len(got) != 0 {
		t.Errorf("a service account is looked up by its username only, got %v", got)
	}

	// one of the two bindings of team-a goes: the rule is still bound to alice
	idx.Delete("user-authz:team-a:admin:custom")
	if got := idx.RulesFor("alice", nil); !slices.Equal(got, []string{"team-a"}) {
		t.Errorf("alice after one deletion -> %v", got)
	}
	// the subjects of the binding change: alice is no longer bound
	idx.Upsert(crb("user-authz:team-a:admin", moduleLabels, devs))
	if got := idx.RulesFor("alice", nil); len(got) != 0 {
		t.Errorf("alice after the update -> %v", got)
	}
	if got := idx.RulesFor("x", []string{"devs"}); !slices.Equal(got, []string{"team-a"}) {
		t.Errorf("devs after the update -> %v", got)
	}
	idx.Delete("user-authz:team-a:admin")
	idx.Delete("never-seen")
	if got := idx.RulesFor("x", []string{"devs"}); len(got) != 0 || idx.Len() != 1 {
		t.Errorf("devs after the deletion -> %v, len=%d", got, idx.Len())
	}
}

func TestIndex_EventHandler(t *testing.T) {
	idx := NewIndex()
	h := idx.EventHandler()
	alice := rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}

	h.OnAdd(crb("user-authz:team-a:admin", moduleLabels, alice), false)
	if got := idx.RulesFor("alice", nil); !slices.Equal(got, []string{"team-a"}) {
		t.Fatalf("after add -> %v", got)
	}
	h.OnUpdate(nil, crb("user-authz:team-a:admin", moduleLabels))
	if got := idx.RulesFor("alice", nil); len(got) != 0 {
		t.Fatalf("after update -> %v", got)
	}
	h.OnAdd(crb("user-authz:team-b:admin", moduleLabels, alice), false)
	h.OnDelete(cache.DeletedFinalStateUnknown{Key: "user-authz:team-b:admin"})
	if got := idx.RulesFor("alice", nil); len(got) != 0 {
		t.Fatalf("after tombstone -> %v", got)
	}
}
