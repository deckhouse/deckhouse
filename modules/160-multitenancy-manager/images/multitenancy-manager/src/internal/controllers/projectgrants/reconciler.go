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

// Package projectgrants materializes spec.grants of a Project into managed ClusterResourceGrantPolicy
// objects, the way templategrants does it for a ProjectTemplate:
//   - spec.grants.resources -> "project-<project>-inline";
//   - each spec.grants.grantPolicies[i], a library policy without a selector, ->
//     "project-<project>-<policy>", copying its resources.
//
// Every managed policy selects the project by the projects.deckhouse.io/project label of the Project,
// which the module stamps and keeps, so it applies to every namespace of the project, the ones created
// later included. It is owned
// by the Project, so deleting the project garbage-collects it, and removing a source prunes just that
// policy. The grants of the project add to the grants of its template.
package projectgrants

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/templategrants"
)

const (
	// LabelManagedByProject links a managed ClusterResourceGrantPolicy to the Project that produced
	// it; the reconciler lists by it to prune stale policies, and the admission policy of the module
	// keeps such policies to the controller.
	LabelManagedByProject = "multitenancy.deckhouse.io/managed-by-project"

	// ManagedPrefix starts the name of every managed policy of a project. The admission policy of the
	// module keeps the names with it to the controller.
	ManagedPrefix = "project-"

	grantSourcePolicy = "policy"

	// Reasons of the Warning events on a Project.
	reasonGrantNotApplied = "GrantPolicyNotApplied"
	reasonGrantMissing    = "GrantPolicyMissing"
)

// Reconciler keeps the managed ClusterResourceGrantPolicy objects of every Project in sync with its
// spec.grants.
type Reconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	// Recorder reports on the Project a grant source that cannot be applied and a reference to a
	// library policy that does not exist. Optional.
	Recorder record.EventRecorder
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Scheme == nil {
		r.Scheme = mgr.GetScheme()
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("project-grant-policies").
		For(&v1alpha3.Project{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&grantsv1alpha2.ClusterResourceGrantPolicy{}).
		// A library policy has no owner reference to the projects that copy it, so Owns does not see
		// it; its status says nothing the copies take.
		Watches(&grantsv1alpha2.ClusterResourceGrantPolicy{}, handler.EnqueueRequestsFromMapFunc(r.projectsReferencing),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// projectsReferencing maps a library ClusterResourceGrantPolicy to the projects that list it in
// spec.grants.grantPolicies, so an edit or a deletion of the library policy reaches its copies.
func (r *Reconciler) projectsReferencing(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if _, managed := labels[LabelManagedByProject]; managed {
		return nil
	}
	if _, managed := labels[templategrants.LabelManagedByTemplate]; managed {
		return nil
	}
	projects := &v1alpha3.ProjectList{}
	if err := r.Client.List(ctx, projects); err != nil {
		ctrllog.FromContext(ctx).Error(err, "list projects referencing a grant policy", "policy", obj.GetName())
		return nil
	}
	var requests []reconcile.Request
	for i := range projects.Items {
		if grants := projects.Items[i].Spec.Grants; grants != nil && slices.Contains(grants.GrantPolicies, obj.GetName()) {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: projects.Items[i].Name}})
		}
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	project := &v1alpha3.Project{}
	if err := r.Client.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			// owner references garbage-collect the managed policies
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get project %s: %w", req.Name, err)
	}
	if !project.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// a library policy that cannot be read keeps its managed policy as it is, neither rewritten nor
	// pruned, and the rest of the sources go on
	desired, missing, unread, readErr := r.desiredPolicies(ctx, project)
	for _, policy := range missing {
		// A library policy that is created later enqueues the projects that list it, so there is
		// nothing to poll for.
		r.warn(project, reasonGrantMissing, "the ClusterResourceGrantPolicy %q listed in spec.grants.grantPolicies does not exist, "+
			"so its resources are not granted until it is created", policy)
	}

	// A source that cannot be applied does not hold back the others, and above all not the pruning:
	// a revoked grant must not stay in force because an unrelated source is broken.
	var errs []error
	if readErr != nil {
		errs = append(errs, readErr)
	}
	for _, name := range slices.Sorted(maps.Keys(desired)) {
		if err := r.applyManaged(ctx, project, name, desired[name]); err != nil {
			r.warn(project, reasonGrantNotApplied, "%s cannot be applied as the %s ClusterResourceGrantPolicy: %v", desired[name].ref, name, err)
			errs = append(errs, fmt.Errorf("apply managed grant policy %s: %w", name, err))
		}
	}
	if err := r.pruneStale(ctx, project.Name, desired, unread); err != nil {
		errs = append(errs, fmt.Errorf("prune stale managed grant policies: %w", err))
	}
	return reconcile.Result{}, errors.Join(errs...)
}

// warn records a Warning event on the project.
func (r *Reconciler) warn(project *v1alpha3.Project, reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, reason, format, args...)
	}
}

// managedSpec is the resolved content of one managed policy.
type managedSpec struct {
	resources []grantsv1alpha1.GrantResource
	source    string
	// ref names the field of the project the policy comes from, for the events.
	ref string
}

// desiredPolicies computes the managed policies of a project. The slice holds the referenced library
// policies that do not exist, and the set the managed names of the ones that could not be read, which
// the error says why.
func (r *Reconciler) desiredPolicies(ctx context.Context, project *v1alpha3.Project) (map[string]managedSpec, []string, map[string]bool, error) {
	grants := project.Spec.Grants
	if grants.IsEmpty() {
		return nil, nil, nil, nil
	}
	desired := make(map[string]managedSpec, 1+len(grants.GrantPolicies))
	if len(grants.Resources) > 0 {
		desired[InlineName(project.Name)] = managedSpec{
			resources: grants.Resources, source: templategrants.GrantSourceInline, ref: "spec.grants.resources",
		}
	}

	var missing []string
	unread := map[string]bool{}
	var errs []error
	for _, policy := range grants.GrantPolicies {
		lib := &grantsv1alpha2.ClusterResourceGrantPolicy{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: policy}, lib); err != nil {
			if apierrors.IsNotFound(err) {
				missing = append(missing, policy)
				continue
			}
			unread[PolicyName(project.Name, policy)] = true
			errs = append(errs, fmt.Errorf("get grant policy %s: %w", policy, err))
			continue
		}
		desired[PolicyName(project.Name, policy)] = managedSpec{
			resources: lib.Spec.Resources, source: grantSourcePolicy, ref: fmt.Sprintf("spec.grants.grantPolicies entry %q", policy),
		}
	}
	return desired, missing, unread, errors.Join(errs...)
}

// applyManaged creates or updates one managed policy. A policy of that name that is not the
// project's is left alone: the Project webhook refuses such a name, and taking it over would change
// the grants of whoever made it.
func (r *Reconciler) applyManaged(ctx context.Context, project *v1alpha3.Project, name string, spec managedSpec) error {
	managed := &grantsv1alpha2.ClusterResourceGrantPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
	switch err := r.Client.Get(ctx, client.ObjectKeyFromObject(managed), managed); {
	case apierrors.IsNotFound(err):
	case err != nil:
		return fmt.Errorf("get: %w", err)
	case managed.Labels[LabelManagedByProject] != project.Name:
		return fmt.Errorf("the %s ClusterResourceGrantPolicy exists and is not managed for the %s project", name, project.Name)
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, managed, func() error {
		if managed.Labels == nil {
			managed.Labels = make(map[string]string, 2)
		}
		managed.Labels[LabelManagedByProject] = project.Name
		managed.Labels[templategrants.LabelGrantSource] = spec.source
		managed.Spec.ProjectSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{v1alpha3.ResourceLabelProject: project.Name},
		}
		managed.Spec.NamespaceSelector = nil
		managed.Spec.Resources = deepCopyResources(spec.resources)
		return controllerutil.SetControllerReference(project, managed, r.Scheme)
	})
	return err
}

// pruneStale deletes the managed policies of the project that are neither desired nor kept.
func (r *Reconciler) pruneStale(ctx context.Context, project string, desired map[string]managedSpec, keep map[string]bool) error {
	list := &grantsv1alpha2.ClusterResourceGrantPolicyList{}
	if err := r.Client.List(ctx, list, client.MatchingLabels{LabelManagedByProject: project}); err != nil {
		return fmt.Errorf("list managed grant policies: %w", err)
	}
	for i := range list.Items {
		item := &list.Items[i]
		if _, wanted := desired[item.Name]; wanted || keep[item.Name] {
			continue
		}
		if err := r.Client.Delete(ctx, item); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale grant policy %s: %w", item.Name, err)
		}
	}
	return nil
}

func deepCopyResources(in []grantsv1alpha1.GrantResource) []grantsv1alpha1.GrantResource {
	if in == nil {
		return nil
	}
	out := make([]grantsv1alpha1.GrantResource, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

// InlineName is the managed policy of the inline resources of a project.
func InlineName(project string) string {
	return ManagedPrefix + project + "-" + templategrants.GrantSourceInline
}

// PolicyName is the managed policy of the reference of a project to a library grant policy.
func PolicyName(project, policy string) string { return ManagedPrefix + project + "-" + policy }

// ManagedNames returns the names of the managed policies a project owns, shared by the reconciler and
// the Project webhook, which refuses grants whose names collide.
func ManagedNames(project *v1alpha3.Project) []string {
	grants := project.Spec.Grants
	if grants.IsEmpty() {
		return nil
	}
	names := make([]string, 0, 1+len(grants.GrantPolicies))
	if len(grants.Resources) > 0 {
		names = append(names, InlineName(project.Name))
	}
	for _, policy := range grants.GrantPolicies {
		names = append(names, PolicyName(project.Name, policy))
	}
	return names
}
