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
	"controller/internal/resolve"
)

// PolicyConditionSelectorsValid is the ClusterResourceGrantPolicy condition that says every label
// selector of the policy compiles. The resolver treats a selector it cannot compile as matching
// nothing, which is the safe direction for an allow-list -- and exactly the direction nobody
// notices: the project simply gets less than the author meant. The condition is where the author
// finds out.
const PolicyConditionSelectorsValid = "SelectorsValid"

// PolicyConditionAllowedEffective says that every name a policy allows can actually be granted.
// The registration of a resource may exclude objects from every project -- the clusterroles
// registration excludes every ClusterRole not marked rbac.deckhouse.io/delegatable=true -- and an
// exclusion wins over an allow-list, so a policy that allows such a name grants nothing and nobody
// told the author. This condition tells them, with the name and the reason.
const PolicyConditionAllowedEffective = "AllowedEffective"

// PolicyReconciler keeps the status of a ClusterResourceGrantPolicy honest about its own spec.
type PolicyReconciler struct {
	client.Client
	// Mapper resolves the granted resources of the definitions the policy names; the same resolver
	// the webhooks use answers whether an allowed name would be granted.
	Mapper apimeta.RESTMapper
}

// Reconcile validates the policy and records the outcome as conditions.
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &v1alpha1.ClusterResourceGrantPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	selectors := metav1.Condition{Type: PolicyConditionSelectorsValid, ObservedGeneration: policy.Generation}
	if problems := invalidSelectors(policy); len(problems) == 0 {
		selectors.Status = metav1.ConditionTrue
		selectors.Reason = "Valid"
		selectors.Message = "Every label selector of the policy compiles."
	} else {
		selectors.Status = metav1.ConditionFalse
		selectors.Reason = "InvalidSelector"
		selectors.Message = strings.Join(problems, "; ") + ". A selector that does not compile matches nothing, so the projects get less than the policy intends."
	}

	effective := metav1.Condition{Type: PolicyConditionAllowedEffective, ObservedGeneration: policy.Generation}
	ineffective, err := r.ineffectiveAllowed(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check allowed names: %w", err)
	}
	if len(ineffective) == 0 {
		effective.Status = metav1.ConditionTrue
		effective.Reason = "Effective"
		effective.Message = "Every allowed name can be granted."
	} else {
		effective.Status = metav1.ConditionFalse
		effective.Reason = "ExcludedByDefinition"
		effective.Message = strings.Join(ineffective, "; ")
	}

	before := policy.Status.DeepCopy()
	policy.Status.ObservedGeneration = policy.Generation
	apimeta.SetStatusCondition(&policy.Status.Conditions, selectors)
	apimeta.SetStatusCondition(&policy.Status.Conditions, effective)
	if before.ObservedGeneration == policy.Status.ObservedGeneration &&
		conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionSelectorsValid) &&
		conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionAllowedEffective) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, fmt.Errorf("update policy status: %w", err)
	}
	return ctrl.Result{}, nil
}

// ineffectiveAllowed names every allowed entry the resolver would refuse anyway: the registration
// excludes it. Entries whose definition is missing are the binding reconciler's business and are
// left alone here; a value-backed definition has no objects to exclude.
func (r *PolicyReconciler) ineffectiveAllowed(ctx context.Context, policy *v1alpha1.ClusterResourceGrantPolicy) ([]string, error) {
	if r.Mapper == nil {
		return nil, nil
	}
	var out []string
	for i := range policy.Spec.Resources {
		entry := policy.Spec.Resources[i]
		if len(entry.Allowed) == 0 {
			continue
		}
		def := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := r.Get(ctx, client.ObjectKey{Name: entry.ResourceName}, def); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return nil, err
		}
		if def.IsValueBacked() || len(def.Spec.Excluded) == 0 {
			continue
		}
		resolved, err := resolve.Resolve(ctx, r.Client, r.Mapper, def, []v1alpha1.GrantResource{entry})
		if err != nil {
			// The granted kind is not served (its CRD is absent): nothing can be said about the names.
			continue
		}
		for _, name := range entry.Allowed {
			if resolved.Decide(name) {
				continue
			}
			reason := "it is excluded by the GrantableClusterResourceDefinition"
			if def.Spec.GrantedResource != nil && def.Spec.GrantedResource.Kind == "ClusterRole" {
				reason = "the ClusterRole is not marked rbac.deckhouse.io/delegatable=true, so it cannot be granted to projects"
			}
			out = append(out, fmt.Sprintf("spec.resources[%d].allowed (%s): %q grants nothing: %s", i, entry.ResourceName, name, reason))
		}
	}
	return out, nil
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
