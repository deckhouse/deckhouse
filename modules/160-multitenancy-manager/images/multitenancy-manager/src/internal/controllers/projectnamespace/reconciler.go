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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
	"controller/internal/rolebinding"
)

// Reconciler owns the additional namespace of a ProjectNamespace.
type Reconciler struct {
	client.Client

	// Recorder reports a restored template label on the ProjectNamespace. Optional.
	Recorder record.EventRecorder
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

// IsInheritedFromProject reports whether every new additional namespace of a project takes the label
// from the project or from its main namespace, whatever the template lists: the ownership labels and
// the fixed inherited ones of NewNamespaceLabels. Such a label cannot tell a new namespace of a
// project from the existing ones.
func IsInheritedFromProject(key string) bool {
	return key == v1alpha3.ResourceLabelHeritage || key == v1alpha3.ResourceLabelProject || slices.Contains(inheritedNamespaceLabels, key)
}

// errMainNamespaceNotRendered stops the reconcile of an additional namespace until the renderer has
// written the template labels on the main namespace: a namespace created without them could fall
// outside a ClusterResourceGrantPolicy that restricts by one of them.
var errMainNamespaceNotRendered = errors.New("the main namespace of the project is not rendered yet")

// templateLabelsOf returns the labels listed in the template-labels annotation of a namespace, without
// the keys the module owns or GitOps tools use for tracking, which are never inherited.
func templateLabelsOf(ns *corev1.Namespace) (map[string]string, error) {
	raw, ok := ns.Annotations[naming.TemplateLabelsAnnotation]
	if !ok {
		return nil, fmt.Errorf("the %s annotation is missing", naming.TemplateLabelsAnnotation)
	}
	labels := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &labels); err != nil {
		return nil, fmt.Errorf("parse the %s annotation: %w", naming.TemplateLabelsAnnotation, err)
	}
	for key := range labels {
		if naming.IsModuleOwnedLabel(key) || naming.IsGitOpsTrackingLabel(key) {
			delete(labels, key)
		}
	}
	return labels, nil
}

// NewNamespaceLabels returns the labels an additional namespace of the project gets when it is created:
// the labels of the project template the main namespace lists, the policy and grant labels inherited
// from the main namespace, and the ownership labels. The labels derived from the name of the
// ProjectNamespace, which the project user chooses, are not part of it. Like ensureNamespace, it fails
// while the main namespace is not rendered.
func NewNamespaceLabels(main *corev1.Namespace, project string) (map[string]string, error) {
	labels, err := templateLabelsOf(main)
	if err != nil {
		return nil, fmt.Errorf("%w: namespace %q: %w", errMainNamespaceNotRendered, project, err)
	}
	for _, key := range inheritedNamespaceLabels {
		if value, ok := main.Labels[key]; ok {
			labels[key] = value
		}
	}
	labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	labels[v1alpha3.ResourceLabelProject] = project
	return labels, nil
}

// ensureNamespace creates or updates the additional namespace: the labels of the project template
// (listed on the main namespace by the renderer), the fixed inherited policy/grant labels, and the
// project ownership labels, in that order, so the module's keys always win.
//
// The template owns the labels it lists: a hand edit of one on the additional namespace is put back
// with a Warning event, and a key the template stops listing is removed. The additional namespace
// records what it took in its own template-labels annotation; labels outside that record belong to
// whoever set them and are left alone. It refuses to adopt a pre-existing namespace that belongs to a
// different project.
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

	// The main namespace of a project is named after the project. Without it, or before the renderer
	// has written its template labels, nothing is created or changed (fail-static).
	main := &corev1.Namespace{}
	switch err := r.Get(ctx, types.NamespacedName{Name: project}, main); {
	case k8serrors.IsNotFound(err):
		return fmt.Errorf("%w: namespace %q does not exist", errMainNamespaceNotRendered, project)
	case err != nil:
		return fmt.Errorf("get main namespace %q: %w", project, err)
	}
	desired, err := templateLabelsOf(main)
	if err != nil {
		return fmt.Errorf("%w: namespace %q: %w", errMainNamespaceNotRendered, project, err)
	}

	record, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("marshal template labels: %w", err)
	}

	var reverted []string
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}

		// What this controller set last time, read from the object being updated, so the record and
		// the labels come from the same read; an unreadable record is treated as empty, so at worst a
		// stale key stays until the administrator removes it.
		previous, _ := templateLabelsOf(ns)
		reverted = reverted[:0]

		for key := range previous {
			if _, still := desired[key]; !still {
				delete(ns.Labels, key)
			}
		}
		for key, value := range desired {
			// A value that differs from what this controller set last time was changed by hand (or
			// was there before the template took the key); a value equal to the last one is simply
			// the template moving on.
			current, set := ns.Labels[key]
			last, owned := previous[key]
			switch {
			case set && current != value && (!owned || current != last):
				reverted = append(reverted, fmt.Sprintf("%s=%q (was %q)", key, value, current))
			case !set && owned:
				reverted = append(reverted, fmt.Sprintf("%s=%q (was removed)", key, value))
			}
			ns.Labels[key] = value
		}
		ns.Annotations[naming.TemplateLabelsAnnotation] = string(record)

		// Inherit the policy and grant labels of the main namespace, and drop the ones it no longer
		// carries: an additional namespace that keeps a label the project has switched off in its
		// template would quietly diverge from the rest of the project.
		for _, key := range inheritedNamespaceLabels {
			if value, ok := main.Labels[key]; ok {
				ns.Labels[key] = value
			} else {
				delete(ns.Labels, key)
			}
		}

		ns.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		ns.Labels[v1alpha3.ResourceLabelProject] = project
		ns.Labels[v1alpha3.ResourceLabelProjectNamespace] = pns.Name
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure namespace %q: %w", name, err)
	}

	if len(reverted) > 0 && r.Recorder != nil {
		slices.Sort(reverted)
		r.Recorder.Eventf(pns, corev1.EventTypeWarning, "TemplateLabelsRestored",
			"namespace %s: labels of the project template restored: %s; change them through the Project or its ProjectTemplate",
			name, strings.Join(reverted, ", "))
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
