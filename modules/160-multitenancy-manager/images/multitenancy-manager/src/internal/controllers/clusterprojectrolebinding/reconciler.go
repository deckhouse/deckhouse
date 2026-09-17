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

// Package clusterprojectrolebinding reconciles ClusterProjectRoleBinding objects by fanning out a
// service RoleBinding (d8:cprb:<name>) into every namespace of every non-virtual project.
package clusterprojectrolebinding

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

// Reconciler fans out service RoleBindings for ClusterProjectRoleBinding objects.
type Reconciler struct {
	client.Client
}

// Reconcile keeps the service RoleBindings of a single ClusterProjectRoleBinding in sync with the
// namespaces of all non-virtual projects.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("clusterprojectrolebinding", req.Name)

	cprb := &v1alpha3.ClusterProjectRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, cprb); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ClusterProjectRoleBinding: %w", err)
	}

	if !cprb.DeletionTimestamp.IsZero() {
		if err := r.cleanup(ctx, cprb.Name); err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.ContainsFinalizer(cprb, v1alpha3.ClusterProjectRoleBindingFinalizer) {
			controllerutil.RemoveFinalizer(cprb, v1alpha3.ClusterProjectRoleBindingFinalizer)
			if err := r.Update(ctx, cprb); err != nil && !k8serrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(cprb, v1alpha3.ClusterProjectRoleBindingFinalizer) {
		controllerutil.AddFinalizer(cprb, v1alpha3.ClusterProjectRoleBindingFinalizer)
		if err := r.Update(ctx, cprb); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	// Defense in depth: the admission webhook already restricts roleRef, but never fan out a
	// forbidden role even if the webhook was bypassed or the role was disabled after binding.
	if !rolebinding.IsRoleAllowed(cprb.Spec.RoleRef.Name) {
		log.Info("roleRef is not allowed for project bindings, cleaning up", "roleRef", cprb.Spec.RoleRef.Name)
		if err := r.cleanup(ctx, cprb.Name); err != nil {
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("roleRef %q is not allowed for project bindings", cprb.Spec.RoleRef.Name)
		changed := false
		if cprb.Status.BoundProjects != 0 {
			cprb.Status.BoundProjects = 0
			changed = true
		}
		if v1alpha3.SetCondition(&cprb.Status.Conditions, v1alpha3.ClusterProjectRoleBindingConditionReady, corev1.ConditionFalse, message) {
			changed = true
		}
		if changed {
			if err := r.Status().Update(ctx, cprb); err != nil {
				return ctrl.Result{}, fmt.Errorf("update status: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	projects := &v1alpha3.ProjectList{}
	if err := r.List(ctx, projects); err != nil {
		return ctrl.Result{}, fmt.Errorf("list projects: %w", err)
	}

	// target maps each namespace that must carry the binding to its owning project.
	target := make(map[string]string, len(projects.Items))
	boundProjects := 0
	for i := range projects.Items {
		project := &projects.Items[i]
		if project.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" {
			continue
		}
		if !project.DeletionTimestamp.IsZero() {
			continue
		}
		boundProjects++
		for _, ns := range rolebinding.ProjectNamespaceNames(project) {
			target[ns] = project.Name
		}
	}

	// Fan out into every namespace, accumulating per-namespace errors so a single bad namespace
	// does not block the rest of the cluster (CPRB can span thousands of namespaces).
	var errs []error
	for ns, project := range target {
		if err := r.upsertRoleBinding(ctx, cprb, ns, project); err != nil {
			errs = append(errs, err)
		}
	}

	if err := r.pruneRoleBindings(ctx, cprb.Name, target); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	// Write status only when it actually changed: an unconditional write would bump the condition
	// timestamps and re-enqueue this object through the For() watch, causing a reconcile hot-loop.
	boundProjects32 := int32(min(boundProjects, 1<<31-1))
	changed := false
	if cprb.Status.ObservedGeneration != cprb.Generation {
		cprb.Status.ObservedGeneration = cprb.Generation
		changed = true
	}
	if cprb.Status.BoundProjects != boundProjects32 {
		cprb.Status.BoundProjects = boundProjects32
		changed = true
	}
	if v1alpha3.SetCondition(&cprb.Status.Conditions, v1alpha3.ClusterProjectRoleBindingConditionReady, corev1.ConditionTrue, "") {
		changed = true
	}
	if changed {
		if err := r.Status().Update(ctx, cprb); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
	}

	log.Info("the cluster project role binding reconciled", "namespaces", len(target), "projects", boundProjects)
	return ctrl.Result{}, nil
}

func (r *Reconciler) upsertRoleBinding(ctx context.Context, cprb *v1alpha3.ClusterProjectRoleBinding, ns, project string) error {
	return rolebinding.UpsertServiceRoleBinding(ctx, r.Client, rolebinding.UpsertParams{
		Name:        rolebinding.CPRBServiceName(cprb.Name),
		Namespace:   ns,
		Project:     project,
		OwnerLabel:  v1alpha3.ResourceLabelOwnedByCPRB,
		OwnerName:   cprb.Name,
		RelatedWith: cprb.Name,
		Subjects:    cprb.Spec.Subjects,
		RoleRef:     cprb.Spec.RoleRef.Name,
	}, nil)
}

func (r *Reconciler) pruneRoleBindings(ctx context.Context, name string, target map[string]string) error {
	keep := make(map[string]struct{}, len(target))
	for ns := range target {
		keep[ns] = struct{}{}
	}
	return rolebinding.PruneServiceRoleBindings(ctx, r.Client, map[string]string{
		v1alpha3.ResourceLabelOwnedByCPRB: name,
	}, keep)
}

func (r *Reconciler) cleanup(ctx context.Context, name string) error {
	return r.pruneRoleBindings(ctx, name, nil)
}

// SetupWithManager wires the reconciler and its watches. The fan-out is sequential
// (MaxConcurrentReconciles: 1) because every reconcile walks the full project list.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueAll := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		list := &v1alpha3.ClusterProjectRoleBindingList{}
		if err := r.List(ctx, list); err != nil {
			ctrllog.FromContext(ctx).Error(err, "list ClusterProjectRoleBindings for project watch")
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: list.Items[i].Name}})
		}
		return reqs
	})

	enqueueByOwnedRoleBinding := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		name, ok := obj.GetLabels()[v1alpha3.ResourceLabelOwnedByCPRB]
		if !ok {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		// Only spec changes (generation bumps) re-enqueue the CPRB itself; status writes must not,
		// or the reconcile loops on its own writes. The project watch is narrowed so that a project
		// status write only re-enqueues every CPRB when the namespace set actually changed.
		For(&v1alpha3.ClusterProjectRoleBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(&v1alpha3.Project{}, enqueueAll, builder.WithPredicates(rolebinding.ProjectFanoutPredicate())).
		Watches(&rbacv1.RoleBinding{}, enqueueByOwnedRoleBinding).
		Named("cluster-project-role-binding").
		Complete(r)
}
