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

package managebindings

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testUser = []rbacv1.Subject{{Kind: "User", APIGroup: rbacv1.GroupName, Name: "test"}}

// The fixtures mirror the granular role catalog (and the tests of the hook this reconciler
// replaces): system and subsystem roles are kind=role with a scope label, the leaves carrying the
// namespaces are the system capabilities of the modules (kind=capability, scope=system).

// systemRole builds a manager role of the system scope (lineage "system") or of a subsystem
// (lineage = the subsystem name) granting the admin namespace role.
func systemRole(name, scope, lineage string) *rbacv1.ClusterRole {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
			LabelUseRole:             "admin",
			"rbac.deckhouse.io/kind": "role",
			LabelScope:               scope,
		}},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
			MatchLabels: map[string]string{
				fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): "manager",
			},
		}}},
	}
	if scope == ScopeSubsystem {
		role.Labels["rbac.deckhouse.io/subsystem"] = lineage
		role.Labels["rbac.deckhouse.io/aggregate-to-system-as"] = "manager"
	}
	return role
}

// roleWith builds a role of the given scope and use-role that selects the labels in selects and
// carries the labels in carries, so higher tiers can aggregate it.
func roleWith(name, scope, useRole string, selects, carries map[string]string) *rbacv1.ClusterRole {
	roleLabels := map[string]string{
		LabelUseRole:             useRole,
		"rbac.deckhouse.io/kind": "role",
		LabelScope:               scope,
	}
	for k, v := range carries {
		roleLabels[k] = v
	}
	selectors := make([]metav1.LabelSelector, 0, len(selects))
	for k, v := range selects {
		selectors = append(selectors, metav1.LabelSelector{MatchLabels: map[string]string{k: v}})
	}
	return &rbacv1.ClusterRole{
		ObjectMeta:      metav1.ObjectMeta{Name: name, Labels: roleLabels},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: selectors},
	}
}

// systemCapability builds the leaf of the graph: a module capability of the system scope that
// carries the module namespace and aggregates into the manager of its subsystem.
func systemCapability(name, subsystem, namespace string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
		"rbac.deckhouse.io/kind": "capability",
		LabelScope:               ScopeSystem,
		LabelNamespace:           namespace,
		fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", subsystem): "manager",
	}}}
}

func manageBinding(name, role string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects:   testUser,
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role},
	}
}

func automatedUseBinding(name, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"}},
		Subjects:   testUser,
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:namespace:admin"},
	}
}

// newClient prepares a fake client the way the manager does: with the manage binding index.
func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&rbacv1.ClusterRoleBinding{}, RoleRefIndexField, RoleRefIndexValue).
		WithIndex(&rbacv1.RoleBinding{}, AutomatedIndexField, AutomatedIndexValue).
		WithObjects(objs...).
		Build()
}

func reconcileOnce(t *testing.T, c client.Client) {
	t.Helper()
	r := New(c, logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func reconcileWith(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	c := newClient(t, objs...)
	reconcileOnce(t, c)
	return c
}

func mustExist(t *testing.T, c client.Client, ns, name string) *rbacv1.RoleBinding {
	t.Helper()
	rb := &rbacv1.RoleBinding{}
	if err := c.Get(t.Context(), client.ObjectKey{Namespace: ns, Name: name}, rb); err != nil {
		t.Fatalf("rolebinding %s/%s: %v", ns, name, err)
	}
	return rb
}

func mustNotExist(t *testing.T, c client.Client, ns, name string) {
	t.Helper()
	err := c.Get(t.Context(), client.ObjectKey{Namespace: ns, Name: name}, &rbacv1.RoleBinding{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("rolebinding %s/%s must not exist, err = %v", ns, name, err)
	}
}

func TestReconcile_SubsystemBindingFansOutToModuleNamespaces(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemCapability("d8:system-capability:test2:edit", "others", "test2-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		systemCapability("d8:system-capability:test3:edit", "test", "test2-ns"),
		systemRole("d8:subsystem:test:manager", ScopeSubsystem, "test"),
		manageBinding("test2", "d8:subsystem:test:manager"),
	)

	rb := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	if rb.RoleRef.Name != "d8:namespace:admin" || rb.Annotations[relatedWithAnnotation] != "test" || rb.Labels[labelAutomated] != "true" {
		t.Errorf("use binding = %+v", rb)
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v", rb.Subjects)
	}
	mustExist(t, c, "test2-ns", "d8:namespace:admin:binding:test")
	mustExist(t, c, "test2-ns", "d8:namespace:admin:binding:test2")
	mustNotExist(t, c, "test-ns", "d8:namespace:admin:binding:test2")
}

func TestReconcile_SystemBindingReachesEveryModuleNamespace(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemCapability("d8:system-capability:test2:edit", "others", "test2-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		systemRole("d8:system:manager", ScopeSystem, "system"),
		manageBinding("test", "d8:system:manager"),
	)

	mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	mustExist(t, c, "test2-ns", "d8:namespace:admin:binding:test")
}

// d8:system:superadmin -> d8:subsystem:security:superadmin -> d8:subsystem:security:manager ->
// capability: the namespace sits three aggregation levels below the bound role, and the bound role
// grants the superadmin namespace role.
func TestReconcile_SuperadminBindingFollowsThreeAggregationLevels(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "security", "sec-ns"),
		roleWith("d8:subsystem:security:manager", ScopeSubsystem, "admin",
			map[string]string{"rbac.deckhouse.io/aggregate-to-security-as": "manager"},
			map[string]string{
				"rbac.deckhouse.io/subsystem":                "security",
				"rbac.deckhouse.io/aggregate-to-security-as": "superadmin",
				"rbac.deckhouse.io/aggregate-to-system-as":   "manager",
			}),
		roleWith("d8:subsystem:security:superadmin", ScopeSubsystem, "superadmin",
			map[string]string{"rbac.deckhouse.io/aggregate-to-security-as": "superadmin"},
			map[string]string{
				"rbac.deckhouse.io/subsystem":              "security",
				"rbac.deckhouse.io/aggregate-to-system-as": "superadmin",
			}),
		roleWith("d8:system:superadmin", ScopeSystem, "superadmin",
			map[string]string{"rbac.deckhouse.io/aggregate-to-system-as": "superadmin"},
			nil),
		manageBinding("test", "d8:system:superadmin"),
	)

	rb := mustExist(t, c, "sec-ns", "d8:namespace:superadmin:binding:test")
	if rb.RoleRef.Name != "d8:namespace:superadmin" {
		t.Errorf("roleRef = %v, want the superadmin namespace role", rb.RoleRef)
	}
}

func TestReconcile_NamespaceDroppingOutRemovesOnlyItsBinding(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		automatedUseBinding("d8:namespace:admin:binding:test", "test2-ns"),
	)

	mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	mustNotExist(t, c, "test2-ns", "d8:namespace:admin:binding:test")
}

func TestReconcile_OrphanedAutomatedBindingsAreDeleted(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		automatedUseBinding("d8:binding:test", "test-ns"),
		automatedUseBinding("d8:binding:test2", "test-ns"),
		automatedUseBinding("d8:binding:test3", "test-ns2"),
		// a rule RoleBinding of the basic model carries the module labels but not the automated one
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:rule:editor", Namespace: "test-ns", Labels: map[string]string{labelHeritage: "deckhouse", "module": "user-authz"}},
			Subjects:   testUser,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "user-authz:editor"},
		},
	)

	mustNotExist(t, c, "test-ns", "d8:binding:test")
	mustNotExist(t, c, "test-ns", "d8:binding:test2")
	mustNotExist(t, c, "test-ns2", "d8:binding:test3")
	mustExist(t, c, "test-ns", "user-authz:rule:editor")
}

// The projections of the previous role names (d8:use:<role>:binding:<binding> to d8:use:role:<role>)
// are automated bindings that are no longer expected: they go, and the new ones take their place.
func TestReconcile_ReplacesProjectionsOfTheFormerRoleNames(t *testing.T) {
	t.Parallel()
	legacy := automatedUseBinding("d8:use:admin:binding:test", "test-ns")
	legacy.RoleRef.Name = "d8:use:role:admin"
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		legacy,
	)

	mustNotExist(t, c, "test-ns", "d8:use:admin:binding:test")
	rb := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	if rb.RoleRef.Name != "d8:namespace:admin" {
		t.Errorf("roleRef = %v", rb.RoleRef)
	}
}

func TestReconcile_RepairsDriftedUseBinding(t *testing.T) {
	t.Parallel()
	drifted := automatedUseBinding("d8:namespace:admin:binding:test", "test-ns")
	drifted.Subjects = []rbacv1.Subject{{Kind: "User", APIGroup: rbacv1.GroupName, Name: "someone-else"}}
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		drifted,
	)

	rb := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v, want the manage binding's subjects", rb.Subjects)
	}
	if rb.Annotations[relatedWithAnnotation] != "test" {
		t.Errorf("annotations = %v", rb.Annotations)
	}
}

func TestReconcile_RoleWithoutUseRoleIsIgnored(t *testing.T) {
	t.Parallel()
	role := systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others")
	delete(role.Labels, LabelUseRole)
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		role,
		manageBinding("test", "d8:subsystem:others:manager"),
	)

	list := &rbacv1.RoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("no use binding expected, got %v", list.Items)
	}
}

// A ClusterRoleBinding to a capability produces nothing: capabilities carry no use-role.
func TestReconcile_CapabilityBindingIsIgnored(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		manageBinding("test", "d8:system-capability:test:edit"),
	)

	list := &rbacv1.RoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("no use binding expected, got %v", list.Items)
	}
}

// Aggregation selectors with matchExpressions are honoured like the API server does.
func TestReconcile_MatchExpressionsSelectorIsHonoured(t *testing.T) {
	t.Parallel()
	role := systemRole("d8:system:exprmanager", ScopeSystem, "system")
	role.AggregationRule = &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "rbac.deckhouse.io/aggregate-to-others-as", Operator: metav1.LabelSelectorOpIn, Values: []string{"manager"}},
		},
	}}}
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemCapability("d8:system-capability:foreign:edit", "foreign", "foreign-ns"),
		role,
		manageBinding("exprbind", "d8:system:exprmanager"),
	)

	mustExist(t, c, "test-ns", "d8:namespace:admin:binding:exprbind")
	mustNotExist(t, c, "foreign-ns", "d8:namespace:admin:binding:exprbind")
}

// Aggregation is followed to any depth: system -> middle -> subsystem -> capability, with a cycle
// thrown in.
func TestReconcile_DeepAggregationIsFollowed(t *testing.T) {
	t.Parallel()
	system := systemRole("d8:system:manager", ScopeSystem, "system")
	subsystem := systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others")
	// a role in the middle that aggregates the subsystem managers and is itself aggregated by system
	middle := roleWith("d8:system:middle", ScopeSystem, "admin",
		map[string]string{LabelScope: ScopeSubsystem},
		map[string]string{"rbac.deckhouse.io/aggregate-to-system-as": "manager"})
	// the subsystem role points back at the middle role: a cycle that must terminate
	subsystem.AggregationRule.ClusterRoleSelectors = append(subsystem.AggregationRule.ClusterRoleSelectors,
		metav1.LabelSelector{MatchLabels: map[string]string{"rbac.deckhouse.io/aggregate-to-system-as": "manager"}})
	delete(subsystem.Labels, "rbac.deckhouse.io/aggregate-to-system-as")

	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		system, middle, subsystem,
		manageBinding("root", "d8:system:manager"),
	)

	mustExist(t, c, "test-ns", "d8:namespace:admin:binding:root")
}

func TestReconcile_PreservesForeignMetadataWithoutChurn(t *testing.T) {
	t.Parallel()
	existing := automatedUseBinding("d8:namespace:admin:binding:test", "test-ns")
	existing.Annotations = map[string]string{"example.com/note": "keep me", relatedWithAnnotation: "test"}
	existing.Labels["example.com/team"] = "blue"
	c := newClient(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		existing,
	)
	before := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")

	reconcileOnce(t, c)

	after := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("a binding that is already correct must not be rewritten")
	}
	if after.Annotations["example.com/note"] != "keep me" || after.Labels["example.com/team"] != "blue" {
		t.Errorf("foreign metadata lost: %+v", after.ObjectMeta)
	}
}

func TestReconcile_RecreatesBindingWithStaleRoleRef(t *testing.T) {
	t.Parallel()
	stale := automatedUseBinding("d8:namespace:admin:binding:test", "test-ns")
	stale.RoleRef.Name = "d8:namespace:user"
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		stale,
	)

	rb := mustExist(t, c, "test-ns", "d8:namespace:admin:binding:test")
	if rb.RoleRef.Name != "d8:namespace:admin" {
		t.Errorf("roleRef = %v, want the use role of the manage role", rb.RoleRef)
	}
}

func TestRoleRefIndexValue(t *testing.T) {
	t.Parallel()
	if got := RoleRefIndexValue(manageBinding("x", "d8:subsystem:others:manager")); len(got) != 1 || got[0] != "d8:subsystem:others:manager" {
		t.Errorf("binding must be indexed by its roleRef, got %v", got)
	}
	roleBinding := manageBinding("x", "d8:subsystem:others:manager")
	roleBinding.RoleRef.Kind = "Role"
	if got := RoleRefIndexValue(roleBinding); got != nil {
		t.Errorf("a binding to a Role must not be indexed, got %v", got)
	}
}

// Manage roles are recognised by their scope label, not by their name: user-created roles of the
// legacy scheme are named custom:*, and the hook this reconciler replaces projected them too.
func TestReconcile_CustomNamedManageRoleIsProjected(t *testing.T) {
	t.Parallel()
	role := systemRole("custom:manage:others:manager", ScopeSubsystem, "others")
	c := reconcileWith(t,
		systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
		role,
		manageBinding("custom-binding", "custom:manage:others:manager"),
		// a binding to a ClusterRole that is not a manage role must produce nothing
		manageBinding("unrelated", "cluster-admin"),
	)

	mustExist(t, c, "test-ns", "d8:namespace:admin:binding:custom-binding")
	list := &rbacv1.RoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("use bindings = %d, want only the projection of the custom-named manage role", len(list.Items))
	}
}

func TestIsManageBinding(t *testing.T) {
	t.Parallel()
	c := newClient(t,
		systemRole("custom:manage:others:manager", ScopeSubsystem, "others"),
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:namespace:admin", Labels: map[string]string{
			"rbac.deckhouse.io/kind": "role", LabelScope: "namespace",
		}}},
	)
	if !IsManageBinding(c, manageBinding("x", "custom:manage:others:manager")) {
		t.Error("a binding to a subsystem-scoped role must match whatever the role is named")
	}
	if IsManageBinding(c, manageBinding("x", "d8:namespace:admin")) {
		t.Error("a binding to a namespace-scoped role is not a manage binding")
	}
	if IsManageBinding(c, manageBinding("x", "d8:system:missing")) {
		t.Error("a binding to a ClusterRole that is not in the cache must not match")
	}
	if IsManageBinding(c, &rbacv1.RoleBinding{}) {
		t.Error("a RoleBinding is never a manage binding")
	}
}

func TestManageRoleSelector(t *testing.T) {
	t.Parallel()
	for scope, want := range map[string]bool{ScopeSystem: true, ScopeSubsystem: true, "namespace": false, "project": false, "": false} {
		if got := ManageRoleSelector.Matches(labels.Set(map[string]string{LabelScope: scope})); got != want {
			t.Errorf("scope %q: selector matches = %v, want %v", scope, got, want)
		}
		if got := isManageRole(map[string]string{LabelScope: scope}); got != want {
			t.Errorf("scope %q: isManageRole = %v, want %v", scope, got, want)
		}
	}
}

func TestAutomatedIndexValue(t *testing.T) {
	t.Parallel()
	if got := AutomatedIndexValue(automatedUseBinding("d8:namespace:admin:binding:x", "ns")); len(got) != 1 {
		t.Errorf("an automated use binding must be indexed, got %v", got)
	}
	rule := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "user-authz:rule:editor", Namespace: "ns", Labels: map[string]string{labelHeritage: "deckhouse", "module": "user-authz"}}}
	if got := AutomatedIndexValue(rule); got != nil {
		t.Errorf("a rule binding must not be indexed, got %v", got)
	}
}
