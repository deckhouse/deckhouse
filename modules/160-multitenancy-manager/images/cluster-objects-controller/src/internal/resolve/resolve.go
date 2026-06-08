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

// Package resolve turns the cluster-scoped grant model (ClusterGrantableResource + ClusterObjectGrant)
// into per-project decisions: which grants apply to a namespace, and the resolved allow/deny/excluded
// name sets and default for a registration. Selector-based rules are expanded against the live granted
// objects so the webhook and reconciler share identical semantics.
package resolve

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/naming"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProjectName returns the project a namespace belongs to: the projects.deckhouse.io/project label
// value, falling back to the namespace name (single-namespace projects).
func ProjectName(ns *corev1.Namespace) string {
	if p := ns.Labels[naming.ProjectLabel]; p != "" {
		return p
	}
	return ns.Name
}

// ApplicableGrants returns every ClusterObjectGrant whose projectSelector matches the labels of the
// given namespace. A nil selector matches nothing; an invalid selector is skipped.
func ApplicableGrants(ctx context.Context, cl client.Client, namespace string) ([]*v1alpha1.ClusterObjectGrant, error) {
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get namespace %s: %w", namespace, err)
	}
	return GrantsForLabels(ctx, cl, ns.Labels)
}

// GrantsForLabels returns every ClusterObjectGrant whose projectSelector matches the given labels.
func GrantsForLabels(ctx context.Context, cl client.Client, nsLabels map[string]string) ([]*v1alpha1.ClusterObjectGrant, error) {
	grantList := &v1alpha1.ClusterObjectGrantList{}
	if err := cl.List(ctx, grantList); err != nil {
		return nil, fmt.Errorf("list ClusterObjectGrants: %w", err)
	}
	set := labels.Set(nsLabels)
	out := make([]*v1alpha1.ClusterObjectGrant, 0)
	for i := range grantList.Items {
		g := &grantList.Items[i]
		if g.Spec.ProjectSelector == nil {
			continue
		}
		sel, err := metav1.LabelSelectorAsSelector(g.Spec.ProjectSelector)
		if err != nil {
			continue
		}
		if sel.Matches(set) {
			out = append(out, g)
		}
	}
	return out, nil
}

// EntriesFor collects the grant resource entries referencing the given registration name across all
// the supplied grants.
func EntriesFor(grants []*v1alpha1.ClusterObjectGrant, resourceRef string) []v1alpha1.GrantResource {
	out := make([]v1alpha1.GrantResource, 0)
	for _, g := range grants {
		for i := range g.Spec.Resources {
			if g.Spec.Resources[i].ResourceRef == resourceRef {
				out = append(out, g.Spec.Resources[i])
			}
		}
	}
	return out
}

// RegistrationsForRequest returns the Managed-enforcement registrations that govern a usage object of
// the given (group, version, resource), i.e. one of their usageReferences' rules matches.
func RegistrationsForRequest(ctx context.Context, cl client.Client, group, version, resource string) ([]*v1alpha1.ClusterGrantableResource, error) {
	regList := &v1alpha1.ClusterGrantableResourceList{}
	if err := cl.List(ctx, regList); err != nil {
		return nil, fmt.Errorf("list ClusterGrantableResources: %w", err)
	}
	out := make([]*v1alpha1.ClusterGrantableResource, 0)
	for i := range regList.Items {
		reg := &regList.Items[i]
		if reg.Spec.Enforcement == v1alpha1.EnforcementExternal {
			continue
		}
		for j := range reg.Spec.UsageReferences {
			if engine.RuleMatches(reg.Spec.UsageReferences[j].Rule, group, version, resource) {
				out = append(out, reg)
				break
			}
		}
	}
	return out, nil
}

// Resolved holds the resolved availability for one registration in one project.
type Resolved struct {
	Reg       *v1alpha1.ClusterGrantableResource
	allowed   map[string]struct{}
	denied    map[string]struct{}
	excluded  map[string]struct{}
	liveNames []string
	anyAll    bool
	anyNone   bool
	def       string
}

// Decide reports whether the project may use the granted object of the given name, applying the
// precedence excluded → denied → allowed → grant availabilityDefault → registration defaultAvailability.
func (r *Resolved) Decide(name string) bool {
	if _, ok := r.excluded[name]; ok {
		return false
	}
	if _, ok := r.denied[name]; ok {
		return false
	}
	if _, ok := r.allowed[name]; ok {
		return true
	}
	if r.anyAll {
		return true
	}
	if r.anyNone {
		return false
	}
	return r.Reg.Spec.DefaultAvailability != v1alpha1.AvailabilityNone
}

// Default returns the effective per-project default name (may be empty).
func (r *Resolved) Default() string { return r.def }

// Available returns the catalog of available names for this project, sorted, with the default flagged.
// For object-backed resources it is the live objects that Decide() allows, plus any explicitly allowed
// name; for value-backed resources it is the allowed names.
func (r *Resolved) Available() []v1alpha1.AvailableObject {
	names := map[string]struct{}{}
	for _, n := range r.liveNames {
		if r.Decide(n) {
			names[n] = struct{}{}
		}
	}
	// Explicitly allowed names that are not live objects (e.g. value-backed values).
	for n := range r.allowed {
		if _, excluded := r.excluded[n]; excluded {
			continue
		}
		if _, denied := r.denied[n]; denied {
			continue
		}
		names[n] = struct{}{}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	out := make([]v1alpha1.AvailableObject, 0, len(sorted))
	for _, n := range sorted {
		out = append(out, v1alpha1.AvailableObject{Name: n, Default: n == r.def})
	}
	return out
}

// Resolve builds the resolved availability for a registration given the applicable grant entries. For
// object-backed resources it lists the live granted objects to expand allowed/denied/excluded selectors.
func Resolve(
	ctx context.Context,
	cl client.Client,
	reg *v1alpha1.ClusterGrantableResource,
	entries []v1alpha1.GrantResource,
) (*Resolved, error) {
	r := &Resolved{
		Reg:      reg,
		allowed:  map[string]struct{}{},
		denied:   map[string]struct{}{},
		excluded: map[string]struct{}{},
	}

	// Literal names and defaults from grant entries.
	for i := range entries {
		e := &entries[i]
		for _, n := range e.Allowed {
			r.allowed[n] = struct{}{}
		}
		for _, n := range e.Denied {
			r.denied[n] = struct{}{}
		}
		if e.Default != "" && r.def == "" {
			r.def = e.Default
		}
		switch e.AvailabilityDefault {
		case v1alpha1.AvailabilityAll:
			r.anyAll = true
		case v1alpha1.AvailabilityNone:
			r.anyNone = true
		}
	}
	// Registration excluded literal names.
	if reg.Spec.Excluded != nil {
		for _, n := range reg.Spec.Excluded.Names {
			r.excluded[n] = struct{}{}
		}
	}

	// Object-backed: list live objects and expand selectors.
	if !reg.IsValueBacked() {
		gv, err := schema.ParseGroupVersion(reg.Spec.GrantedResource.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("parse grantedResource apiVersion %q: %w", reg.Spec.GrantedResource.APIVersion, err)
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: reg.Spec.GrantedResource.Kind + "List"})
		if err := cl.List(ctx, list); err != nil {
			return nil, fmt.Errorf("list granted resource %s: %w", reg.Spec.GrantedResource.Kind, err)
		}

		excludedSel, err := filterSelector(reg.Spec.Excluded)
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			name := list.Items[i].GetName()
			objLabels := labels.Set(list.Items[i].GetLabels())
			r.liveNames = append(r.liveNames, name)
			if excludedSel != nil && excludedSel.Matches(objLabels) {
				r.excluded[name] = struct{}{}
			}
			for j := range entries {
				if matchSel(entries[j].DeniedSelector, objLabels) {
					r.denied[name] = struct{}{}
				}
				if matchSel(entries[j].AllowedSelector, objLabels) {
					r.allowed[name] = struct{}{}
				}
			}
		}
		sort.Strings(r.liveNames)
	}

	// Effective default falls back to the registration's defaultFrom annotation.
	if r.def == "" && !reg.IsValueBacked() && reg.Spec.DefaultFrom != nil && reg.Spec.DefaultFrom.AnnotationKey != "" {
		def, err := defaultFromAnnotation(ctx, cl, reg)
		if err == nil {
			r.def = def
		}
	}
	return r, nil
}

func filterSelector(f *v1alpha1.ResourceFilter) (labels.Selector, error) {
	if f == nil || (len(f.MatchLabels) == 0 && len(f.MatchExpressions) == 0) {
		return nil, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: f.MatchLabels, MatchExpressions: f.MatchExpressions})
	if err != nil {
		return nil, fmt.Errorf("invalid excluded selector: %w", err)
	}
	return sel, nil
}

func matchSel(ls *metav1.LabelSelector, objLabels labels.Set) bool {
	if ls == nil {
		return false
	}
	sel, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return false
	}
	return sel.Matches(objLabels)
}

// defaultFromAnnotation finds the single granted object annotated as the cluster-wide default.
func defaultFromAnnotation(ctx context.Context, cl client.Client, reg *v1alpha1.ClusterGrantableResource) (string, error) {
	gv, err := schema.ParseGroupVersion(reg.Spec.GrantedResource.APIVersion)
	if err != nil {
		return "", err
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: reg.Spec.GrantedResource.Kind + "List"})
	if err := cl.List(ctx, list); err != nil {
		return "", err
	}
	var found []string
	for i := range list.Items {
		if _, ok := list.Items[i].GetAnnotations()[reg.Spec.DefaultFrom.AnnotationKey]; ok {
			found = append(found, list.Items[i].GetName())
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	return "", fmt.Errorf("no single default annotated object (found %d)", len(found))
}

// ProjectNamespaces returns the names of namespaces belonging to the project, including the control
// namespace (the project name). The project namespaces are those labelled projects.deckhouse.io/project.
func ProjectNamespaces(ctx context.Context, cl client.Client, project string) ([]string, error) {
	nsList := &corev1.NamespaceList{}
	if err := cl.List(ctx, nsList, client.MatchingLabels{naming.ProjectLabel: project}); err != nil {
		return nil, fmt.Errorf("list project namespaces: %w", err)
	}
	out := make([]string, 0, len(nsList.Items)+1)
	for i := range nsList.Items {
		out = append(out, nsList.Items[i].Name)
	}
	if !slices.Contains(out, project) {
		// Control namespace is named after the project; include it even if it lacks the label.
		ns := &corev1.Namespace{}
		if err := cl.Get(ctx, client.ObjectKey{Name: project}, ns); err == nil {
			out = append(out, project)
		}
	}
	sort.Strings(out)
	return out, nil
}
