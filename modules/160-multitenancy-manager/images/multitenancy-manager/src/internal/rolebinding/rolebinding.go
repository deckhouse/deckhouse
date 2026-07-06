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

// Package rolebinding holds helpers shared by the ProjectRoleBinding and ClusterProjectRoleBinding
// reconcilers and webhooks.
package rolebinding

import (
	"context"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	// prbServicePrefix/cprbServicePrefix prefix the service RoleBindings fanned out by the
	// PRB/CPRB reconcilers.
	prbServicePrefix  = "d8:prb:"
	cprbServicePrefix = "d8:cprb:"

	// AnnotationDisabledForProjects, when "true" on a ClusterRole, forbids granting it directly via
	// a (Cluster)ProjectRoleBinding or a template-rendered binding. The role may still be aggregated
	// into a custom role.
	AnnotationDisabledForProjects = "rbac.deckhouse.io/disabled-for-direct-use-in-projects"

	// ControllerServiceAccount/DeckhouseServiceAccount are the privileged identities recognised
	// across the module (binding/project/template/protect webhooks and main wiring). They live here,
	// the lowest shared package, so the value is defined once and cannot drift between callers.
	ControllerServiceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	DeckhouseServiceAccount  = "system:serviceaccount:d8-system:deckhouse"
)

// PRBServiceName returns the name of the service RoleBinding fanned out for a ProjectRoleBinding.
func PRBServiceName(name string) string {
	return prbServicePrefix + name
}

// CPRBServiceName returns the name of the service RoleBinding fanned out for a
// ClusterProjectRoleBinding.
func CPRBServiceName(name string) string {
	return cprbServicePrefix + name
}

// ProjectNamespaceNames returns the namespaces of the project. It always includes the main
// namespace (the project name) even before the status is populated.
func ProjectNamespaceNames(project *v1alpha3.Project) []string {
	if len(project.Status.Namespaces) == 0 {
		return []string{project.Name}
	}
	names := make([]string, 0, len(project.Status.Namespaces))
	hasMain := false
	for _, ns := range project.Status.Namespaces {
		names = append(names, ns.Name)
		if ns.Name == project.Name {
			hasMain = true
		}
	}
	if !hasMain {
		names = append(names, project.Name)
	}
	return names
}

// CopySubjects deep-copies a subjects slice.
func CopySubjects(in []rbacv1.Subject) []rbacv1.Subject {
	if in == nil {
		return nil
	}
	out := make([]rbacv1.Subject, len(in))
	copy(out, in)
	return out
}

// AllowedRolePrefixes lists the ClusterRole name prefixes that may be granted via PRB/CPRB.
var AllowedRolePrefixes = []string{
	"d8:project:",
	"d8:namespace:",
	"d8:project-capability:",
	"d8:namespace-capability:",
	"d8:custom:",
}

// IsRoleAllowed reports whether a ClusterRole name may be granted via a (Cluster)ProjectRoleBinding.
func IsRoleAllowed(name string) bool {
	for _, prefix := range AllowedRolePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// UpsertParams describes a single service RoleBinding fanned out by a (Cluster)ProjectRoleBinding.
type UpsertParams struct {
	// Name is the service RoleBinding name (PRBServiceName/CPRBServiceName).
	Name string
	// Namespace is the target namespace.
	Namespace string
	// Project labels the binding with its owning project.
	Project string
	// OwnerLabel is the owned-by label key (ResourceLabelOwnedByPRB/ResourceLabelOwnedByCPRB).
	OwnerLabel string
	// OwnerName is the name of the source (Cluster)ProjectRoleBinding.
	OwnerName string
	// RelatedWith is the value of the related-with annotation linking back to the source binding.
	RelatedWith string
	// Subjects are copied into the service RoleBinding.
	Subjects []rbacv1.Subject
	// RoleRef is the granted ClusterRole name.
	RoleRef string
}

// UpsertServiceRoleBinding creates or updates the service RoleBinding described by params. roleRef is
// immutable in the Kubernetes API, so a change of role recreates the binding; the change is detected
// inside the mutate function, which avoids a redundant pre-Get on the hot fan-out path (one read in
// the steady state, an extra read only on the rare role change). setOwner, when non-nil, installs a
// controller owner reference and is only valid for a same-namespace owner — cross-namespace owner
// references are not allowed, so additional namespaces rely on label-based cleanup.
func UpsertServiceRoleBinding(ctx context.Context, c client.Client, params UpsertParams, setOwner func(*rbacv1.RoleBinding) error) error {
	apply := func(rb *rbacv1.RoleBinding) error {
		if rb.Labels == nil {
			rb.Labels = map[string]string{}
		}
		rb.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		rb.Labels[v1alpha3.ResourceLabelProject] = params.Project
		rb.Labels[params.OwnerLabel] = params.OwnerName
		if rb.Annotations == nil {
			rb.Annotations = map[string]string{}
		}
		rb.Annotations[v1alpha3.ResourceAnnotationRelatedWith] = params.RelatedWith
		rb.Subjects = CopySubjects(params.Subjects)
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     params.RoleRef,
		}
		if setOwner != nil {
			return setOwner(rb)
		}
		return nil
	}

	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: params.Name, Namespace: params.Namespace}}
	recreate := false
	if _, err := controllerutil.CreateOrUpdate(ctx, c, rb, func() error {
		// A populated, differing roleRef means the binding already exists with another (immutable)
		// role: leave it untouched here and recreate it below instead of issuing a doomed update.
		if rb.RoleRef.Name != "" && rb.RoleRef.Name != params.RoleRef {
			recreate = true
			return nil
		}
		return apply(rb)
	}); err != nil {
		return fmt.Errorf("upsert RoleBinding %s/%s: %w", params.Namespace, params.Name, err)
	}
	if !recreate {
		return nil
	}

	if err := c.Delete(ctx, rb); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("recreate RoleBinding %s/%s on roleRef change: %w", params.Namespace, params.Name, err)
	}
	fresh := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: params.Name, Namespace: params.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, fresh, func() error { return apply(fresh) }); err != nil {
		return fmt.Errorf("upsert RoleBinding %s/%s: %w", params.Namespace, params.Name, err)
	}
	return nil
}

// PruneServiceRoleBindings deletes the service RoleBindings matching selector whose namespace is not
// in keep. A nil keep deletes every match, which is what cleanup needs.
func PruneServiceRoleBindings(ctx context.Context, c client.Client, selector map[string]string, keep map[string]struct{}) error {
	list := &rbacv1.RoleBindingList{}
	if err := c.List(ctx, list, client.MatchingLabels(selector)); err != nil {
		return fmt.Errorf("list service RoleBindings: %w", err)
	}
	for i := range list.Items {
		if _, ok := keep[list.Items[i].Namespace]; ok {
			continue
		}
		if err := c.Delete(ctx, &list.Items[i]); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete stale RoleBinding %s/%s: %w", list.Items[i].Namespace, list.Items[i].Name, err)
		}
	}
	return nil
}

// NamespaceSet returns the namespaces of the project as a set, for membership tests and pruning.
func NamespaceSet(project *v1alpha3.Project) map[string]struct{} {
	names := ProjectNamespaceNames(project)
	set := make(map[string]struct{}, len(names))
	for _, ns := range names {
		set[ns] = struct{}{}
	}
	return set
}

// ProjectFanoutChanged reports whether a Project update is relevant to the PRB/CPRB fan-out: only a
// change to the project's namespace set, its virtual-project label, or its deletion state can add or
// remove a service RoleBinding. Frequent status writes (conditions, usage, observedGeneration) are
// ignored so they no longer re-enqueue every binding on every project status write.
func ProjectFanoutChanged(oldProject, newProject *v1alpha3.Project) bool {
	if oldProject.Labels[v1alpha3.ProjectLabelVirtualProject] != newProject.Labels[v1alpha3.ProjectLabelVirtualProject] {
		return true
	}
	if oldProject.DeletionTimestamp.IsZero() != newProject.DeletionTimestamp.IsZero() {
		return true
	}
	oldNames := ProjectNamespaceNames(oldProject)
	newNames := ProjectNamespaceNames(newProject)
	slices.Sort(oldNames)
	slices.Sort(newNames)
	return !slices.Equal(oldNames, newNames)
}

// ProjectFanoutPredicate filters Project watch events feeding the PRB/CPRB fan-out so that only
// changes which can add or remove a service RoleBinding (see ProjectFanoutChanged) re-enqueue the
// bindings; create and delete events always pass.
func ProjectFanoutPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldProject, ok := e.ObjectOld.(*v1alpha3.Project)
			if !ok {
				return true
			}
			newProject, ok := e.ObjectNew.(*v1alpha3.Project)
			if !ok {
				return true
			}
			return ProjectFanoutChanged(oldProject, newProject)
		},
	}
}
