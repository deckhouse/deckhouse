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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

const (
	// subjectNamespaceIndex indexes a ProjectRoleBinding by the namespaces other than its own whose
	// ServiceAccounts its subjects name (see subjectNamespaces).
	subjectNamespaceIndex = ".spec.subjects.serviceAccountNamespaces"

	// reasonSubjectsLeftOut is the reason of the Warning event sent when the set of subjects the
	// fan-out leaves out changes.
	reasonSubjectsLeftOut = "SubjectsLeftOut"
)

// Reconciler fans out service RoleBindings for ProjectRoleBinding objects.
type Reconciler struct {
	client.Client

	// Recorder reports the subjects the fan-out leaves out on the ProjectRoleBinding. Optional.
	Recorder record.EventRecorder
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

	// A ServiceAccount gets the rights only while its namespace is a namespace of the project, whatever
	// the webhook saw when the subject was added. The spec is left as it is, and a subject comes back
	// into the RoleBindings once its namespace is a namespace of the project again. The administrators
	// binding the controller writes carries the administrators the project names, whatever namespace
	// their ServiceAccounts are in.
	var administrators []v1alpha3.Administrator
	if rolebinding.IsAdministratorsBinding(prb.Name, prb.Labels[v1alpha3.ResourceLabelManagedBy]) {
		administrators = project.Spec.Administrators
	}
	subjects, leftOut, err := rolebinding.SplitProjectSubjects(ctx, r.Client, prb.Namespace, prb.Spec.Subjects, administrators)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fan out into every namespace, accumulating per-namespace errors so a single bad namespace
	// does not block the rest of the project (relevant at scale). With every subject left out the
	// RoleBindings stay, without subjects, so they grant nothing and need no other handling.
	var errs []error
	for _, ns := range target {
		if err := r.upsertRoleBinding(ctx, prb, ns, related, subjects); err != nil {
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
	bound, leftOutMessage := corev1.ConditionTrue, ""
	if len(leftOut) > 0 {
		bound, leftOutMessage = corev1.ConditionFalse, subjectsLeftOutMessage(leftOut)
	}
	leftOutChanged := v1alpha3.SetCondition(&prb.Status.Conditions, v1alpha3.ProjectRoleBindingConditionSubjectsBound, bound, leftOutMessage)
	if leftOutChanged {
		changed = true
	}
	if changed {
		if err := r.Status().Update(ctx, prb); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
	}

	// The event goes out once for each set of left-out subjects, after the status that records the set
	// is written, so a restart or a new leader does not send it again.
	if leftOutChanged && len(leftOut) > 0 && r.Recorder != nil {
		r.Recorder.Event(prb, corev1.EventTypeWarning, reasonSubjectsLeftOut, leftOutMessage)
	}

	log.Info("the project role binding reconciled", "namespaces", len(target), "subjectsLeftOut", len(leftOut))
	return ctrl.Result{}, nil
}

// subjectsLeftOutMessage lists the subjects the fan-out leaves out, sorted, so that the message and
// with it the condition change only when the set does. It does not say which project a namespace
// belongs to now, as the readers of this binding may not be allowed to see that project.
func subjectsLeftOutMessage(leftOut []rbacv1.Subject) string {
	names := make([]string, 0, len(leftOut))
	for _, s := range leftOut {
		if s.Kind == rbacv1.ServiceAccountKind {
			names = append(names, fmt.Sprintf("%s %s/%s", s.Kind, s.Namespace, s.Name))
		} else {
			names = append(names, s.Kind+" "+s.Name)
		}
	}
	slices.Sort(names)
	return "The subjects are left out of the RoleBindings because their ServiceAccounts are not in a namespace of the project: " +
		strings.Join(slices.Compact(names), ", ")
}

func (r *Reconciler) upsertRoleBinding(ctx context.Context, prb *v1alpha3.ProjectRoleBinding, ns, related string, subjects []rbacv1.Subject) error {
	// The main-namespace RoleBinding is owned by the PRB (same namespace); cross-namespace
	// ownerReferences are not allowed, so additional namespaces rely on label-based cleanup.
	var setOwner func(*rbacv1.RoleBinding) error
	if ns == prb.Namespace {
		setOwner = func(rb *rbacv1.RoleBinding) error {
			return controllerutil.SetControllerReference(prb, rb, r.Scheme())
		}
	}
	_, err := rolebinding.UpsertServiceRoleBinding(ctx, r.Client, rolebinding.UpsertParams{
		Name:        rolebinding.PRBServiceName(prb.Name),
		Namespace:   ns,
		Project:     prb.Namespace,
		OwnerLabel:  v1alpha3.ResourceLabelOwnedByPRB,
		OwnerName:   prb.Name,
		RelatedWith: related,
		Subjects:    subjects,
		RoleRef:     prb.Spec.RoleRef.Name,
	}, setOwner)
	return err
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

	// A namespace can come and go without a change to the binding or to its project, when a
	// ProjectNamespace is deleted or a namespace under the freed name is made for another project. The
	// index finds the bindings whose subjects name the ServiceAccounts of a namespace, so the change
	// reaches them. The setup has no context of its own, and the indexer uses it only to look up the
	// informer.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha3.ProjectRoleBinding{}, subjectNamespaceIndex, subjectNamespaces); err != nil {
		return fmt.Errorf("index ProjectRoleBindings by the namespaces of their subjects: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		// Only spec changes (generation bumps) re-enqueue the PRB itself; status writes must not,
		// or the reconcile loops on its own writes. The owned-RoleBinding watch below still catches
		// external drift of the fanned-out bindings.
		For(&v1alpha3.ProjectRoleBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha3.Project{}, enqueueByProject, builder.WithPredicates(rolebinding.ProjectFanoutPredicate())).
		Watches(&rbacv1.RoleBinding{}, enqueueByOwnedRoleBinding).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(r.requestsForNamespace), builder.WithPredicates(membershipMayChange())).
		Named("project-role-binding").
		Complete(r)
}

// subjectNamespaces is the subjectNamespaceIndex of a ProjectRoleBinding. Its own namespace, the main
// namespace of the project, is left out, as its ServiceAccounts always get the rights.
func subjectNamespaces(obj client.Object) []string {
	prb, ok := obj.(*v1alpha3.ProjectRoleBinding)
	if !ok {
		return nil
	}
	return rolebinding.ServiceAccountNamespaces(prb.Spec.Subjects, prb.Namespace)
}

// requestsForNamespace enqueues the ProjectRoleBindings whose subjects name the ServiceAccounts of the
// namespace.
func (r *Reconciler) requestsForNamespace(ctx context.Context, obj client.Object) []reconcile.Request {
	list := &v1alpha3.ProjectRoleBindingList{}
	if err := r.List(ctx, list, client.MatchingFields{subjectNamespaceIndex: obj.GetName()}); err != nil {
		ctrllog.FromContext(ctx).Error(err, "list ProjectRoleBindings for namespace watch", "namespace", obj.GetName())
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: list.Items[i].Namespace, Name: list.Items[i].Name}})
	}
	return reqs
}

// membershipMayChange lets through the namespace events that can change whether the ServiceAccounts
// of the namespace get the rights of a binding. The namespace appears or is gone, its project label
// changes, or its deletion starts. Create and delete events always pass.
func membershipMayChange() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			label := v1alpha3.ResourceLabelProject
			if e.ObjectOld.GetLabels()[label] != e.ObjectNew.GetLabels()[label] {
				return true
			}
			return e.ObjectOld.GetDeletionTimestamp().IsZero() && !e.ObjectNew.GetDeletionTimestamp().IsZero()
		},
	}
}
