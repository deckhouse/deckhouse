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

package controllers

import (
	"context"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/api/v1alpha1"
)

func policyCondition(t *testing.T, r *PolicyReconciler, name string) *metav1.Condition {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	policy := &v1alpha1.ClusterResourceGrantPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, policy); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(policy.Status.Conditions, PolicyConditionSelectorsValid)
	if cond == nil {
		t.Fatalf("condition %s missing on %s", PolicyConditionSelectorsValid, name)
	}
	if policy.Status.ObservedGeneration != policy.Generation {
		t.Fatalf("observedGeneration %d != generation %d", policy.Status.ObservedGeneration, policy.Generation)
	}
	return cond
}

// TestPolicy_InvalidSelectorIsReported: a matchExpressions entry the API server accepts but the
// selector library refuses (In without values) used to become a silent "matches nothing"; the
// policy now says so in its status.
func TestPolicy_InvalidSelectorIsReported(t *testing.T) {
	bad := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Generation: 3},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceName:    "storageclasses",
				AllowedSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "tier", Operator: metav1.LabelSelectorOpIn}}},
			}},
		},
	}
	good := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "good", Generation: 1},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
	r := &PolicyReconciler{Client: buildClientWithStatus(t, bad, good)}

	cond := policyCondition(t, r, "bad")
	if cond.Status != metav1.ConditionFalse || cond.Reason != "InvalidSelector" {
		t.Fatalf("expected False/InvalidSelector, got %s/%s", cond.Status, cond.Reason)
	}
	if !strings.Contains(cond.Message, "spec.resources[0].allowedSelector (storageclasses)") {
		t.Fatalf("message must point at the selector, got %q", cond.Message)
	}

	cond = policyCondition(t, r, "good")
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("a valid policy must be True, got %s: %s", cond.Status, cond.Message)
	}
}

func rolesMapper() apimeta.RESTMapper {
	m := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "rbac.authorization.k8s.io", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, apimeta.RESTScopeRoot)
	return m
}

// TestPolicy_NonDelegatableRoleIsReported: the clusterroles registration excludes every ClusterRole
// not marked delegatable, and an exclusion wins over an allow-list, so a policy that allows such a
// role grants nothing. USAGE told administrators to write exactly that policy; the condition now
// tells them why nothing happened.
func TestPolicy_NonDelegatableRoleIsReported(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "clusterroles"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			Excluded: []v1alpha1.ResourceFilter{{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "rbac.deckhouse.io/delegatable", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"true"},
			}}}},
		},
	}
	delegatable := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "team-viewer", Labels: map[string]string{"rbac.deckhouse.io/delegatable": "true"}}}
	plain := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "team-editor"}}
	policy := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "roles", Generation: 1},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "clusterroles", Allowed: []string{"team-viewer", "team-editor"}}},
		},
	}
	r := &PolicyReconciler{Client: buildClientWithStatus(t, def, delegatable, plain, policy), Mapper: rolesMapper()}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil {
		t.Fatal(err)
	}
	got := &v1alpha1.ClusterResourceGrantPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "roles"}, got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, PolicyConditionAllowedEffective)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("expected AllowedEffective=False, got %+v", cond)
	}
	if !strings.Contains(cond.Message, `"team-editor"`) || !strings.Contains(cond.Message, "delegatable") || strings.Contains(cond.Message, `"team-viewer"`) {
		t.Fatalf("message must name the non-delegatable role only, got %q", cond.Message)
	}

	// A ClusterRole label change is what heals the condition, so the label watch must enqueue
	// this policy, and until then the reconciler asks to be re-run on the catalog cadence.
	if reqs := r.policiesAllowing(context.Background(), plain); len(reqs) != 1 || reqs[0].Name != "roles" {
		t.Fatalf("a ClusterRole change must enqueue the allow-list policy, got %v", reqs)
	}
	if res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil || res.RequeueAfter != ResyncInterval {
		t.Fatalf("an ineffective policy must requeue after %s, got %+v %v", ResyncInterval, res, err)
	}

	// Marking the role delegatable clears the condition on the next reconcile.
	plain.Labels = map[string]string{"rbac.deckhouse.io/delegatable": "true"}
	if err := r.Update(context.Background(), plain); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "roles"}, got); err != nil {
		t.Fatal(err)
	}
	if cond := apimeta.FindStatusCondition(got.Status.Conditions, PolicyConditionAllowedEffective); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected AllowedEffective=True once the role is delegatable, got %+v", cond)
	}
	if res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil || res.RequeueAfter != 0 {
		t.Fatalf("an effective policy must not requeue, got %+v %v", res, err)
	}
}
