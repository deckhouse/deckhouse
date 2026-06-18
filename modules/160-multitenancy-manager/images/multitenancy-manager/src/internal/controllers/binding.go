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
	"sort"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/api/v1alpha1"
)

// ReferenceReconciler keeps GrantableClusterResourceReference.status.bound in sync with whether the
// named GrantableClusterResourceDefinition exists.
type ReferenceReconciler struct {
	client.Client
}

// Reconcile sets the reference's Bound status.
func (r *ReferenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ref := &v1alpha1.GrantableClusterResourceReference{}
	if err := r.Get(ctx, req.NamespacedName, ref); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	def := &v1alpha1.GrantableClusterResourceDefinition{}
	err := r.Get(ctx, types.NamespacedName{Name: ref.Spec.GrantableClusterResourceName}, def)
	bound := err == nil
	if err != nil && !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get definition %s: %w", ref.Spec.GrantableClusterResourceName, err)
	}

	cond := metav1.Condition{Type: "Bound", ObservedGeneration: ref.Generation}
	if bound {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "Resolved"
		cond.Message = fmt.Sprintf("GrantableClusterResourceDefinition %q exists.", ref.Spec.GrantableClusterResourceName)
	} else {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "UnknownResource"
		cond.Message = fmt.Sprintf("GrantableClusterResourceDefinition %q not found.", ref.Spec.GrantableClusterResourceName)
	}

	ref.Status.Bound = bound
	ref.Status.ObservedGeneration = ref.Generation
	apimeta.SetStatusCondition(&ref.Status.Conditions, cond)
	if err := r.Status().Update(ctx, ref); err != nil {
		return ctrl.Result{}, fmt.Errorf("update reference status: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the reference reconciler; a definition change re-evaluates references naming it.
func (r *ReferenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueNamingRefs := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			refList := &v1alpha1.GrantableClusterResourceReferenceList{}
			if err := r.List(ctx, refList); err != nil {
				return nil
			}
			var reqs []reconcile.Request
			for i := range refList.Items {
				if refList.Items[i].Spec.GrantableClusterResourceName == obj.GetName() {
					reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: refList.Items[i].Name}})
				}
			}
			return reqs
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrantableClusterResourceReference{}).
		Watches(&v1alpha1.GrantableClusterResourceDefinition{}, enqueueNamingRefs).
		Named("reference-binding").
		Complete(r)
}

// DefinitionReconciler maintains GrantableClusterResourceDefinition.status.references — the reverse
// index of references bound to it.
type DefinitionReconciler struct {
	client.Client
}

// Reconcile rebuilds the definition's reference list.
func (r *DefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	def := &v1alpha1.GrantableClusterResourceDefinition{}
	if err := r.Get(ctx, req.NamespacedName, def); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	refList := &v1alpha1.GrantableClusterResourceReferenceList{}
	if err := r.List(ctx, refList); err != nil {
		return ctrl.Result{}, fmt.Errorf("list references: %w", err)
	}
	var bindings []v1alpha1.ResourceReferenceBinding
	for i := range refList.Items {
		ref := &refList.Items[i]
		if ref.Spec.GrantableClusterResourceName != def.Name {
			continue
		}
		bindings = append(bindings, v1alpha1.ResourceReferenceBinding{
			Name:      ref.Name,
			Resources: ref.Spec.Rule.Resources,
		})
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Name < bindings[j].Name })

	def.Status.References = bindings
	def.Status.ReferenceCount = len(bindings)
	def.Status.ObservedGeneration = def.Generation
	if err := r.Status().Update(ctx, def); err != nil {
		return ctrl.Result{}, fmt.Errorf("update definition status: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager wires the definition reconciler; a reference change re-evaluates the definition it names.
func (r *DefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueNamedDef := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			ref, ok := obj.(*v1alpha1.GrantableClusterResourceReference)
			if !ok || ref.Spec.GrantableClusterResourceName == "" {
				return nil
			}
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ref.Spec.GrantableClusterResourceName}}}
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GrantableClusterResourceDefinition{}).
		Watches(&v1alpha1.GrantableClusterResourceReference{}, enqueueNamedDef).
		Named("definition-references").
		Complete(r)
}
