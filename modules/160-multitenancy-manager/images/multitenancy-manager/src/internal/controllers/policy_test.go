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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
