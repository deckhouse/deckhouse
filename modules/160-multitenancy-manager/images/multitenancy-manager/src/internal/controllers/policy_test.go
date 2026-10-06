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
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

func policyCondition(t *testing.T, r *PolicyReconciler, name string) *metav1.Condition {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	policy := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, policy); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(policy.Status.Conditions, PolicyConditionSelectorsValid)
	if cond == nil {
		t.Fatalf("condition %s missing on %s", PolicyConditionSelectorsValid, name)
	}
	if policy.Status.ObservedGeneration != policy.Generation {
		t.Fatalf("observedGeneration %d != generation %d", policy.Status.ObservedGeneration, policy.Generation)
	}
	return cond
}

// TestPolicy_InvalidSelectorIsReported: a matchExpressions entry the API server accepts but the
// selector library refuses (In without values) used to become a silent "matches nothing"; the
// policy now says so in its status.
func TestPolicy_InvalidSelectorIsReported(t *testing.T) {
	bad := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Generation: 3},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceName:    "storageclasses",
				AllowedSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "tier", Operator: metav1.LabelSelectorOpIn}}},
			}},
		},
	}
	good := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "good", Generation: 1},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
	r := &PolicyReconciler{Client: buildClientWithStatus(t, bad, good)}

	cond := policyCondition(t, r, "bad")
	if cond.Status != metav1.ConditionFalse || cond.Reason != "InvalidSelector" {
		t.Fatalf("expected False/InvalidSelector, got %s/%s", cond.Status, cond.Reason)
	}
	if !strings.Contains(cond.Message, "spec.resources[0].allowedSelector (storageclasses)") {
		t.Fatalf("message must point at the selector, got %q", cond.Message)
	}

	cond = policyCondition(t, r, "good")
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("a valid policy must be True, got %s: %s", cond.Status, cond.Message)
	}
}

// TestPolicy_InvalidNamespaceSelectorIsReported: the namespace selector is checked the same way as
// the project selector, and the message names it.
func TestPolicy_InvalidNamespaceSelectorIsReported(t *testing.T) {
	bad := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Generation: 1},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "env", Operator: metav1.LabelSelectorOpIn}}},
			Resources:         []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
	r := &PolicyReconciler{Client: buildClientWithStatus(t, bad)}

	cond := policyCondition(t, r, "bad")
	if cond.Status != metav1.ConditionFalse || cond.Reason != "InvalidSelector" {
		t.Fatalf("expected False/InvalidSelector, got %s/%s", cond.Status, cond.Reason)
	}
	if !strings.Contains(cond.Message, "spec.namespaceSelector") {
		t.Fatalf("message must point at the namespace selector, got %q", cond.Message)
	}
}

func rolesMapper() apimeta.RESTMapper {
	m := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "rbac.authorization.k8s.io", Version: "v1"}})
	m.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, apimeta.RESTScopeRoot)
	return m
}

// TestPolicy_NonDelegatableRoleIsReported: the clusterroles registration excludes every ClusterRole
// not marked delegatable, and an exclusion wins over an allow-list, so a policy that allows such a
// role grants nothing. USAGE told administrators to write exactly that policy; the condition now
// tells them why nothing happened.
func TestPolicy_NonDelegatableRoleIsReported(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "clusterroles"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			Excluded: []v1alpha1.ResourceFilter{{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "rbac.deckhouse.io/delegatable", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"true"},
			}}}},
		},
	}
	delegatable := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "team-viewer", Labels: map[string]string{"rbac.deckhouse.io/delegatable": "true"}}}
	plain := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "team-editor"}}
	policy := &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "roles", Generation: 1},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "clusterroles", Allowed: []string{"team-viewer", "team-editor"}}},
		},
	}
	r := &PolicyReconciler{Client: buildClientWithStatus(t, def, delegatable, plain, policy), Mapper: rolesMapper()}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil {
		t.Fatal(err)
	}
	got := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "roles"}, got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, PolicyConditionAllowedEffective)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("expected AllowedEffective=False, got %+v", cond)
	}
	if !strings.Contains(cond.Message, `"team-editor"`) || !strings.Contains(cond.Message, "delegatable") || strings.Contains(cond.Message, `"team-viewer"`) {
		t.Fatalf("message must name the non-delegatable role only, got %q", cond.Message)
	}

	// A ClusterRole label change is what heals the condition, so the label watch must enqueue
	// this policy, and until then the reconciler asks to be re-run on the catalog cadence.
	if reqs := r.policiesAllowing(context.Background(), plain); len(reqs) != 1 || reqs[0].Name != "roles" {
		t.Fatalf("a ClusterRole change must enqueue the allow-list policy, got %v", reqs)
	}
	if res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil || res.RequeueAfter != ResyncInterval {
		t.Fatalf("an ineffective policy must requeue after %s, got %+v %v", ResyncInterval, res, err)
	}

	// Marking the role delegatable clears the condition on the next reconcile.
	plain.Labels = map[string]string{"rbac.deckhouse.io/delegatable": "true"}
	if err := r.Update(context.Background(), plain); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "roles"}, got); err != nil {
		t.Fatal(err)
	}
	if cond := apimeta.FindStatusCondition(got.Status.Conditions, PolicyConditionAllowedEffective); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected AllowedEffective=True once the role is delegatable, got %+v", cond)
	}
	if res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "roles"}}); err != nil || res.RequeueAfter != 0 {
		t.Fatalf("an effective policy must not requeue, got %+v %v", res, err)
	}
}

func storageDefinition(availability v1alpha1.AvailabilityDefault) *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: availability,
		},
	}
}

func restrictingPolicy(namespaceSelector *metav1.LabelSelector) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "restrict", Generation: 1},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{
			NamespaceSelector: namespaceSelector,
			Resources:         []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"standard"}}},
		},
	}
}

// projectObjects is a project whose main namespace carries two template labels (tier, and zone, whose
// value an administrator changed by hand from a to b), an inherited module label and a label set by
// hand (hand).
func projectObjects() []client.Object {
	return projectNamed("team-a")
}

func projectNamed(name string) []client.Object {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			naming.ProjectLabel: name, "tier": "gold", "zone": "b", "hand": "yes",
			"security.deckhouse.io/pod-policy": "baseline",
		},
		Annotations: map[string]string{naming.TemplateLabelsAnnotation: `{"tier":"gold","zone":"a"}`},
	}}
	return []client.Object{project, main}
}

func newNamespacesCovered(t *testing.T, r *PolicyReconciler) *metav1.Condition {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "restrict"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	policy := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "restrict"}, policy); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(policy.Status.Conditions, PolicyConditionNewNamespacesCovered)
	if cond == nil {
		t.Fatal("the condition is missing")
	}
	return cond
}

// TestPolicy_NewNamespacesCovered: a policy that restricts a resource whose baseline is All, and
// picks namespaces by a label a new additional namespace may not carry, says so; the other cases are
// fine.
func TestPolicy_NewNamespacesCovered(t *testing.T) {
	tests := []struct {
		name         string
		availability v1alpha1.AvailabilityDefault
		selector     *metav1.LabelSelector
		expected     metav1.ConditionStatus
		mentions     string
	}{
		{name: "no namespace selector", availability: v1alpha1.AvailabilityAll, expected: metav1.ConditionTrue},
		{name: "a template label", availability: v1alpha1.AvailabilityAll, selector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "gold"}}, expected: metav1.ConditionTrue},
		{name: "a label set by hand", availability: v1alpha1.AvailabilityAll, selector: &metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}}, expected: metav1.ConditionFalse, mentions: "hand"},
		{name: "a tenant-chosen value", availability: v1alpha1.AvailabilityAll, selector: &metav1.LabelSelector{MatchLabels: map[string]string{"projects.deckhouse.io/project-namespace": "cache"}}, expected: metav1.ConditionFalse, mentions: "projects.deckhouse.io/project-namespace"},
		{
			name:         "every additional namespace",
			availability: v1alpha1.AvailabilityAll,
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "projects.deckhouse.io/project-namespace", Operator: metav1.LabelSelectorOpExists},
			}},
			expected: metav1.ConditionTrue,
		},
		{name: "a grant, not a restriction", availability: v1alpha1.AvailabilityNone, selector: &metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}}, expected: metav1.ConditionTrue},
		{
			name:         "NotIn on a label set by hand: a new namespace without the label matches",
			availability: v1alpha1.AvailabilityAll,
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "hand", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"no"}},
			}},
			expected: metav1.ConditionTrue,
		},
		{
			name:         "a template label whose value on the main namespace was changed by hand",
			availability: v1alpha1.AvailabilityAll,
			selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"zone": "b"}},
			expected:     metav1.ConditionFalse,
			mentions:     "gets zone=a",
		},
		{name: "an inherited module label", availability: v1alpha1.AvailabilityAll, selector: &metav1.LabelSelector{MatchLabels: map[string]string{"security.deckhouse.io/pod-policy": "baseline"}}, expected: metav1.ConditionTrue},
		{
			name:         "main namespace only, through the project-namespace label",
			availability: v1alpha1.AvailabilityAll,
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "projects.deckhouse.io/project-namespace", Operator: metav1.LabelSelectorOpDoesNotExist},
			}},
			expected: metav1.ConditionFalse,
			mentions: "every additional namespace carries it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append(projectObjects(), storageDefinition(tt.availability), restrictingPolicy(tt.selector))
			r := &PolicyReconciler{Client: buildClientWithStatus(t, objs...)}

			cond := newNamespacesCovered(t, r)
			if cond.Status != tt.expected {
				t.Fatalf("got %s (%s), expected %s", cond.Status, cond.Message, tt.expected)
			}
			if tt.mentions != "" && !strings.Contains(cond.Message, tt.mentions) {
				t.Fatalf("the message must name %q, got %q", tt.mentions, cond.Message)
			}
		})
	}
}

// TestPolicy_NewNamespacesCoveredCountsWhatRestricts: denied names restrict whatever the baseline is,
// an allow-list or availabilityDefault None only on a baseline of All, and an entry that only sets a
// default or opens the resource restricts nothing.
func TestPolicy_NewNamespacesCoveredCountsWhatRestricts(t *testing.T) {
	handSet := &metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}}
	tests := []struct {
		name         string
		availability v1alpha1.AvailabilityDefault
		entry        v1alpha1.GrantResource
		expected     metav1.ConditionStatus
		mentions     string
	}{
		{name: "All with denied names", availability: v1alpha1.AvailabilityAll, entry: v1alpha1.GrantResource{AvailabilityDefault: v1alpha1.AvailabilityAll, Denied: []string{"fast"}}, expected: metav1.ConditionFalse, mentions: "the names the policy denies are not refused there"},
		{name: "a denied selector on a None baseline another policy may open", availability: v1alpha1.AvailabilityNone, entry: v1alpha1.GrantResource{DeniedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "fast"}}}, expected: metav1.ConditionFalse, mentions: "the names the policy denies are not refused there"},
		{name: "None on an All baseline", availability: v1alpha1.AvailabilityAll, entry: v1alpha1.GrantResource{AvailabilityDefault: v1alpha1.AvailabilityNone}, expected: metav1.ConditionFalse, mentions: "the resource baseline is All"},
		{name: "a default only", availability: v1alpha1.AvailabilityAll, entry: v1alpha1.GrantResource{Default: "standard"}, expected: metav1.ConditionTrue},
		{name: "All without denied names", availability: v1alpha1.AvailabilityAll, entry: v1alpha1.GrantResource{AvailabilityDefault: v1alpha1.AvailabilityAll}, expected: metav1.ConditionTrue},
		{name: "an allow-list on a None baseline", availability: v1alpha1.AvailabilityNone, entry: v1alpha1.GrantResource{Allowed: []string{"standard"}}, expected: metav1.ConditionTrue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := restrictingPolicy(handSet)
			tt.entry.ResourceName = "storageclasses"
			policy.Spec.Resources = []v1alpha1.GrantResource{tt.entry}
			objs := append(projectObjects(), storageDefinition(tt.availability), policy)
			r := &PolicyReconciler{Client: buildClientWithStatus(t, objs...)}

			cond := newNamespacesCovered(t, r)
			if cond.Status != tt.expected {
				t.Fatalf("got %s (%s), expected %s", cond.Status, cond.Message, tt.expected)
			}
			if tt.mentions != "" && !strings.Contains(cond.Message, tt.mentions) {
				t.Fatalf("the message must say %q, got %q", tt.mentions, cond.Message)
			}
		})
	}
}

// reversedLists returns namespace lists in reverse name order, the way an informer cache may return
// them in any order.
type reversedLists struct {
	client.Client
}

func (c reversedLists) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, list, opts...); err != nil {
		return err
	}
	if namespaces, ok := list.(*corev1.NamespaceList); ok {
		slices.Reverse(namespaces.Items)
	}
	return nil
}

// TestPolicy_NewNamespacesCoveredIsStable: the message names the first project in name order whatever
// order the namespaces come in, so a second pass over an unchanged cluster writes no status. A message
// that followed the list order was rewritten on nearly every pass, and every write re-queued the policy.
func TestPolicy_NewNamespacesCoveredIsStable(t *testing.T) {
	objs := append(append(projectNamed("team-a"), projectNamed("team-b")...),
		storageDefinition(v1alpha1.AvailabilityAll), restrictingPolicy(&metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}}))
	base := buildClientWithStatus(t, objs...)
	r := &PolicyReconciler{Client: reversedLists{Client: base}}

	first := newNamespacesCovered(t, r)
	if !strings.Contains(first.Message, "project team-a") {
		t.Fatalf("the message must name the first project in name order, got %q", first.Message)
	}
	policy := &grantsv1alpha2.ClusterResourceGrantPolicy{}
	if err := base.Get(context.Background(), types.NamespacedName{Name: "restrict"}, policy); err != nil {
		t.Fatal(err)
	}
	version := policy.ResourceVersion

	r.Client = base
	second := newNamespacesCovered(t, r)
	if second.Message != first.Message {
		t.Fatalf("the message changed with the list order: %q, then %q", first.Message, second.Message)
	}
	if err := base.Get(context.Background(), types.NamespacedName{Name: "restrict"}, policy); err != nil {
		t.Fatal(err)
	}
	if policy.ResourceVersion != version {
		t.Fatal("an unchanged cluster must not rewrite the policy status")
	}
}

// countingLists records the kinds of list the reconciler asks for.
type countingLists struct {
	client.Client
	listed *[]string
}

func (c countingLists) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	switch list.(type) {
	case *corev1.NamespaceList:
		*c.listed = append(*c.listed, "namespaces")
	case *v1alpha3.ProjectList:
		*c.listed = append(*c.listed, "projects")
	}
	return c.Client.List(ctx, list, opts...)
}

// TestPolicy_NamespaceResync: a policy is re-checked on the resync cadence only when its
// namespaceSelector relies on a label a namespace can gain or lose on its own. A template-managed
// policy selects by project-template, which every new namespace takes from the main one: it is
// covered without listing the cluster and is not resynced. A hand-set label keeps the resync even while
// the policy restricts nothing, since only the resync notices a definition that starts to make it
// restrict.
func TestPolicy_NamespaceResync(t *testing.T) {
	tests := []struct {
		name         string
		availability v1alpha1.AvailabilityDefault
		selector     *metav1.LabelSelector
		entry        v1alpha1.GrantResource
		resync       bool
		lists        bool
	}{
		{
			name:         "template-managed",
			availability: v1alpha1.AvailabilityAll,
			selector:     &metav1.LabelSelector{MatchLabels: map[string]string{v1alpha3.ResourceLabelTemplate: "default"}},
			entry:        v1alpha1.GrantResource{Denied: []string{"fast"}},
		},
		{
			name:         "a hand-set label on a policy that restricts nothing yet",
			availability: v1alpha1.AvailabilityNone,
			selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}},
			entry:        v1alpha1.GrantResource{AvailabilityDefault: v1alpha1.AvailabilityNone},
			resync:       true,
		},
		{
			name:         "a hand-set label on a restricting policy",
			availability: v1alpha1.AvailabilityAll,
			selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"hand": "yes"}},
			entry:        v1alpha1.GrantResource{Denied: []string{"fast"}},
			resync:       true,
			lists:        true,
		},
		{
			name:         "no namespace selector",
			availability: v1alpha1.AvailabilityAll,
			entry:        v1alpha1.GrantResource{Denied: []string{"fast"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := restrictingPolicy(tt.selector)
			tt.entry.ResourceName = "storageclasses"
			policy.Spec.Resources = []v1alpha1.GrantResource{tt.entry}
			var listed []string
			base := buildClientWithStatus(t, append(projectObjects(), storageDefinition(tt.availability), policy)...)
			r := &PolicyReconciler{Client: countingLists{Client: base, listed: &listed}}

			res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "restrict"}})
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			if resync := res.RequeueAfter == ResyncInterval; resync != tt.resync {
				t.Fatalf("resync %v, expected %v (%+v)", resync, tt.resync, res)
			}
			if lists := len(listed) > 0; lists != tt.lists {
				t.Fatalf("listed %v, expected listing %v", listed, tt.lists)
			}

			r.Client = base
			if cond := newNamespacesCovered(t, r); !tt.lists && cond.Status != metav1.ConditionTrue {
				t.Fatalf("got %s (%s), expected True", cond.Status, cond.Message)
			}
		})
	}
}
