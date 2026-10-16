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
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authz-controller/internal/metrics"
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

// legacyAlias builds the compatibility alias of a manage role of the previous release the way
// user-authz renders it: labelled like that role (rbac.deckhouse.io/kind: manage and its use role),
// aggregating by selects, with the selectors of its projection in the use-binding-selectors
// annotation.
func legacyAlias(t *testing.T, name, useRole string, selects map[string]string, projection ...metav1.LabelSelector) *rbacv1.ClusterRole {
	t.Helper()
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				labelHeritage:                  "deckhouse",
				"module":                       "user-authz",
				"rbac.deckhouse.io/deprecated": "true",
				LabelKind:                      KindManage,
				LabelUseRole:                   useRole,
			},
			Annotations: map[string]string{AnnotationUseBindingSelectors: string(raw)},
		},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{MatchLabels: selects}}},
	}
}

// legacyCapability builds a capability of the previous scheme, the way a module released on its own
// still ships it: kind manage, the lineage of the old subsystem, no scope.
func legacyCapability(name, lineage, level, namespace string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
		labelHeritage:             "deckhouse",
		LabelKind:                 KindManage,
		"rbac.deckhouse.io/level": "module",
		LabelNamespace:            namespace,
		fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): level,
	}}}
}

// moduleCapability builds a system capability of a module in the current scheme.
func moduleCapability(module, lineage, level, namespace string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: fmt.Sprintf("d8:system-capability:%s:%s", module, level),
		Labels: map[string]string{
			labelHeritage:  "deckhouse",
			"module":       module,
			LabelKind:      "capability",
			LabelScope:     ScopeSystem,
			LabelNamespace: namespace,
			fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): level,
		},
	}}
}

// formerModules selects the system capabilities of the given modules on lineage at level, the
// projection of an alias for the modules of this repository that were in its old subsystem.
func formerModules(lineage, level string, modules ...string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelKind:  "capability",
			LabelScope: ScopeSystem,
			fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): level,
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "module", Operator: metav1.LabelSelectorOpIn, Values: modules}},
	}
}

// oldLineage selects the capabilities of the previous scheme on the lineage of an old subsystem.
func oldLineage(lineage, level string) metav1.LabelSelector {
	return metav1.LabelSelector{MatchLabels: map[string]string{
		LabelKind: KindManage,
		fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): level,
	}}
}

// legacyUseBinding builds a projection of the previous release: d8:use:<role>:binding:<binding> to
// d8:use:role:<role>.
func legacyUseBinding(useRole, binding, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "d8:use:" + useRole + ":binding:" + binding,
			Namespace:   namespace,
			Labels:      map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"},
			Annotations: map[string]string{relatedWithAnnotation: binding},
		},
		Subjects: testUser,
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:use:role:" + useRole},
	}
}

// useBindingsOf returns the namespace/name/roleRef of every RoleBinding, sorted.
func useBindingsOf(t *testing.T, c client.Client) []string {
	t.Helper()
	list := &rbacv1.RoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].Namespace+"/"+list.Items[i].Name+" -> "+list.Items[i].RoleRef.Name)
	}
	slices.Sort(out)
	return out
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
	// The fake client holds every ClusterRole, so it serves as the cache of the previous scheme too.
	r := New(c, c, metrics.New(), logr.Discard())
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

func TestReconcile_ReportsMetrics(t *testing.T) {
	t.Parallel()
	m := metrics.New()
	c := newClient(t,
		systemCapability("d8:capability:module:test:edit", "others", "test-ns"),
		systemCapability("d8:capability:module:test2:edit", "others", "test2-ns"),
		systemRole("d8:subsystem:others:manager", ScopeSubsystem, "others"),
		manageBinding("test", "d8:subsystem:others:manager"),
		automatedUseBinding("d8:use:admin:binding:orphan", "test-ns"),
	)
	if _, err := New(c, c, m, logr.Discard()).Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	expected := `
# HELP d8_user_authz_bindings_actual Bindings of the reconciled objects of a kind after their last reconcile; equals desired once the controller converged.
# TYPE d8_user_authz_bindings_actual gauge
d8_user_authz_bindings_actual{kind="manage"} 2
# HELP d8_user_authz_bindings_desired Bindings the reconciled objects of a kind must have.
# TYPE d8_user_authz_bindings_desired gauge
d8_user_authz_bindings_desired{kind="manage"} 2
`
	if err := testutil.CollectAndCompare(m, strings.NewReader(expected), "d8_user_authz_bindings_desired", "d8_user_authz_bindings_actual"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(m.ApplyTotal().WithLabelValues(metrics.KindManage, metrics.OpCreate, metrics.ResultSuccess)); got != 2 {
		t.Errorf("creates = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.ApplyTotal().WithLabelValues(metrics.KindManage, metrics.OpDelete, metrics.ResultSuccess)); got != 1 {
		t.Errorf("deletes = %v, want 1 (the orphan)", got)
	}
}

// A ClusterRoleBinding to the alias of a manage role of the previous release keeps the RoleBindings
// that release made for it: the same names, the same use role and the namespaces of the modules of
// the old subsystem. The alias aggregates the whole network lineage, but its projection reaches only
// the modules its annotation names and the capabilities of the previous scheme on the old lineage,
// so neither a module that joined the subsystem since nor a module already relabelled to the new
// scheme gets a RoleBinding the previous release did not make.
func TestReconcile_LegacyAliasKeepsTheProjectionOfThePreviousRelease(t *testing.T) {
	t.Parallel()
	alias := legacyAlias(t, "d8:manage:networking:manager", "user",
		map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
		formerModules("network", "viewer", "cni-cilium", "kube-dns"),
		formerModules("network", "manager", "cni-cilium", "kube-dns"),
		oldLineage("networking", "viewer"),
		oldLineage("networking", "manager"),
	)
	kept := legacyUseBinding("user", "netmgr", "d8-cni-cilium")
	c := newClient(t,
		alias,
		moduleCapability("cni-cilium", "network", "viewer", "d8-cni-cilium"),
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		moduleCapability("kube-dns", "network", "manager", "kube-system"),
		// joined network from infrastructure: the previous networking role did not reach it
		moduleCapability("keepalived", "network", "manager", "d8-keepalived"),
		// a module released on its own and already on the network lineage of the current scheme
		moduleCapability("sdn", "network", "manager", "d8-sdn"),
		// a module released on its own and still on the networking lineage of the previous scheme
		legacyCapability("d8:manage:permission:module:ingress-nginx:view", "networking", "viewer", "d8-ingress-nginx"),
		manageBinding("netmgr", "d8:manage:networking:manager"),
		// a capability of the previous scheme carries no use role and projects nothing
		manageBinding("cap", "d8:manage:permission:module:ingress-nginx:view"),
		kept,
	)
	before := mustExist(t, c, "d8-cni-cilium", "d8:use:user:binding:netmgr")

	reconcileOnce(t, c)

	want := []string{
		"d8-cni-cilium/d8:use:user:binding:netmgr -> d8:use:role:user",
		"d8-ingress-nginx/d8:use:user:binding:netmgr -> d8:use:role:user",
		"kube-system/d8:use:user:binding:netmgr -> d8:use:role:user",
	}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
	after := mustExist(t, c, "d8-cni-cilium", "d8:use:user:binding:netmgr")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("the RoleBinding the previous release made must be kept as it is, not rewritten")
	}
	created := mustExist(t, c, "d8-ingress-nginx", "d8:use:user:binding:netmgr")
	if created.Annotations[relatedWithAnnotation] != "netmgr" || created.Labels[labelAutomated] != "true" || created.Labels[labelHeritage] != "deckhouse" {
		t.Errorf("projection metadata = %+v", created.ObjectMeta)
	}
	if len(created.Subjects) != 1 || created.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v", created.Subjects)
	}
}

// A role of the previous scheme that no Deckhouse module renders, such as a custom:* role of a
// user, is not projected, as in the current scheme: its leftover projections go.
func TestReconcile_LegacyRoleOfAUserIsNotProjected(t *testing.T) {
	t.Parallel()
	custom := legacyAlias(t, "custom:manage:mycustom:manager", "user",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-networking-as": "manager"},
		oldLineage("networking", "manager"),
	)
	delete(custom.Labels, labelHeritage)
	custom.Labels["module"] = ""
	c := reconcileWith(t,
		custom,
		legacyCapability("d8:manage:permission:module:ingress-nginx:edit", "networking", "manager", "d8-ingress-nginx"),
		manageBinding("custom", "custom:manage:mycustom:manager"),
		legacyUseBinding("user", "custom", "d8-ingress-nginx"),
	)

	if got := useBindingsOf(t, c); len(got) != 0 {
		t.Errorf("use bindings = %v, want none", got)
	}
}

// A manage role of the previous scheme that a module still ships has no projection annotation: its
// RoleBindings follow its aggregation rule, as the previous release had it.
func TestReconcile_LegacyModuleRoleFollowsItsAggregation(t *testing.T) {
	t.Parallel()
	role := legacyAlias(t, "d8:manage:virtualization:manager", "user",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-virtualization-as": "manager"})
	delete(role.Annotations, AnnotationUseBindingSelectors)
	delete(role.Labels, "rbac.deckhouse.io/deprecated")
	role.Labels["module"] = "virtualization"
	c := reconcileWith(t,
		role,
		legacyCapability("d8:manage:permission:module:virtualization:edit", "virtualization", "manager", "d8-virtualization"),
		manageBinding("virt", "d8:manage:virtualization:manager"),
	)

	want := []string{"d8-virtualization/d8:use:user:binding:virt -> d8:use:role:user"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// An alias whose projection annotation cannot be read projects nothing rather than falling back to
// its aggregation rule, which would reach namespaces the previous release did not.
func TestReconcile_LegacyAliasWithAnUnreadableProjectionProjectsNothing(t *testing.T) {
	t.Parallel()
	alias := legacyAlias(t, "d8:manage:observability:viewer", "viewer",
		map[string]string{"rbac.deckhouse.io/aggregate-to-observability-as": "viewer"})
	alias.Annotations[AnnotationUseBindingSelectors] = "{not json"
	c := reconcileWith(t,
		alias,
		moduleCapability("okmeter", "observability", "viewer", "d8-okmeter"),
		manageBinding("obsview", "d8:manage:observability:viewer"),
	)

	if got := useBindingsOf(t, c); len(got) != 0 {
		t.Errorf("use bindings = %v, want none", got)
	}
}

// One entry of the projection annotation that selects everything, does not parse or carries a key
// the selector does not have makes the whole annotation unreadable: the alias projects nothing
// rather than reaching every namespace a cached role names, and nothing rather than what the
// other entries select.
func TestReconcile_LegacyAliasWithAnInvalidProjectionEntryProjectsNothing(t *testing.T) {
	t.Parallel()
	okmeter, err := json.Marshal(formerModules("observability", "viewer", "okmeter"))
	if err != nil {
		t.Fatal(err)
	}
	for name, annotation := range map[string]string{
		"empty entry":              `[{}]`,
		"null entry":               `[null]`,
		"misspelled key":           `[{"matchLabel":{"module":"okmeter"}}]`,
		"empty expression list":    `[{"matchExpressions":[]}]`,
		"empty label map":          `[{"matchLabels":{}}]`,
		"misspelled expression":    `[{"matchExpressions":[{"key":"module","operater":"In","values":["okmeter"]}]}]`,
		"valid and empty entry":    `[` + string(okmeter) + `,{}]`,
		"valid and invalid entry":  `[` + string(okmeter) + `,{"matchExpressions":[{"key":"module","operator":"Near","values":["okmeter"]}]}]`,
		"trailing data":            `[` + string(okmeter) + `] []`,
		"trailing bracket":         `[` + string(okmeter) + `]]`,
		"object instead of a list": `{"matchLabels":{"module":"okmeter"}}`,
		// read leniently, the entry would keep its labels and lose the module narrowing
		"misspelled expression list next to labels": `[{"matchLabels":{"rbac.deckhouse.io/kind":"capability","rbac.deckhouse.io/scope":"system",` +
			`"rbac.deckhouse.io/aggregate-to-observability-as":"viewer"},"matchExpresions":[{"key":"module","operator":"In","values":["okmeter"]}]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			alias := legacyAlias(t, "d8:manage:observability:viewer", "viewer",
				map[string]string{"rbac.deckhouse.io/aggregate-to-observability-as": "viewer"})
			alias.Annotations[AnnotationUseBindingSelectors] = annotation
			c := reconcileWith(t,
				alias,
				moduleCapability("okmeter", "observability", "viewer", "d8-okmeter"),
				moduleCapability("loki", "observability", "viewer", "d8-loki"),
				moduleCapability("deckhouse", "cluster", "viewer", "d8-system"),
				moduleCapability("control-plane-manager", "cluster", "viewer", "kube-system"),
				moduleCapability("user-authn", "iam", "viewer", "d8-user-authn"),
				manageBinding("obsview", "d8:manage:observability:viewer"),
			)

			if got := useBindingsOf(t, c); len(got) != 0 {
				t.Errorf("use bindings = %v, want none", got)
			}
		})
	}

	// The same fixtures with the valid annotation reach the okmeter namespace only.
	c := reconcileWith(t,
		legacyAlias(t, "d8:manage:observability:viewer", "viewer",
			map[string]string{"rbac.deckhouse.io/aggregate-to-observability-as": "viewer"},
			formerModules("observability", "viewer", "okmeter")),
		moduleCapability("okmeter", "observability", "viewer", "d8-okmeter"),
		moduleCapability("loki", "observability", "viewer", "d8-loki"),
		moduleCapability("deckhouse", "cluster", "viewer", "d8-system"),
		moduleCapability("control-plane-manager", "cluster", "viewer", "kube-system"),
		manageBinding("obsview", "d8:manage:observability:viewer"),
	)
	want := []string{"d8-okmeter/d8:use:viewer:binding:obsview -> d8:use:role:viewer"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// Only a compatibility alias of Deckhouse projects the old way: a ClusterRole named d8:manage:*
// with heritage: deckhouse, the manage kind and a use role. A role of Deckhouse under another name
// that carries the same labels and the annotation projects nothing, and a role of the previous
// scheme that is not deprecated follows its aggregation rule whatever its annotation says.
func TestReconcile_LegacyProjectionIsLimitedToTheOldManageRolesOfDeckhouse(t *testing.T) {
	t.Parallel()
	outside := legacyAlias(t, "user-authz:scale", "admin",
		map[string]string{"rbac.deckhouse.io/aggregate-to-observability-as": "viewer"},
		formerModules("cluster", "viewer", "control-plane-manager"))
	moduleRole := legacyAlias(t, "d8:manage:virtualization:manager", "user",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-virtualization-as": "manager"},
		formerModules("cluster", "viewer", "control-plane-manager"))
	delete(moduleRole.Labels, "rbac.deckhouse.io/deprecated")
	moduleRole.Labels["module"] = "virtualization"
	c := reconcileWith(t,
		outside,
		moduleRole,
		moduleCapability("control-plane-manager", "cluster", "viewer", "kube-system"),
		legacyCapability("d8:manage:permission:subsystem:virtualization:manage_resources", "virtualization", "manager", "d8-virtualization"),
		manageBinding("scale", "user-authz:scale"),
		manageBinding("virt", "d8:manage:virtualization:manager"),
		legacyUseBinding("admin", "scale", "kube-system"),
	)

	want := []string{"d8-virtualization/d8:use:user:binding:virt -> d8:use:role:user"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// The projection of an old manage role follows the ClusterRoles of Deckhouse only, named d8:* and
// labelled heritage: deckhouse. A ClusterRole that a user labels the old way, a custom role the
// previous release documented or a capability on the lineage of a module with a subsystem of its
// own, adds no namespace to it, and neither does a ClusterRole of Deckhouse outside the d8: names
// that someone relabels, so its namespace label cannot reach the subjects of the binding.
func TestReconcile_LegacyProjectionFollowsTheRolesOfDeckhouseOnly(t *testing.T) {
	t.Parallel()
	all := legacyAlias(t, "d8:manage:all:manager", "user",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-all-as": "manager"},
		metav1.LabelSelector{MatchLabels: map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-all-as": "manager"}})
	virtualization := legacyAlias(t, "d8:manage:virtualization:manager", "user",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-virtualization-as": "manager"})
	delete(virtualization.Annotations, AnnotationUseBindingSelectors)
	delete(virtualization.Labels, "rbac.deckhouse.io/deprecated")
	virtualization.Labels["module"] = "virtualization"
	virtualization.Labels["rbac.deckhouse.io/aggregate-to-all-as"] = "manager"
	// A custom subsystem role of the previous release, as its FAQ described it, and what it aggregates.
	custom := legacyAlias(t, "custom:manage:mycustom:manager", "admin",
		map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-mycustom-as": "manager"})
	delete(custom.Labels, labelHeritage)
	delete(custom.Annotations, AnnotationUseBindingSelectors)
	custom.Labels["rbac.deckhouse.io/aggregate-to-all-as"] = "manager"
	customLeaf := legacyCapability("custom:manage:permission:mycustom:edit", "mycustom", "manager", "d8-system")
	delete(customLeaf.Labels, labelHeritage)
	// A ClusterRole a user adds to the lineage of a module with a subsystem of its own.
	planted := legacyCapability("planted", "virtualization", "manager", "kube-system")
	delete(planted.Labels, labelHeritage)
	// A ClusterRole of Deckhouse outside the d8: names, which an UPDATE may relabel onto that lineage.
	relabelled := legacyCapability("user-authz:scale", "virtualization", "manager", "d8-system")
	// A ClusterRole under a d8: name that users may create, without heritage: deckhouse.
	namedLikeDeckhouse := legacyCapability("d8:commander:planted", "virtualization", "manager", "d8-cni-cilium")
	delete(namedLikeDeckhouse.Labels, labelHeritage)
	c := reconcileWith(t,
		all, virtualization, custom, customLeaf, planted, relabelled, namedLikeDeckhouse,
		legacyCapability("d8:manage:permission:subsystem:virtualization:manage_resources", "virtualization", "manager", "d8-virtualization"),
		manageBinding("allmgr", "d8:manage:all:manager"),
		manageBinding("virt", "d8:manage:virtualization:manager"),
	)

	want := []string{
		"d8-virtualization/d8:use:user:binding:allmgr -> d8:use:role:user",
		"d8-virtualization/d8:use:user:binding:virt -> d8:use:role:user",
	}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// A RoleBinding the previous release made for a ClusterRoleBinding to an old manage role stays while
// the ClusterRoleBinding binds the alias, even when the selectors no longer reach its namespace, as
// when a module released on its own moves to the labels of the current scheme. It follows the
// subjects of the ClusterRoleBinding. Only a RoleBinding with the name, the related-with annotation
// and the roleRef the reconciler gives it is kept.
func TestReconcile_LegacyAliasKeepsItsRoleBindingsWhileTheBindingStays(t *testing.T) {
	t.Parallel()
	alias := legacyAlias(t, "d8:manage:networking:manager", "user",
		map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
		formerModules("network", "manager", "cni-cilium"),
		oldLineage("networking", "manager"),
	)
	kept := legacyUseBinding("user", "netmgr", "d8-ingress-nginx")
	drifted := legacyUseBinding("user", "netmgr", "d8-log-shipper")
	drifted.Subjects = []rbacv1.Subject{{Kind: "User", APIGroup: rbacv1.GroupName, Name: "someone-else"}}
	foreignAnnotation := legacyUseBinding("user", "netmgr", "d8-monitoring")
	foreignAnnotation.Annotations[relatedWithAnnotation] = "other"
	otherRole := legacyUseBinding("user", "netmgr", "d8-loki")
	otherRole.RoleRef.Name = "d8:use:role:viewer"
	foreignName := legacyUseBinding("user", "other", "d8-istio")
	foreignName.Annotations[relatedWithAnnotation] = "netmgr"
	c := newClient(t,
		alias,
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		// ingress-nginx moved to the labels of the current scheme at the upgrade
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:system-capability:ingress-nginx:edit", Labels: map[string]string{
			labelHeritage: "deckhouse", "module": "ingress-nginx", LabelKind: "capability", LabelScope: ScopeSystem,
			LabelNamespace: "d8-ingress-nginx", "rbac.deckhouse.io/aggregate-to-network-as": "manager",
		}}},
		manageBinding("netmgr", "d8:manage:networking:manager"),
		kept, drifted, foreignAnnotation, otherRole, foreignName,
		// the ClusterRoleBinding of this one is gone
		legacyUseBinding("user", "gone", "d8-ingress-nginx"),
	)
	before := mustExist(t, c, "d8-ingress-nginx", "d8:use:user:binding:netmgr")

	reconcileOnce(t, c)

	want := []string{
		"d8-cni-cilium/d8:use:user:binding:netmgr -> d8:use:role:user",
		"d8-ingress-nginx/d8:use:user:binding:netmgr -> d8:use:role:user",
		"d8-log-shipper/d8:use:user:binding:netmgr -> d8:use:role:user",
	}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
	if after := mustExist(t, c, "d8-ingress-nginx", "d8:use:user:binding:netmgr"); after.ResourceVersion != before.ResourceVersion {
		t.Errorf("a kept RoleBinding that is already correct must not be rewritten")
	}
	if rb := mustExist(t, c, "d8-log-shipper", "d8:use:user:binding:netmgr"); len(rb.Subjects) != 1 || rb.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v, want the subjects of the ClusterRoleBinding", rb.Subjects)
	}
	// A RoleBinding named for another ClusterRoleBinding is neither kept nor copied under this name.
	mustNotExist(t, c, "d8-istio", "d8:use:user:binding:other")
	mustNotExist(t, c, "d8-istio", "d8:use:user:binding:netmgr")
}

// A manager alias projects viewer where the previous release projected user. Its kept RoleBindings
// d8:use:user:* are replaced by d8:use:viewer:*, in the namespaces its selectors reach and in those
// only the kept RoleBindings hold, such as that of a module that moved to the labels of the current
// scheme, so its subjects keep read access everywhere the previous release gave them access.
func TestReconcile_LoweredAliasReplacesItsKeptRoleBindings(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		legacyAlias(t, "d8:manage:networking:manager", "viewer",
			map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
			formerModules("network", "manager", "cni-cilium"),
		),
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		manageBinding("netmgr", "d8:manage:networking:manager"),
		legacyUseBinding("user", "netmgr", "d8-cni-cilium"),
		// ingress-nginx moved to the labels of the current scheme at the upgrade
		legacyUseBinding("user", "netmgr", "d8-ingress-nginx"),
	)

	want := []string{
		"d8-cni-cilium/d8:use:viewer:binding:netmgr -> d8:use:role:viewer",
		"d8-ingress-nginx/d8:use:viewer:binding:netmgr -> d8:use:role:viewer",
	}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// A kept RoleBinding is never raised: a d8:use:viewer:* RoleBinding in a namespace only the kept
// RoleBindings hold does not become d8:use:user:* when the alias projects user.
func TestReconcile_KeptRoleBindingIsNeverRaised(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		legacyAlias(t, "d8:manage:networking:manager", "user",
			map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
			formerModules("network", "manager", "cni-cilium"),
		),
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		manageBinding("netmgr", "d8:manage:networking:manager"),
		legacyUseBinding("viewer", "netmgr", "d8-ingress-nginx"),
	)

	want := []string{"d8-cni-cilium/d8:use:user:binding:netmgr -> d8:use:role:user"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// A ClusterRoleBinding deleted and created again under the same name, here on another alias with
// the same use role, does not take over the RoleBindings its predecessor kept outside the selectors
// of its own alias. The reconciler and the hook of the previous release create a RoleBinding only
// once its ClusterRoleBinding exists, so a RoleBinding older than the ClusterRoleBinding was made
// for the predecessor and goes. Those made for the current ClusterRoleBinding stay, the one in the
// namespace of a module that moved to the new labels among them, and so does a RoleBinding in a
// namespace the selectors reach, whatever its age.
func TestReconcile_RecreatedBindingDoesNotKeepTheRoleBindingsOfItsPredecessor(t *testing.T) {
	t.Parallel()
	created := metav1.NewTime(time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC))
	earlier := metav1.NewTime(created.Add(-time.Hour))
	withTimestamp := func(rb *rbacv1.RoleBinding, at metav1.Time) *rbacv1.RoleBinding {
		rb.CreationTimestamp = at
		return rb
	}
	alias := legacyAlias(t, "d8:manage:networking:manager", "user",
		map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
		formerModules("network", "manager", "cni-cilium"),
	)
	binding := manageBinding("team", "d8:manage:networking:manager")
	binding.CreationTimestamp = created
	c := reconcileWith(t,
		alias,
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		binding,
		// made for a predecessor bound to d8:manage:all:manager, which reached these namespaces
		withTimestamp(legacyUseBinding("user", "team", "d8-system"), earlier),
		withTimestamp(legacyUseBinding("user", "team", "kube-system"), earlier),
		// made for the current ClusterRoleBinding before ingress-nginx moved to the new labels
		withTimestamp(legacyUseBinding("user", "team", "d8-ingress-nginx"), created),
		withTimestamp(legacyUseBinding("user", "team", "d8-log-shipper"), metav1.NewTime(created.Add(time.Minute))),
		// in a namespace the selectors reach
		withTimestamp(legacyUseBinding("user", "team", "d8-cni-cilium"), earlier),
	)

	want := []string{
		"d8-cni-cilium/d8:use:user:binding:team -> d8:use:role:user",
		"d8-ingress-nginx/d8:use:user:binding:team -> d8:use:role:user",
		"d8-log-shipper/d8:use:user:binding:team -> d8:use:role:user",
	}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// A kept RoleBinding goes with its ClusterRoleBinding, and with the alias: once the ClusterRoleBinding
// binds no alias of an old manage role, nothing keeps it.
func TestReconcile_KeptRoleBindingsGoWithTheBinding(t *testing.T) {
	t.Parallel()
	alias := legacyAlias(t, "d8:manage:networking:manager", "user",
		map[string]string{"rbac.deckhouse.io/aggregate-to-network-as": "manager"},
		oldLineage("networking", "manager"),
	)
	notAnAlias := legacyAlias(t, "d8:manage:storage:manager", "user",
		map[string]string{"rbac.deckhouse.io/aggregate-to-storage-as": "manager"},
		oldLineage("storage", "manager"),
	)
	delete(notAnAlias.Labels, LabelUseRole)
	c := reconcileWith(t,
		alias, notAnAlias,
		manageBinding("storagemgr", "d8:manage:storage:manager"),
		manageBinding("missing", "d8:manage:missing:manager"),
		legacyUseBinding("user", "netmgr", "d8-ingress-nginx"),
		legacyUseBinding("user", "storagemgr", "d8-sds-node-configurator"),
		legacyUseBinding("user", "missing", "d8-ingress-nginx"),
	)

	if got := useBindingsOf(t, c); len(got) != 0 {
		t.Errorf("use bindings = %v, want none", got)
	}
}

// The roles of the current scheme project as before: the capabilities of the previous scheme that
// modules released on their own still ship are not in their graph, even when the aggregation
// controller merges them by the lineage label.
func TestReconcile_CurrentRolesDoNotReachTheCapabilitiesOfThePreviousScheme(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		moduleCapability("okmeter", "observability", "manager", "d8-okmeter"),
		legacyCapability("d8:manage:permission:module:prometheus:edit", "observability", "manager", "d8-monitoring"),
		systemRole("d8:subsystem:observability:manager", ScopeSubsystem, "observability"),
		manageBinding("obsmgr", "d8:subsystem:observability:manager"),
	)

	want := []string{"d8-okmeter/d8:namespace:admin:binding:obsmgr -> d8:namespace:admin"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

// Moving a binding from an alias to the role that replaces it replaces the projections of the
// previous release with the ones of the current scheme.
func TestReconcile_BindingMovedOffTheAliasGetsTheCurrentProjection(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		moduleCapability("cni-cilium", "network", "manager", "d8-cni-cilium"),
		systemRole("d8:subsystem:network:manager", ScopeSubsystem, "network"),
		manageBinding("netmgr", "d8:subsystem:network:manager"),
		legacyUseBinding("user", "netmgr", "d8-cni-cilium"),
	)

	want := []string{"d8-cni-cilium/d8:namespace:admin:binding:netmgr -> d8:namespace:admin"}
	if got := useBindingsOf(t, c); !slices.Equal(got, want) {
		t.Errorf("use bindings = %v, want %v", got, want)
	}
}

func TestIsLegacyManageBinding(t *testing.T) {
	t.Parallel()
	custom := legacyAlias(t, "custom:manage:mycustom:manager", "user", map[string]string{LabelKind: KindManage})
	delete(custom.Labels, labelHeritage)
	c := newClient(t,
		legacyAlias(t, "d8:manage:all:viewer", "viewer", map[string]string{"rbac.deckhouse.io/aggregate-to-system-as": "viewer"}),
		custom,
		systemRole("d8:subsystem:network:manager", ScopeSubsystem, "network"),
	)
	if !IsLegacyManageBinding(c, manageBinding("x", "d8:manage:all:viewer")) {
		t.Error("a binding to an alias of a manage role of the previous release must match")
	}
	if IsLegacyManageBinding(c, manageBinding("x", "custom:manage:mycustom:manager")) {
		t.Error("a binding to a role of the previous scheme that Deckhouse does not render must not match")
	}
	if IsLegacyManageBinding(c, manageBinding("x", "d8:subsystem:network:manager")) {
		t.Error("a binding to a role of the current scheme is not a binding of the previous one")
	}
	if IsLegacyManageBinding(c, manageBinding("x", "d8:manage:missing:viewer")) {
		t.Error("a binding to a ClusterRole that is not in the cache must not match")
	}
	outside := legacyAlias(t, "user-authz:scale", "admin", map[string]string{LabelKind: KindManage})
	noUseRole := legacyAlias(t, "d8:manage:permission:module:ingress-nginx:edit", "user", map[string]string{LabelKind: KindManage})
	delete(noUseRole.Labels, LabelUseRole)
	c = newClient(t, outside, noUseRole)
	if IsLegacyManageBinding(c, manageBinding("x", "user-authz:scale")) {
		t.Error("a binding to a role of Deckhouse that is not named d8:manage: must not match")
	}
	if IsLegacyManageBinding(c, manageBinding("x", "d8:manage:permission:module:ingress-nginx:edit")) {
		t.Error("a binding to a role without a use role must not match")
	}
}

func TestLegacyRoleSelector(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]bool{KindManage: true, "use": false, "capability": false, "role": false, "": false} {
		if got := LegacyRoleSelector.Matches(labels.Set(map[string]string{LabelKind: kind})); got != want {
			t.Errorf("kind %q: selector matches = %v, want %v", kind, got, want)
		}
	}
	both := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:manage:networking:manager", Labels: map[string]string{
		LabelKind: KindManage, labelHeritage: "deckhouse", LabelUseRole: "user", LabelScope: ScopeSubsystem,
	}}}
	if isLegacyManageRole(both) {
		t.Error("a role with the scope of the current scheme is projected the current way")
	}
}
