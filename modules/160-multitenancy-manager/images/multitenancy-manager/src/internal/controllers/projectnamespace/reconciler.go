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

// Package projectnamespace reconciles ProjectNamespace objects by creating and owning an additional
// namespace "<project>-<spec.name>" for the project. The namespace carries the project ownership
// labels, so the project controller picks it up into Project.status.namespaces (kind Additional) and
// the PRB/CPRB reconcilers fan their service RoleBindings into it. Namespaced template objects are
// rendered into every project namespace, so the additional namespace also receives them.
package projectnamespace

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Reconciler owns the additional namespace of a ProjectNamespace.
type Reconciler struct {
	client.Client
}

// Reconcile keeps the additional namespace of a single ProjectNamespace in sync with its object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("projectnamespace", req.NamespacedName.String())

	pns := &v1alpha3.ProjectNamespace{}
	if err := r.Get(ctx, req.NamespacedName, pns); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ProjectNamespace: %w", err)
	}

	// The project's main namespace equals the project name, which equals the object's namespace.
	project := &v1alpha3.Project{}
	projectFound := true
	if err := r.Get(ctx, types.NamespacedName{Name: req.Namespace}, project); err != nil {
		if !k8serrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("get project: %w", err)
		}
		projectFound = false
	}

	resulting := r.namespaceName(pns)

	// Cleanup path: the object is being deleted, or its project is gone/terminating.
	if !pns.DeletionTimestamp.IsZero() || !projectFound || !project.DeletionTimestamp.IsZero() {
		if err := r.deleteNamespace(ctx, resulting, req.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.ContainsFinalizer(pns, v1alpha3.ProjectNamespaceFinalizer) {
			controllerutil.RemoveFinalizer(pns, v1alpha3.ProjectNamespaceFinalizer)
			if err := r.Update(ctx, pns); err != nil && !k8serrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(pns, v1alpha3.ProjectNamespaceFinalizer) {
		controllerutil.AddFinalizer(pns, v1alpha3.ProjectNamespaceFinalizer)
		if err := r.Update(ctx, pns); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	if err := r.ensureNamespace(ctx, pns, req.Namespace); err != nil {
		if v1alpha3.SetCondition(&pns.Status.Conditions, v1alpha3.ProjectNamespaceConditionReady, corev1.ConditionFalse, err.Error()) {
			if statusErr := r.Status().Update(ctx, pns); statusErr != nil {
				return ctrl.Result{}, fmt.Errorf("update status: %w", statusErr)
			}
		}
		return ctrl.Result{}, err
	}

	// Write status only when it actually changed: an unconditional write would bump the condition
	// timestamps and re-enqueue this object through the For() watch, causing a reconcile hot-loop.
	changed := false
	if pns.Status.Namespace != resulting {
		pns.Status.Namespace = resulting
		changed = true
	}
	if pns.Status.ObservedGeneration != pns.Generation {
		pns.Status.ObservedGeneration = pns.Generation
		changed = true
	}
	if v1alpha3.SetCondition(&pns.Status.Conditions, v1alpha3.ProjectNamespaceConditionReady, corev1.ConditionTrue, "") {
		changed = true
	}
	if changed {
		if err := r.Status().Update(ctx, pns); err != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", err)
		}
	}

	log.Info("the project namespace reconciled", "namespace", resulting)
	return ctrl.Result{}, nil
}

// namespaceName returns the name of the namespace the ProjectNamespace claims.
func (r *Reconciler) namespaceName(pns *v1alpha3.ProjectNamespace) string {
	return pns.Namespace + "-" + pns.Spec.Name
}

// inheritedNamespaceLabels are policy/grant labels an additional namespace inherits from the project's
// main namespace, so that features (monitoring, vulnerability scanning), Pod Security Standard and
// cluster resource grants (managed ClusterResourceGrantPolicy selects by project-template) apply in
// EVERY namespace of the project, not just the main one. The main namespace is the source of truth:
// these labels are rendered there from the ProjectTemplate (with fromParam already resolved).
var inheritedNamespaceLabels = []string{
	"security.deckhouse.io/pod-policy",
	"extended-monitoring.deckhouse.io/enabled",
	"security-scanning.deckhouse.io/enabled",
	v1alpha3.ResourceLabelTemplate,
}

// ensureNamespace creates or updates the additional namespace, stamping the project ownership
// labels and inheriting the project's policy/grant labels from the main namespace. It refuses to
// adopt a pre-existing namespace that belongs to a different project.
func (r *Reconciler) ensureNamespace(ctx context.Context, pns *v1alpha3.ProjectNamespace, project string) error {
	name := r.namespaceName(pns)

	existing := &corev1.Namespace{}
	switch err := r.Get(ctx, types.NamespacedName{Name: name}, existing); {
	case err == nil:
		if owner := existing.Labels[v1alpha3.ResourceLabelProject]; owner != "" && owner != project {
			return fmt.Errorf("namespace %q already exists and is owned by project %q", name, owner)
		}
	case !k8serrors.IsNotFound(err):
		return fmt.Errorf("get namespace %q: %w", name, err)
	}

	// Главный namespace проекта носит имя проекта; читаем его лейблы, чтобы унаследовать политики/гранты.
	mainLabels := map[string]string{}
	main := &corev1.Namespace{}
	switch err := r.Get(ctx, types.NamespacedName{Name: project}, main); {
	case err == nil:
		mainLabels = main.Labels
	case !k8serrors.IsNotFound(err):
		return fmt.Errorf("get main namespace %q: %w", project, err)
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		ns.Labels[v1alpha3.ResourceLabelProject] = project
		ns.Labels[v1alpha3.ResourceLabelProjectNamespace] = pns.Name
		// Наследуем policy/grant-лейблы главного namespace; отсутствующие — снимаем, чтобы доп. namespace
		// оставался синхронным (например, при выключении фичи в шаблоне).
		for _, key := range inheritedNamespaceLabels {
			if value, ok := mainLabels[key]; ok {
				ns.Labels[key] = value
			} else {
				delete(ns.Labels, key)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure namespace %q: %w", name, err)
	}
	return nil
}

// deleteNamespace removes the additional namespace, but only when it is still owned by this project.
func (r *Reconciler) deleteNamespace(ctx context.Context, name, project string) error {
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get namespace %q: %w", name, err)
	}
	if ns.Labels[v1alpha3.ResourceLabelProject] != project {
		return nil
	}
	if err := r.Delete(ctx, ns); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete namespace %q: %w", name, err)
	}
	return nil
}

// SetupWithManager wires the reconciler and its watches.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueByProject := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		// a project change enqueues all ProjectNamespaces in its main namespace
		list := &v1alpha3.ProjectNamespaceList{}
		if err := r.List(ctx, list, client.InNamespace(obj.GetName())); err != nil {
			ctrllog.FromContext(ctx).Error(err, "list ProjectNamespaces for project watch", "project", obj.GetName())
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: list.Items[i].Namespace, Name: list.Items[i].Name}})
		}
		return reqs
	})

	// A namespace change re-enqueues ProjectNamespaces. An owned (additional) namespace — labelled with
	// both the project and the project-namespace — maps to its own ProjectNamespace. The project's MAIN
	// namespace carries the project label but NO project-namespace label; it is the source of the
	// inherited policy/grant labels (Pod Security Standard, monitoring/scanning, grant-template), so a
	// change to it must re-sync EVERY ProjectNamespace of the project. Without this, inherited labels
	// drift on additional namespaces whenever they change post-creation (e.g. PSS flipped from Baseline
	// to Privileged, or a feature toggled), because the ProjectNamespace is otherwise only reconciled on
	// its own spec change or on a change to the project's namespace-name set.
	enqueueByProjectNamespace := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		project, ok := obj.GetLabels()[v1alpha3.ResourceLabelProject]
		if !ok {
			return nil
		}
		if name, ok := obj.GetLabels()[v1alpha3.ResourceLabelProjectNamespace]; ok {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: project, Name: name}}}
		}
		list := &v1alpha3.ProjectNamespaceList{}
		if err := r.List(ctx, list, client.InNamespace(project)); err != nil {
			ctrllog.FromContext(ctx).Error(err, "list ProjectNamespaces for main-namespace drift", "project", project)
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: list.Items[i].Namespace, Name: list.Items[i].Name}})
		}
		return reqs
	})

	return ctrl.NewControllerManagedBy(mgr).
		// Only spec changes (generation bumps) re-enqueue the ProjectNamespace itself; status writes
		// must not, or the reconcile loops on its own writes. The owned-namespace watch still catches
		// external drift of the created namespace.
		For(&v1alpha3.ProjectNamespace{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha3.Project{}, enqueueByProject, builder.WithPredicates(rolebinding.ProjectFanoutPredicate())).
		Watches(&corev1.Namespace{}, enqueueByProjectNamespace).
		Named("project-namespace").
		Complete(r)
}
