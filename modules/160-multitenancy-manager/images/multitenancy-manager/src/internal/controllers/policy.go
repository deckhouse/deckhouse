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

package controllers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/projectnamespace"
	"controller/internal/naming"
	"controller/internal/resolve"
)

// PolicyConditionSelectorsValid is the ClusterResourceGrantPolicy condition that says every label
// selector of the policy compiles. The resolver treats a selector it cannot compile as matching
// nothing, which is the safe direction for an allow-list -- and exactly the direction nobody
// notices: the project simply gets less than the author meant. The condition is where the author
// finds out.
const PolicyConditionSelectorsValid = "SelectorsValid"

// PolicyConditionAllowedEffective says that every name a policy allows can actually be granted.
// The registration of a resource may exclude objects from every project -- the clusterroles
// registration excludes every ClusterRole not marked rbac.deckhouse.io/delegatable=true -- and an
// exclusion wins over an allow-list, so a policy that allows such a name grants nothing and nobody
// told the author. This condition tells them, with the name and the reason.
const PolicyConditionAllowedEffective = "AllowedEffective"

// PolicyConditionNewNamespacesCovered says whether a policy that restricts a resource still covers an
// additional namespace a project user creates later. Such a namespace carries only the labels the
// module sets and the labels of the project template; a restriction whose namespaceSelector relies on
// a label set by hand, or on a value the project user chooses (the name of the namespace), does not
// reach it, and with a baseline of All the new namespace sees every resource.
const PolicyConditionNewNamespacesCovered = "NewNamespacesCovered"

// tenantChosenLabels are labels whose value a project user picks, through the name of a
// ProjectNamespace. Every additional namespace carries them, with whatever value the user chose.
var tenantChosenLabels = map[string]bool{
	"kubernetes.io/metadata.name":             true,
	"projects.deckhouse.io/project-namespace": true,
}

// PolicyReconciler keeps the status of a ClusterResourceGrantPolicy honest about its own spec.
type PolicyReconciler struct {
	client.Client
	// Mapper resolves the granted resources of the definitions the policy names; the same resolver
	// the webhooks use answers whether an allowed name would be granted.
	Mapper apimeta.RESTMapper
}

// Reconcile validates the policy and records the outcome as conditions.
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	selectors := metav1.Condition{Type: PolicyConditionSelectorsValid, ObservedGeneration: policy.Generation}
	if problems := invalidSelectors(policy); len(problems) == 0 {
		selectors.Status = metav1.ConditionTrue
		selectors.Reason = "Valid"
		selectors.Message = "Every label selector of the policy compiles."
	} else {
		selectors.Status = metav1.ConditionFalse
		selectors.Reason = "InvalidSelector"
		selectors.Message = strings.Join(problems, "; ") + ". A selector that does not compile matches nothing, so the projects get less than the policy intends."
	}

	effective := metav1.Condition{Type: PolicyConditionAllowedEffective, ObservedGeneration: policy.Generation}
	ineffective, err := r.ineffectiveAllowed(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check allowed names: %w", err)
	}
	if len(ineffective) == 0 {
		effective.Status = metav1.ConditionTrue
		effective.Reason = "Effective"
		effective.Message = "Every allowed name can be granted."
	} else {
		effective.Status = metav1.ConditionFalse
		effective.Reason = "ExcludedByDefinition"
		effective.Message = strings.Join(ineffective, "; ")
	}

	covered := metav1.Condition{Type: PolicyConditionNewNamespacesCovered, ObservedGeneration: policy.Generation}
	uncovered, how, err := r.uncoveredNamespaceKeys(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check the namespace selector: %w", err)
	}
	if len(uncovered) == 0 {
		covered.Status = metav1.ConditionTrue
		covered.Reason = "Covered"
		covered.Message = "A new namespace of a selected project is covered like the existing ones."
	} else {
		consequence := "the resource baseline is All, so such a namespace is not restricted"
		if how == restrictionByDenial {
			consequence = "the names the policy denies are not refused there, so another policy or the resource baseline may grant them"
		}
		covered.Status = metav1.ConditionFalse
		covered.Reason = "NamespaceLabelNotInherited"
		covered.Message = "spec.namespaceSelector relies on " + strings.Join(uncovered, "; ") +
			". A new additional namespace of a project may not match it, and " + consequence +
			". Select projects with spec.projectSelector, or use a label of the project template."
	}

	// An ineffective entry depends on objects this reconciler does not own (a label on a
	// ClusterRole, an excluded filter on a definition), and so do namespace labels; besides the
	// watches below, re-check on the catalog's cadence so the conditions heal even when no event
	// reaches us.
	result := ctrl.Result{}
	if len(ineffective) > 0 || needsNamespaceResync(policy) {
		result.RequeueAfter = ResyncInterval
	}

	before := policy.Status.DeepCopy()
	policy.Status.ObservedGeneration = policy.Generation
	apimeta.SetStatusCondition(&policy.Status.Conditions, selectors)
	apimeta.SetStatusCondition(&policy.Status.Conditions, effective)
	apimeta.SetStatusCondition(&policy.Status.Conditions, covered)
	if before.ObservedGeneration == policy.Status.ObservedGeneration &&
		conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionSelectorsValid) &&
		conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionAllowedEffective) &&
		conditionUnchanged(before.Conditions, policy.Status.Conditions, PolicyConditionNewNamespacesCovered) {
		return result, nil
	}
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, fmt.Errorf("update policy status: %w", err)
	}
	return result, nil
}

// uncoveredNamespaceKeys names the namespaceSelector keys of a restricting policy that a new
// additional namespace of a covered project would not match. The labels such a namespace gets are
// known in advance (projectnamespace.NewNamespaceLabels), except the ones derived from its name, which
// the project user chooses: a requirement on those holds for every new namespace only when it asks for
// the label to exist. Every other requirement is checked against the labels a new namespace would get
// in each project the policy covers now. Nothing is reported for a policy without a namespaceSelector,
// one whose selector does not compile (it covers nothing), or one that restricts no resource with a
// baseline of All.
//
// Projects are visited in name order and each key is reported once, for the first project it fails in,
// so the message does not change between passes while nothing changes in the cluster.
func (r *PolicyReconciler) uncoveredNamespaceKeys(ctx context.Context, policy *grantsv1alpha2.ClusterResourceGrantPolicy) ([]string, restriction, error) {
	selector := policy.Spec.NamespaceSelector
	if selector == nil {
		return nil, restrictionNone, nil
	}
	if _, err := metav1.LabelSelectorAsSelector(selector); err != nil {
		return nil, restrictionNone, nil
	}
	how, err := r.restrictionOf(ctx, policy)
	if err != nil || how == restrictionNone {
		return nil, how, err
	}

	requirements := make([]metav1.LabelSelectorRequirement, 0, len(selector.MatchLabels)+len(selector.MatchExpressions))
	for key, value := range selector.MatchLabels {
		requirements = append(requirements, metav1.LabelSelectorRequirement{Key: key, Operator: metav1.LabelSelectorOpIn, Values: []string{value}})
	}
	requirements = append(requirements, selector.MatchExpressions...)

	var projects []coveredProject
	found := map[string]string{}
	for _, requirement := range requirements {
		if _, reported := found[requirement.Key]; reported {
			continue
		}
		// A new namespace takes these from its project or its main namespace, and so matches as the
		// existing namespaces of a covered project do (REQ-08). The selector of a template-managed
		// policy is only such a key, and needs no listing.
		if projectnamespace.IsInheritedFromProject(requirement.Key) {
			continue
		}
		if tenantChosenLabels[requirement.Key] {
			switch requirement.Operator {
			case metav1.LabelSelectorOpExists:
			case metav1.LabelSelectorOpDoesNotExist:
				found[requirement.Key] = "every additional namespace carries it"
			default:
				found[requirement.Key] = "its value is the name a project user gives a namespace"
			}
			continue
		}
		if projects == nil {
			if projects, err = r.coveredProjects(ctx, policy); err != nil {
				return nil, how, err
			}
		}
		for _, project := range projects {
			if !requirementMatches(requirement, project.newNamespaceLabels) {
				found[requirement.Key] = fmt.Sprintf("a new additional namespace of project %s gets %s", project.name, describeLabel(project.newNamespaceLabels, requirement.Key))
				break
			}
		}
	}

	out := make([]string, 0, len(found))
	for key, why := range found {
		out = append(out, fmt.Sprintf("%s (%s)", key, why))
	}
	slices.Sort(out)
	return out, how, nil
}

// needsNamespaceResync reports whether the policy's namespaceSelector has a requirement on a label a
// namespace can gain or lose while the policy stays as it is: one neither taken from the project nor
// chosen by the project user. The NewNamespacesCovered condition of such a policy follows namespace
// labels, and no watch wakes it for them, whatever the policy restricts today: the definition watch
// wakes only policies with allowed names. A template-managed policy selects by project-template
// alone and is not resynced.
func needsNamespaceResync(policy *grantsv1alpha2.ClusterResourceGrantPolicy) bool {
	selector := policy.Spec.NamespaceSelector
	if selector == nil {
		return false
	}
	if _, err := metav1.LabelSelectorAsSelector(selector); err != nil {
		return false
	}
	followsLabels := func(key string) bool {
		return !projectnamespace.IsInheritedFromProject(key) && !tenantChosenLabels[key]
	}
	for key := range selector.MatchLabels {
		if followsLabels(key) {
			return true
		}
	}
	for _, requirement := range selector.MatchExpressions {
		if followsLabels(requirement.Key) {
			return true
		}
	}
	return false
}

// restriction is how a policy makes a namespace it covers see less than a namespace it misses.
type restriction int

const (
	restrictionNone restriction = iota
	// restrictionByDenial: the policy denies names, which are refused even where another policy opens
	// the resource.
	restrictionByDenial
	// restrictionByNarrowing: an allow-list or availabilityDefault None narrows a resource whose
	// baseline is All.
	restrictionByNarrowing
)

// restrictionOf reports how the policy restricts, narrowing first when it does both. Denied names
// restrict whatever the baseline is. An allow-list or availabilityDefault None narrows only a resource
// whose baseline is All; an entry that sets only a default, or opens the resource with All, restricts
// nothing.
func (r *PolicyReconciler) restrictionOf(ctx context.Context, policy *grantsv1alpha2.ClusterResourceGrantPolicy) (restriction, error) {
	how := restrictionNone
	for _, entry := range policy.Spec.Resources {
		denies := len(entry.Denied) > 0 || entry.DeniedSelector != nil
		narrows := entry.AvailabilityDefault == v1alpha1.AvailabilityNone ||
			(entry.AvailabilityDefault == "" && (len(entry.Allowed) > 0 || entry.AllowedSelector != nil))
		if !denies && !narrows {
			continue
		}
		def := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := r.Get(ctx, client.ObjectKey{Name: entry.ResourceName}, def); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return restrictionNone, fmt.Errorf("get GrantableClusterResourceDefinition %s: %w", entry.ResourceName, err)
		}
		if narrows && def.Spec.DefaultAvailability != v1alpha1.AvailabilityNone {
			return restrictionByNarrowing, nil
		}
		if denies {
			how = restrictionByDenial
		}
	}
	return how, nil
}

// coveredProject is a project with a namespace the policy covers, with the labels a new additional
// namespace of it would get.
type coveredProject struct {
	name               string
	newNamespaceLabels map[string]string
}

// coveredProjects returns, in name order, every project that has a namespace the policy covers and a
// main namespace a new additional namespace can be made from. A project whose main namespace is not
// rendered gets no new namespace until it is, and is checked again on the next pass.
func (r *PolicyReconciler) coveredProjects(ctx context.Context, policy *grantsv1alpha2.ClusterResourceGrantPolicy) ([]coveredProject, error) {
	projects := &v1alpha3.ProjectList{}
	if err := r.List(ctx, projects); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projectLabels := make(map[string]map[string]string, len(projects.Items))
	for i := range projects.Items {
		projectLabels[projects.Items[i].Name] = projects.Items[i].Labels
	}

	namespaces := &corev1.NamespaceList{}
	if err := r.List(ctx, namespaces, client.HasLabels{naming.ProjectLabel}); err != nil {
		return nil, fmt.Errorf("list project namespaces: %w", err)
	}
	mains := map[string]*corev1.Namespace{}
	covered := map[string]bool{}
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		project := ns.Labels[naming.ProjectLabel]
		if ns.Name == project {
			mains[project] = ns
		}
		if resolve.PolicyMatches(policy, projectLabels[project], ns.Labels) {
			covered[project] = true
		}
	}

	out := make([]coveredProject, 0, len(covered))
	for project := range covered {
		main, ok := mains[project]
		if !ok {
			continue
		}
		labels, err := projectnamespace.NewNamespaceLabels(main, project)
		if err != nil {
			continue
		}
		out = append(out, coveredProject{name: project, newNamespaceLabels: labels})
	}
	slices.SortFunc(out, func(a, b coveredProject) int { return strings.Compare(a.name, b.name) })
	return out, nil
}

// requirementMatches reports whether a namespace with the labels satisfies one selector requirement.
func requirementMatches(requirement metav1.LabelSelectorRequirement, set map[string]string) bool {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{requirement}})
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(set))
}

// describeLabel says what a namespace with the labels carries under the key.
func describeLabel(set map[string]string, key string) string {
	if value, ok := set[key]; ok {
		return fmt.Sprintf("%s=%s", key, value)
	}
	return "no " + key + " label"
}

// policiesAllowing enqueues every policy with an allow-list: the object whose labels changed may be
// one of the allowed names, and policies are few enough that filtering by name is not worth it.
func (r *PolicyReconciler) policiesAllowing(ctx context.Context, _ client.Object) []reconcile.Request {
	policies := &grantsv1alpha2.ClusterResourceGrantPolicyList{}
	if err := r.List(ctx, policies); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range policies.Items {
		for _, entry := range policies.Items[i].Spec.Resources {
			if len(entry.Allowed) > 0 {
				out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Name: policies.Items[i].Name}})
				break
			}
		}
	}
	return out
}

// ineffectiveAllowed names every allowed entry the resolver would refuse anyway: the registration
// excludes it. Entries whose definition is missing are the binding reconciler's business and are
// left alone here; a value-backed definition has no objects to exclude.
func (r *PolicyReconciler) ineffectiveAllowed(ctx context.Context, policy *grantsv1alpha2.ClusterResourceGrantPolicy) ([]string, error) {
	if r.Mapper == nil {
		return nil, nil
	}
	var out []string
	for i := range policy.Spec.Resources {
		entry := policy.Spec.Resources[i]
		if len(entry.Allowed) == 0 {
			continue
		}
		def := &v1alpha1.GrantableClusterResourceDefinition{}
		if err := r.Get(ctx, client.ObjectKey{Name: entry.ResourceName}, def); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return nil, err
		}
		if def.IsValueBacked() || len(def.Spec.Excluded) == 0 {
			continue
		}
		resolved, err := resolve.Resolve(ctx, r.Client, r.Mapper, def, []v1alpha1.GrantResource{entry})
		if err != nil {
			// The granted kind is not served (its CRD is absent) or is namespaced, which is refused:
			// nothing can be said about the names.
			continue
		}
		for _, name := range entry.Allowed {
			if resolved.Decide(name) {
				continue
			}
			reason := "it is excluded by the GrantableClusterResourceDefinition"
			if def.Spec.GrantedResource != nil && def.Spec.GrantedResource.Kind == "ClusterRole" {
				reason = "the ClusterRole is not marked rbac.deckhouse.io/delegatable=true, so it cannot be granted to projects"
			}
			out = append(out, fmt.Sprintf("spec.resources[%d].allowed (%s): %q grants nothing: %s", i, entry.ResourceName, name, reason))
		}
	}
	return out, nil
}

// invalidSelectors names every selector of the policy that does not compile, in the words the
// author needs to find it.
func invalidSelectors(policy *grantsv1alpha2.ClusterResourceGrantPolicy) []string {
	var problems []string
	for _, s := range []struct {
		field    string
		selector *metav1.LabelSelector
	}{
		{field: "spec.projectSelector", selector: policy.Spec.ProjectSelector},
		{field: "spec.namespaceSelector", selector: policy.Spec.NamespaceSelector},
	} {
		if s.selector == nil {
			continue
		}
		if _, err := metav1.LabelSelectorAsSelector(s.selector); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", s.field, err))
		}
	}
	for i := range policy.Spec.Resources {
		entry := &policy.Spec.Resources[i]
		if entry.AllowedSelector != nil {
			if _, err := metav1.LabelSelectorAsSelector(entry.AllowedSelector); err != nil {
				problems = append(problems, fmt.Sprintf("spec.resources[%d].allowedSelector (%s): %v", i, entry.ResourceName, err))
			}
		}
		if entry.DeniedSelector != nil {
			if _, err := metav1.LabelSelectorAsSelector(entry.DeniedSelector); err != nil {
				problems = append(problems, fmt.Sprintf("spec.resources[%d].deniedSelector (%s): %v", i, entry.ResourceName, err))
			}
		}
	}
	return problems
}

// conditionUnchanged reports whether the named condition has the same status, reason and message on
// both sides, ignoring the transition timestamp SetStatusCondition refreshes.
func conditionUnchanged(before, after []metav1.Condition, condType string) bool {
	b := apimeta.FindStatusCondition(before, condType)
	a := apimeta.FindStatusCondition(after, condType)
	if b == nil || a == nil {
		return b == a
	}
	return b.Status == a.Status && b.Reason == a.Reason && b.Message == a.Message && b.ObservedGeneration == a.ObservedGeneration
}

// SetupWithManager wires the reconciler.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&grantsv1alpha2.ClusterResourceGrantPolicy{}).
		Named("grant-policy-status").
		// A definition's excluded filter or a ClusterRole's delegatable label decides whether an
		// allowed name grants anything; both change without touching the policy.
		Watches(&v1alpha1.GrantableClusterResourceDefinition{}, handler.EnqueueRequestsFromMapFunc(r.policiesAllowing)).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(r.policiesAllowing),
			builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Complete(r)
}
