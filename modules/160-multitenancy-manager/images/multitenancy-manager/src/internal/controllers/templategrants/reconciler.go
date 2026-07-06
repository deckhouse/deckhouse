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

// Package templategrants materializes the cluster-resource grant fields of a schema-based
// ProjectTemplate (deckhouse.io/v1alpha2) into managed ClusterResourceGrantPolicy objects.
//
// Contract (the behaviour the CRD promises, independent of how it is implemented):
//   - One managed policy PER SOURCE, never merged:
//   - template.spec.resources      -> "template-<tmpl>-inline"
//   - each template.spec.grantPolicies[i] (a library policy without a projectSelector)
//     -> "template-<tmpl>-<policyName>" copying its resources
//   - Every managed policy targets the projects of the template via a projectSelector on the
//     projects.deckhouse.io/project-template namespace label, so it applies to all (current and
//     future) projects of the template without per-project objects.
//   - Every managed policy is owned by the ProjectTemplate, so deleting the template garbage-collects
//     its managed policies; removing a source prunes just that policy.
package templategrants

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	grantsv1alpha1 "controller/api/v1alpha1"
	deckhousev1alpha2 "controller/apis/deckhouse.io/v1alpha2"
)

const (
	// LabelManagedByTemplate links a controller-managed ClusterResourceGrantPolicy to the
	// ProjectTemplate that produced it; the reconciler lists by it to prune stale managed policies.
	LabelManagedByTemplate = "multitenancy.deckhouse.io/managed-by-template"
	// LabelGrantSource records what produced the managed policy: the inline resources or a referenced
	// library policy.
	LabelGrantSource = "multitenancy.deckhouse.io/grant-source"

	// GrantSourceInline is the LabelGrantSource value (and managed-name suffix) of the policy
	// materialized from a template's inline resources. It is exported because it is a reserved
	// grant-policy name: a referenced library policy named "inline" would collide with the inline slot.
	GrantSourceInline = "inline"
	grantSourcePolicy = "policy"

	managedPrefix = "template-"
)

// Reconciler keeps the managed ClusterResourceGrantPolicy objects of every schema-based
// ProjectTemplate in sync with the template's resources / grantPolicies fields.
type Reconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Scheme == nil {
		r.Scheme = mgr.GetScheme()
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("template-grants").
		For(&deckhousev1alpha2.ProjectTemplate{}).
		Owns(&grantsv1alpha1.ClusterResourceGrantPolicy{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithName("template-grants")

	tmpl := &deckhousev1alpha2.ProjectTemplate{}
	if err := r.Client.Get(ctx, req.NamespacedName, tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			// Owner references garbage-collect the managed policies; nothing left to do.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get project template %s: %w", req.Name, err)
	}
	if !tmpl.DeletionTimestamp.IsZero() {
		// Managed policies are owned by the template and removed by the garbage collector.
		return reconcile.Result{}, nil
	}

	desired, requeue, err := r.desiredPolicies(ctx, tmpl)
	if err != nil {
		return reconcile.Result{}, err
	}

	for name, spec := range desired {
		if err := r.applyManaged(ctx, tmpl, name, spec); err != nil {
			return reconcile.Result{}, fmt.Errorf("apply managed grant policy %s: %w", name, err)
		}
	}
	if err := r.pruneStale(ctx, tmpl.Name, desired); err != nil {
		return reconcile.Result{}, fmt.Errorf("prune stale managed grant policies: %w", err)
	}

	if requeue {
		// A referenced library policy is not present yet (the template webhook normally rejects this,
		// but a create-after-template race is possible). Retry until every reference resolves.
		log.V(1).Info("requeue: a referenced grant policy is missing", "template", tmpl.Name)
		return reconcile.Result{Requeue: true}, nil
	}
	log.V(1).Info("template grants materialized", "template", tmpl.Name, "managedPolicies", len(desired))
	return reconcile.Result{}, nil
}

// managedSpec is the resolved content of one managed policy.
type managedSpec struct {
	resources []grantsv1alpha1.GrantResource
	source    string
}

// desiredPolicies computes the managed policies a template should own. The bool return reports
// whether at least one referenced library policy was missing (caller requeues).
func (r *Reconciler) desiredPolicies(ctx context.Context, tmpl *deckhousev1alpha2.ProjectTemplate) (map[string]managedSpec, bool, error) {
	desired := make(map[string]managedSpec, 1+len(tmpl.Spec.GrantPolicies))

	if len(tmpl.Spec.Resources) > 0 {
		desired[InlineName(tmpl.Name)] = managedSpec{resources: tmpl.Spec.Resources, source: GrantSourceInline}
	}

	requeue := false
	for _, policy := range tmpl.Spec.GrantPolicies {
		lib := &grantsv1alpha1.ClusterResourceGrantPolicy{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: policy}, lib); err != nil {
			if apierrors.IsNotFound(err) {
				requeue = true
				continue
			}
			return nil, false, fmt.Errorf("get grant policy %s: %w", policy, err)
		}
		desired[PolicyName(tmpl.Name, policy)] = managedSpec{resources: lib.Spec.Resources, source: grantSourcePolicy}
	}
	return desired, requeue, nil
}

func (r *Reconciler) applyManaged(ctx context.Context, tmpl *deckhousev1alpha2.ProjectTemplate, name string, spec managedSpec) error {
	managed := &grantsv1alpha1.ClusterResourceGrantPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, managed, func() error {
		if managed.Labels == nil {
			managed.Labels = make(map[string]string, 2)
		}
		managed.Labels[LabelManagedByTemplate] = tmpl.Name
		managed.Labels[LabelGrantSource] = spec.source
		managed.Spec.ProjectSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{deckhousev1alpha2.ResourceLabelTemplate: tmpl.Name},
		}
		managed.Spec.Resources = deepCopyResources(spec.resources)
		return controllerutil.SetControllerReference(tmpl, managed, r.Scheme)
	})
	return err
}

func (r *Reconciler) pruneStale(ctx context.Context, tmplName string, desired map[string]managedSpec) error {
	list := &grantsv1alpha1.ClusterResourceGrantPolicyList{}
	if err := r.Client.List(ctx, list, client.MatchingLabels{LabelManagedByTemplate: tmplName}); err != nil {
		return fmt.Errorf("list managed grant policies: %w", err)
	}
	for i := range list.Items {
		item := &list.Items[i]
		if _, keep := desired[item.Name]; keep {
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

// InlineName is the managed policy name for a template's inline resources.
func InlineName(tmpl string) string { return managedPrefix + tmpl + "-" + GrantSourceInline }

// PolicyName is the managed policy name for a template's reference to a library grant policy.
func PolicyName(tmpl, policy string) string { return managedPrefix + tmpl + "-" + policy }

// ManagedNames returns the names of the managed ClusterResourceGrantPolicy objects a template owns:
// the inline slot (only when it declares inline resources) and one per referenced grant policy. It is
// the single source of truth for managed naming, shared by the reconciler and the admission webhook
// (which uses it to reject templates whose names would collide).
func ManagedNames(tmpl *deckhousev1alpha2.ProjectTemplate) []string {
	names := make([]string, 0, 1+len(tmpl.Spec.GrantPolicies))
	if len(tmpl.Spec.Resources) > 0 {
		names = append(names, InlineName(tmpl.Name))
	}
	for _, policy := range tmpl.Spec.GrantPolicies {
		names = append(names, PolicyName(tmpl.Name, policy))
	}
	return names
}
