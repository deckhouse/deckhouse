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

// Package projectrolebinding reconciles ProjectRoleBinding objects by fanning out a service
// RoleBinding (d8:prb:<name>) into every namespace of the target project.
package projectrolebinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

// Reconciler fans out service RoleBindings for ProjectRoleBinding objects.
type Reconciler struct {
	client.Client
}

// Reconcile keeps the service RoleBindings of a single ProjectRoleBinding in sync with the
// namespaces of its project.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("projectrolebinding", req.NamespacedName.String())

	prb := &v1alpha3.ProjectRoleBinding{}
	if err := r.Get(ctx, req.NamespacedName, prb); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ProjectRoleBinding: %w", err)
	}

	// The project's main namespace equals the project name, which equals the binding namespace.
	project := &v1alpha3.Project{}
	projectFound := true
	if err := r.Get(ctx, types.NamespacedName{Name: req.Namespace}, project); err != nil {
		if !k8serrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("get project: %w", err)
		}
		projectFound = false
	}

	// Cleanup path: the binding is being deleted, or its project is gone/terminating.
	if !prb.DeletionTimestamp.IsZero() || !projectFound || !project.DeletionTimestamp.IsZero() {
		if err := r.cleanup(ctx, prb.Name, req.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.ContainsFinalizer(prb, v1alpha3.ProjectRoleBindingFinalizer) {
			controllerutil.RemoveFinalizer(prb, v1alpha3.ProjectRoleBindingFinalizer)
			if err := r.Update(ctx, prb); err != nil && !k8serrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(prb, v1alpha3.ProjectRoleBindingFinalizer) {
		controllerutil.AddFinalizer(prb, v1alpha3.ProjectRoleBindingFinalizer)
		if err := r.Update(ctx, prb); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	// Defense in depth: the admission webhook already restricts roleRef, but never fan out a
	// forbidden role even if the webhook was bypassed or the role was disabled after binding.
	if !rolebinding.IsRoleAllowed(prb.Spec.RoleRef.Name) {
		log.Info("roleRef is not allowed for project bindings, cleaning up", "roleRef", prb.Spec.RoleRef.Name)
		if err := r.cleanup(ctx, prb.Name, prb.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		message := fmt.Sprintf("roleRef %q is not allowed for project bindings", prb.Spec.RoleRef.Name)
		if v1alpha3.SetCondition(&prb.Status.Conditions, v1alpha3.ProjectRoleBindingConditionReady, corev1.ConditionFalse, message) {
			if err := r.Status().Update(ctx, prb); err != nil {
				return ctrl.Result{}, fmt.Errorf("update status: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	target := rolebinding.ProjectNamespaceNames(project)
	related := fmt.Sprintf("%s/%s", prb.Namespace, prb.Name)

	// Fan out into every namespace, accumulating per-namespace errors so a single bad namespace
	// does not block the rest of the project (relevant at scale).
	var errs []error
	for _, ns := range target {
		if err := r.upsertRoleBinding(ctx, prb, ns, related); err != nil {
			errs = append(errs, err)
		}
	}

	if err := r.pruneRoleBindings(ctx, prb.Name, prb.Namespace, target); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	// Write status only when it actually changed: an unconditional write would bump the condition
	// timestamps and re-enqueue this object through the For() watch, causing a reconcile hot-loop.
	changed := false
	if prb.Status.ObservedGeneration != prb.Generation {
		prb.Status.ObservedGeneration = prb.Generation
		changed = true
	}
	if v1alpha3.SetCondition(&prb.Status.Conditions, v1alpha3.ProjectRoleBindingConditionReady, corev1.ConditionTrue, "") {
		changed = true
	}
	if changed {
		if err := r.Status().Update(ctx, prb); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
	}

	log.Info("the project role binding reconciled", "namespaces", len(target))
	return ctrl.Result{}, nil
}

func (r *Reconciler) upsertRoleBinding(ctx context.Context, prb *v1alpha3.ProjectRoleBinding, ns, related string) error {
	// The main-namespace RoleBinding is owned by the PRB (same namespace); cross-namespace
	// ownerReferences are not allowed, so additional namespaces rely on label-based cleanup.
	var setOwner func(*rbacv1.RoleBinding) error
	if ns == prb.Namespace {
		setOwner = func(rb *rbacv1.RoleBinding) error {
			return controllerutil.SetControllerReference(prb, rb, r.Scheme())
		}
	}
	return rolebinding.UpsertServiceRoleBinding(ctx, r.Client, rolebinding.UpsertParams{
		Name:        rolebinding.PRBServiceName(prb.Name),
		Namespace:   ns,
		Project:     prb.Namespace,
		OwnerLabel:  v1alpha3.ResourceLabelOwnedByPRB,
		OwnerName:   prb.Name,
		RelatedWith: related,
		Subjects:    prb.Spec.Subjects,
		RoleRef:     prb.Spec.RoleRef.Name,
	}, setOwner)
}

// pruneRoleBindings deletes service RoleBindings of this PRB in namespaces that are no longer part
// of the target set. It is scoped to the project (PRB names are only unique within a project
// namespace), so it never touches bindings of an identically named PRB in another project.
func (r *Reconciler) pruneRoleBindings(ctx context.Context, name, project string, target []string) error {
	keep := make(map[string]struct{}, len(target))
	for _, ns := range target {
		keep[ns] = struct{}{}
	}
	return rolebinding.PruneServiceRoleBindings(ctx, r.Client, map[string]string{
		v1alpha3.ResourceLabelOwnedByPRB: name,
		v1alpha3.ResourceLabelProject:    project,
	}, keep)
}

// cleanup removes every service RoleBinding fanned out by the named PRB within its project.
func (r *Reconciler) cleanup(ctx context.Context, name, project string) error {
	return r.pruneRoleBindings(ctx, name, project, nil)
}

// SetupWithManager wires the reconciler and its watches.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueByProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		// a project change enqueues all PRBs in its main namespace
		list := &v1alpha3.ProjectRoleBindingList{}
		if err := r.List(ctx, list, client.InNamespace(obj.GetName())); err != nil {
			ctrllog.FromContext(ctx).Error(err, "list ProjectRoleBindings for project watch", "project", obj.GetName())
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: list.Items[i].Namespace, Name: list.Items[i].Name}})
		}
		return reqs
	})

	enqueueByOwnedRoleBinding := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		name, ok := obj.GetLabels()[v1alpha3.ResourceLabelOwnedByPRB]
		if !ok {
			return nil
		}
		related := obj.GetAnnotations()[v1alpha3.ResourceAnnotationRelatedWith]
		parts := strings.SplitN(related, "/", 2)
		if len(parts) != 2 {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: parts[0], Name: name}}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		// Only spec changes (generation bumps) re-enqueue the PRB itself; status writes must not,
		// or the reconcile loops on its own writes. The owned-RoleBinding watch below still catches
		// external drift of the fanned-out bindings.
		For(&v1alpha3.ProjectRoleBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha3.Project{}, enqueueByProject, builder.WithPredicates(rolebinding.ProjectFanoutPredicate())).
		Watches(&rbacv1.RoleBinding{}, enqueueByOwnedRoleBinding).
		Named("project-role-binding").
		Complete(r)
}
