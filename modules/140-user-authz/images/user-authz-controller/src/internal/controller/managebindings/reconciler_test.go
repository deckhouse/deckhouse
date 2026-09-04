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
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testUser = []rbacv1.Subject{{Kind: "User", APIGroup: rbacv1.GroupName, Name: "test"}}

// The fixtures mirror the ones of the hook this reconciler replaces.

func manageRole(name, level, subsystem string) *rbacv1.ClusterRole {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
			LabelUseRole:              "admin",
			"rbac.deckhouse.io/level": level,
			LabelKind:                 KindManage,
		}},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
			MatchLabels: map[string]string{
				LabelKind: KindManage,
				fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", subsystem): "manager",
			},
		}}},
	}
	if level != "all" {
		role.Labels["rbac.deckhouse.io/aggregate-to-all-as"] = "manager"
	}
	return role
}

func manageModuleRole(name, subsystem, namespace string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
		"rbac.deckhouse.io/level": "module",
		LabelKind:                 KindManage,
		LabelNamespace:            namespace,
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
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:use:role:admin"},
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
		WithIndex(&rbacv1.ClusterRoleBinding{}, ManageBindingIndexField, ManageBindingIndexValue).
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
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageModuleRole("d8:manage:permission:module:test2:edit", "others", "test2-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageBinding("test", "d8:manage:others:manager"),
		manageModuleRole("d8:manage:permission:module:test3:edit", "test", "test2-ns"),
		manageRole("d8:manage:test:manager", "subsystem", "test"),
		manageBinding("test2", "d8:manage:test:manager"),
	)

	rb := mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	if rb.RoleRef.Name != "d8:use:role:admin" || rb.Annotations[relatedWithAnnotatio] != "test" || rb.Labels[labelAutomated] != "true" {
		t.Errorf("use binding = %+v", rb)
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v", rb.Subjects)
	}
	mustExist(t, c, "test2-ns", "d8:use:admin:binding:test")
	mustExist(t, c, "test2-ns", "d8:use:admin:binding:test2")
	mustNotExist(t, c, "test-ns", "d8:use:admin:binding:test2")
}

func TestReconcile_AllBindingReachesEveryModuleNamespace(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageModuleRole("d8:manage:permission:module:test2:edit", "others", "test2-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageRole("d8:manage:all:manager", "all", "all"),
		manageBinding("test", "d8:manage:all:manager"),
	)

	mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	mustExist(t, c, "test2-ns", "d8:use:admin:binding:test")
}

func TestReconcile_NamespaceDroppingOutRemovesOnlyItsBinding(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageBinding("test", "d8:manage:others:manager"),
		automatedUseBinding("d8:use:admin:binding:test", "test2-ns"),
	)

	mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	mustNotExist(t, c, "test2-ns", "d8:use:admin:binding:test")
}

func TestReconcile_OrphanedAutomatedBindingsAreDeleted(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		automatedUseBinding("d8:binding:test", "test-ns"),
		automatedUseBinding("d8:binding:test2", "test-ns"),
		automatedUseBinding("d8:binding:test3", "test-ns2"),
		// a rule RoleBinding of the current model carries the module labels but not the automated one
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

func TestReconcile_RepairsDriftedUseBinding(t *testing.T) {
	t.Parallel()
	drifted := automatedUseBinding("d8:use:admin:binding:test", "test-ns")
	drifted.Subjects = []rbacv1.Subject{{Kind: "User", APIGroup: rbacv1.GroupName, Name: "someone-else"}}
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageBinding("test", "d8:manage:others:manager"),
		drifted,
	)

	rb := mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "test" {
		t.Errorf("subjects = %v, want the manage binding's subjects", rb.Subjects)
	}
	if rb.Annotations[relatedWithAnnotatio] != "test" {
		t.Errorf("annotations = %v", rb.Annotations)
	}
}

func TestReconcile_ManageRoleWithoutUseRoleIsIgnored(t *testing.T) {
	t.Parallel()
	role := manageRole("d8:manage:others:manager", "subsystem", "others")
	delete(role.Labels, LabelUseRole)
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		role,
		manageBinding("test", "d8:manage:others:manager"),
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
	role := manageRole("d8:manage:others:manager", "subsystem", "others")
	role.AggregationRule = &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: LabelKind, Operator: metav1.LabelSelectorOpIn, Values: []string{KindManage}},
			{Key: "rbac.deckhouse.io/aggregate-to-others-as", Operator: metav1.LabelSelectorOpExists},
		},
	}}}
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageModuleRole("d8:manage:permission:module:foreign:edit", "foreign", "foreign-ns"),
		role,
		manageBinding("test", "d8:manage:others:manager"),
	)

	mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	mustNotExist(t, c, "foreign-ns", "d8:use:admin:binding:test")
}

// Aggregation is followed to any depth: all -> middle -> subsystem -> module, with a cycle thrown in.
func TestReconcile_DeepAggregationIsFollowed(t *testing.T) {
	t.Parallel()
	all := manageRole("d8:manage:all:manager", "all", "all")
	subsystem := manageRole("d8:manage:others:manager", "subsystem", "others")
	// a role in the middle that aggregates the subsystem and is itself aggregated by "all"
	middle := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "d8:manage:middle", Labels: map[string]string{
			LabelKind: KindManage, "rbac.deckhouse.io/aggregate-to-all-as": "manager",
		}},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
			MatchLabels: map[string]string{LabelKind: KindManage, "rbac.deckhouse.io/level": "subsystem"},
		}}},
	}
	// the subsystem role points back at the middle role: a cycle that must terminate
	subsystem.AggregationRule.ClusterRoleSelectors = append(subsystem.AggregationRule.ClusterRoleSelectors,
		metav1.LabelSelector{MatchLabels: map[string]string{"rbac.deckhouse.io/aggregate-to-all-as": "manager"}})
	delete(subsystem.Labels, "rbac.deckhouse.io/aggregate-to-all-as")

	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		all, middle, subsystem,
		manageBinding("root", "d8:manage:all:manager"),
	)

	mustExist(t, c, "test-ns", "d8:use:admin:binding:root")
}

func TestReconcile_PreservesForeignMetadataWithoutChurn(t *testing.T) {
	t.Parallel()
	existing := automatedUseBinding("d8:use:admin:binding:test", "test-ns")
	existing.Annotations = map[string]string{"example.com/note": "keep me", relatedWithAnnotatio: "test"}
	existing.Labels["example.com/team"] = "blue"
	c := newClient(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageBinding("test", "d8:manage:others:manager"),
		existing,
	)
	before := mustExist(t, c, "test-ns", "d8:use:admin:binding:test")

	reconcileOnce(t, c)

	after := mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("a binding that is already correct must not be rewritten")
	}
	if after.Annotations["example.com/note"] != "keep me" || after.Labels["example.com/team"] != "blue" {
		t.Errorf("foreign metadata lost: %+v", after.ObjectMeta)
	}
}

func TestReconcile_RecreatesBindingWithStaleRoleRef(t *testing.T) {
	t.Parallel()
	stale := automatedUseBinding("d8:use:admin:binding:test", "test-ns")
	stale.RoleRef.Name = "d8:use:role:user"
	c := reconcileWith(t,
		manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
		manageRole("d8:manage:others:manager", "subsystem", "others"),
		manageBinding("test", "d8:manage:others:manager"),
		stale,
	)

	rb := mustExist(t, c, "test-ns", "d8:use:admin:binding:test")
	if rb.RoleRef.Name != "d8:use:role:admin" {
		t.Errorf("roleRef = %v, want the use role of the manage role", rb.RoleRef)
	}
}

func TestManageBindingIndexValue(t *testing.T) {
	t.Parallel()
	if got := ManageBindingIndexValue(manageBinding("x", "d8:manage:others:manager")); len(got) != 1 {
		t.Errorf("manage binding must be indexed, got %v", got)
	}
	if got := ManageBindingIndexValue(manageBinding("x", "cluster-admin")); got != nil {
		t.Errorf("unrelated binding must not be indexed, got %v", got)
	}
}
