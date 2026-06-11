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
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
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
		if err := r.cleanup(ctx, prb.Name); err != nil {
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

	target := rolebinding.ProjectNamespaceNames(project)
	related := fmt.Sprintf("%s/%s", prb.Namespace, prb.Name)

	for _, ns := range target {
		if err := r.upsertRoleBinding(ctx, prb, ns, related); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.pruneRoleBindings(ctx, prb.Name, target); err != nil {
		return ctrl.Result{}, err
	}

	prb.Status.ObservedGeneration = prb.Generation
	setReady(&prb.Status.Conditions)
	if err := r.Status().Update(ctx, prb); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}

	log.Info("the project role binding reconciled", "namespaces", len(target))
	return ctrl.Result{}, nil
}

func (r *Reconciler) upsertRoleBinding(ctx context.Context, prb *v1alpha3.ProjectRoleBinding, ns, related string) error {
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: rolebinding.PRBServiceName(prb.Name), Namespace: ns}}
	project := prb.Namespace
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		if rb.Labels == nil {
			rb.Labels = map[string]string{}
		}
		rb.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		rb.Labels[v1alpha3.ResourceLabelProject] = project
		rb.Labels[v1alpha3.ResourceLabelOwnedByPRB] = prb.Name
		if rb.Annotations == nil {
			rb.Annotations = map[string]string{}
		}
		rb.Annotations[v1alpha3.ResourceAnnotationRelatedWith] = related
		rb.Subjects = rolebinding.CopySubjects(prb.Spec.Subjects)
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     prb.Spec.RoleRef.Name,
		}
		// The main-namespace RoleBinding is owned by the PRB (same namespace); cross-namespace
		// ownerReferences are not allowed, so additional namespaces rely on label-based cleanup.
		if ns == prb.Namespace {
			return controllerutil.SetControllerReference(prb, rb, r.Scheme())
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert RoleBinding %s/%s: %w", ns, rb.Name, err)
	}
	return nil
}

// pruneRoleBindings deletes service RoleBindings of this PRB in namespaces that are no longer part
// of the target set.
func (r *Reconciler) pruneRoleBindings(ctx context.Context, name string, target []string) error {
	keep := make(map[string]struct{}, len(target))
	for _, ns := range target {
		keep[ns] = struct{}{}
	}

	list := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelOwnedByPRB: name}); err != nil {
		return fmt.Errorf("list service RoleBindings: %w", err)
	}
	for i := range list.Items {
		if _, ok := keep[list.Items[i].Namespace]; ok {
			continue
		}
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete stale RoleBinding %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}
	return nil
}

// cleanup removes every service RoleBinding fanned out by the named PRB.
func (r *Reconciler) cleanup(ctx context.Context, name string) error {
	return r.pruneRoleBindings(ctx, name, nil)
}

func setReady(conditions *[]v1alpha3.Condition) {
	for i := range *conditions {
		if (*conditions)[i].Type == v1alpha3.ProjectRoleBindingConditionReady {
			(*conditions)[i].Status = "True"
			(*conditions)[i].LastProbeTime = metav1.Now()
			return
		}
	}
	*conditions = append(*conditions, v1alpha3.Condition{
		Type:               v1alpha3.ProjectRoleBindingConditionReady,
		Status:             "True",
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager wires the reconciler and its watches.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueByProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		// a project change enqueues all PRBs in its main namespace
		list := &v1alpha3.ProjectRoleBindingList{}
		if err := r.List(ctx, list, client.InNamespace(obj.GetName())); err != nil {
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
		For(&v1alpha3.ProjectRoleBinding{}).
		Watches(&v1alpha3.Project{}, enqueueByProject).
		Watches(&rbacv1.RoleBinding{}, enqueueByOwnedRoleBinding).
		Named("project-role-binding").
		Complete(r)
}
