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

package desired

import (
	"errors"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"

	v1 "user-authz-controller/api/v1"
)

var subjects = []rbacv1.Subject{{Kind: "User", Name: "Efrem Testenev"}}

// nameAndRole is the compact shape the tests compare: <binding name> -> <ClusterRole>.
type nameAndRole struct{ name, role string }

func shapes(bs []Binding) []nameAndRole {
	out := make([]nameAndRole, 0, len(bs))
	for _, b := range bs {
		out = append(out, nameAndRole{b.Name, b.RoleRef})
	}
	return out
}

// The expectations below are the objects the module chart rendered for the same rules before the
// controller took over (golden values), so the take-over is name-for-name.
func TestBindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		rule Rule
		want []nameAndRole
	}{
		{
			name: "cluster rule with additional role, Admin level and scale",
			rule: Rule{Name: "testenev", AccessLevel: AccessLevelAdmin, AllowScale: true, Subjects: subjects,
				AdditionalRoles: []v1.AdditionalRole{{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cluster-write-all"}}},
			want: []nameAndRole{
				{"user-authz:testenev:additional-role:cluster-write-all", "cluster-write-all"},
				{"user-authz:testenev:admin", "user-authz:admin"},
				{"user-authz:testenev:admin:custom", "user-authz:admin:custom"},
				{"user-authz:testenev:scale", "user-authz:scale"},
			},
		},
		{
			name: "namespaced rule with Editor level and scale",
			rule: Rule{Name: "testenev-namespaced", Namespace: "testenv", AccessLevel: AccessLevelEditor, AllowScale: true, Subjects: subjects},
			want: []nameAndRole{
				{"user-authz:testenev-namespaced:editor", "user-authz:editor"},
				{"user-authz:testenev-namespaced:editor:custom", "user-authz:editor:custom"},
				{"user-authz:testenev-namespaced:scale", "user-authz:scale"},
			},
		},
		{
			name: "PrivilegedUser is kebab-cased like the chart did",
			rule: Rule{Name: "pu", AccessLevel: AccessLevelPrivilegedUser, PortForwarding: true, Subjects: subjects},
			want: []nameAndRole{
				{"user-authz:pu:privileged-user", "user-authz:privileged-user"},
				{"user-authz:pu:privileged-user:custom", "user-authz:privileged-user:custom"},
				{"user-authz:pu:port-forward", "user-authz:port-forward"},
			},
		},
		{
			name: "SuperAdmin gets no aggregated custom binding",
			rule: Rule{Name: "root", AccessLevel: AccessLevelSuperAdmin, Subjects: subjects},
			want: []nameAndRole{{"user-authz:root:super-admin", "user-authz:super-admin"}},
		},
		{
			name: "rule without accessLevel renders only the explicit grants",
			rule: Rule{Name: "roles-only", PortForwarding: true, Subjects: subjects,
				AdditionalRoles: []v1.AdditionalRole{{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "view"}}},
			want: []nameAndRole{
				{"user-authz:roles-only:additional-role:view", "view"},
				{"user-authz:roles-only:port-forward", "user-authz:port-forward"},
			},
		},
		{
			name: "allowScale=false and portForwarding=false render nothing extra",
			rule: Rule{Name: "plain", AccessLevel: AccessLevelUser, Subjects: subjects},
			want: []nameAndRole{
				{"user-authz:plain:user", "user-authz:user"},
				{"user-authz:plain:user:custom", "user-authz:user:custom"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Bindings(tc.rule)
			if err != nil {
				t.Fatalf("Bindings: %v", err)
			}
			gotShapes := shapes(got)
			if len(gotShapes) != len(tc.want) {
				t.Fatalf("got %d bindings %v, want %d %v", len(gotShapes), gotShapes, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if gotShapes[i] != tc.want[i] {
					t.Errorf("binding %d = %v, want %v", i, gotShapes[i], tc.want[i])
				}
			}
			for _, b := range got {
				if b.Namespace != tc.rule.Namespace {
					t.Errorf("%s: namespace = %q, want %q", b.Name, b.Namespace, tc.rule.Namespace)
				}
				if b.Labels[LabelHeritage] != HeritageValue || b.Labels[LabelModule] != ModuleName || b.Labels[LabelManagedBy] != ManagedByValue {
					t.Errorf("%s: labels = %v", b.Name, b.Labels)
				}
				isAggregated := b.Labels[LabelBindingKind] == BindingKindAggregate
				if isAggregated != (len(b.Name) > 7 && b.Name[len(b.Name)-7:] == ":custom") {
					t.Errorf("%s: binding-kind label = %q", b.Name, b.Labels[LabelBindingKind])
				}
				if len(b.Subjects) != len(tc.rule.Subjects) {
					t.Errorf("%s: subjects = %v", b.Name, b.Subjects)
				}
			}
		})
	}
}

func TestBindingsInvalidSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		rule Rule
	}{
		{"unknown level", Rule{Name: "x", AccessLevel: "Wrong", Subjects: subjects}},
		{"cluster-wide level on a namespaced rule", Rule{Name: "x", Namespace: "ns", AccessLevel: AccessLevelClusterAdmin, Subjects: subjects}},
		{"SuperAdmin on a namespaced rule", Rule{Name: "x", Namespace: "ns", AccessLevel: AccessLevelSuperAdmin, Subjects: subjects}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Bindings(tc.rule); !errors.Is(err, ErrInvalidSpec) {
				t.Fatalf("err = %v, want ErrInvalidSpec", err)
			}
		})
	}
}

func TestRuleNameOf(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name string
		ok   bool
	}{
		"user-authz:testenev:admin":                                     {"testenev", true},
		"user-authz:testenev:admin:custom-cluster-role:d8:user-authz:x": {"testenev", true},
		"user-authz:testenev:additional-role:cluster-write-all":         {"testenev", true},
		"d8:user-authz:admin-kubeconfig":                                {"", false},
		"user-authz:":                                                   {"", false},
		"user-authz:noprefix":                                           {"", false},
	}

	for in, want := range cases {
		got, ok := RuleNameOf(in)
		if ok != want.ok || got != want.name {
			t.Errorf("RuleNameOf(%q) = (%q, %v), want (%q, %v)", in, got, ok, want.name, want.ok)
		}
	}
}

func TestObjectsCarryOwnerAndRoleRef(t *testing.T) {
	t.Parallel()

	rule := Rule{Name: "r", Namespace: "ns", UID: "uid-1", APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule",
		AccessLevel: AccessLevelUser, Subjects: subjects}
	bs, err := Bindings(rule)
	if err != nil {
		t.Fatal(err)
	}
	rb := RoleBinding(bs[0], OwnerReference(rule))
	if rb.Namespace != "ns" || rb.RoleRef.Kind != "ClusterRole" || rb.RoleRef.Name != "user-authz:user" {
		t.Errorf("rolebinding = %+v", rb)
	}
	if len(rb.OwnerReferences) != 1 || rb.OwnerReferences[0].UID != "uid-1" || rb.OwnerReferences[0].Kind != "AuthorizationRule" {
		t.Errorf("owner = %+v", rb.OwnerReferences)
	}

	crule := Rule{Name: "c", UID: "uid-2", APIVersion: "deckhouse.io/v1", Kind: "ClusterAuthorizationRule", AccessLevel: AccessLevelUser, Subjects: subjects}
	cbs, _ := Bindings(crule)
	crb := ClusterRoleBinding(cbs[0], OwnerReference(crule))
	if crb.Namespace != "" || crb.OwnerReferences[0].Kind != "ClusterAuthorizationRule" {
		t.Errorf("clusterrolebinding = %+v", crb)
	}
}
