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
	"fmt"
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
)

// PolicyConditionSelectorsValid is the ClusterResourceGrantPolicy condition that says every label
// selector of the policy compiles. The resolver treats a selector it cannot compile as matching
// nothing, which is the safe direction for an allow-list -- and exactly the direction nobody
// notices: the project simply gets less than the author meant. The condition is where the author
// finds out.
const PolicyConditionSelectorsValid = "SelectorsValid"

// PolicyReconciler keeps the status of a ClusterResourceGrantPolicy honest about its own spec.
type PolicyReconciler struct {
	client.Client
}

// Reconcile validates the selectors of the policy and records the outcome as a condition.
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &v1alpha1.ClusterResourceGrantPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	problems := invalidSelectors(policy)
	cond := metav1.Condition{Type: PolicyConditionSelectorsValid, ObservedGeneration: policy.Generation}
	if len(problems) == 0 {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "Valid"
		cond.Message = "Every label selector of the policy compiles."
	} else {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InvalidSelector"
		cond.Message = strings.Join(problems, "; ") + ". A selector that does not compile matches nothing, so the projects get less than the policy intends."
	}

	before := policy.Status.DeepCopy()
	policy.Status.ObservedGeneration = policy.Generation
	apimeta.SetStatusCondition(&policy.Status.Conditions, cond)
	if before.ObservedGeneration == policy.Status.ObservedGeneration && conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionSelectorsValid) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, fmt.Errorf("update policy status: %w", err)
	}
	return ctrl.Result{}, nil
}

// invalidSelectors names every selector of the policy that does not compile, in the words the
// author needs to find it.
func invalidSelectors(policy *v1alpha1.ClusterResourceGrantPolicy) []string {
	var problems []string
	if policy.Spec.ProjectSelector != nil {
		if _, err := metav1.LabelSelectorAsSelector(policy.Spec.ProjectSelector); err != nil {
			problems = append(problems, fmt.Sprintf("spec.projectSelector: %v", err))
		}
	}
	for i := range policy.Spec.Resources {
		entry := &policy.Spec.Resources[i]
		if entry.AllowedSelector != nil {
			if _, err := metav1.LabelSelectorAsSelector(entry.AllowedSelector); err != nil {
				problems = append(problems, fmt.Sprintf("spec.resources[%d].allowedSelector (%s): %v", i, entry.ResourceName, err))
			}
		}
		if entry.DeniedSelector != nil {
			if _, err := metav1.LabelSelectorAsSelector(entry.DeniedSelector); err != nil {
				problems = append(problems, fmt.Sprintf("spec.resources[%d].deniedSelector (%s): %v", i, entry.ResourceName, err))
			}
		}
	}
	return problems
}

// conditionUnchanged reports whether the named condition has the same status, reason and message on
// both sides, ignoring the transition timestamp SetStatusCondition refreshes.
func conditionUnchanged(before, after []metav1.Condition, condType string) bool {
	b := apimeta.FindStatusCondition(before, condType)
	a := apimeta.FindStatusCondition(after, condType)
	if b == nil || a == nil {
		return b == a
	}
	return b.Status == a.Status && b.Reason == a.Reason && b.Message == a.Message && b.ObservedGeneration == a.ObservedGeneration
}

// SetupWithManager wires the reconciler.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterResourceGrantPolicy{}).
		Named("grant-policy-status").
		Complete(r)
}
