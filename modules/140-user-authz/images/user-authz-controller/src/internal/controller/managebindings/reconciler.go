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

// Package managebindings projects the ClusterRoleBindings of the experimental role model's manage
// roles (d8:manage:*) into namespaced use RoleBindings.
//
// A manage role carries the label rbac.deckhouse.io/use-role naming the use role its subjects get
// in the namespaces of the modules it manages. Those namespaces are found through the role's
// aggregation rule: every manage ClusterRole matched by the selectors contributes the namespace in
// its rbac.deckhouse.io/namespace label, and roles that aggregate further are followed to any
// depth (subsystem roles aggregating module roles, "all" aggregating subsystems). For each manage
// ClusterRoleBinding and each such namespace a RoleBinding d8:use:<use-role>:binding:<binding> is
// kept; automated use RoleBindings that are no longer expected are removed. The whole set is
// recomputed on every change (single request key); the ClusterRoleBindings are read through a cache
// index by roleRef, one lookup per manage role, so that a reconcile does not copy every
// ClusterRoleBinding of the cluster. A manage role is any ClusterRole labelled
// rbac.deckhouse.io/kind=manage, whatever its name (the hook this reconciler replaces matched by
// label too; user-created manage roles of the legacy scheme are named custom:*).
package managebindings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authz-controller/internal/metrics"
)

const (
	// RequestName is the constant key every event is mapped to.
	RequestName = "manage-bindings"

	// RoleRefIndexField indexes the ClusterRoleBindings by the ClusterRole they reference.
	RoleRefIndexField = "user-authz.deckhouse.io/role-ref"
	// AutomatedIndexField indexes the use RoleBindings this reconciler owns: a plain label selector on
	// the cache would scan every RoleBinding of the cluster on every reconcile.
	AutomatedIndexField = "user-authz.deckhouse.io/automated-use-binding"
	automatedIndexValue = "true"

	LabelKind      = "rbac.deckhouse.io/kind"
	LabelUseRole   = "rbac.deckhouse.io/use-role"
	LabelNamespace = "rbac.deckhouse.io/namespace"
	labelHeritage  = "heritage"
	labelAutomated = "rbac.deckhouse.io/automated"
	labelDict      = "rbac.deckhouse.io/dict"

	KindManage = "manage"

	useRolePrefix         = "d8:use:role:"
	relatedWithAnnotation = "rbac.deckhouse.io/related-with"
)

// ManageRoleLabels select the manage ClusterRoles.
var ManageRoleLabels = map[string]string{LabelKind: KindManage}

// AutomatedLabels mark the use RoleBindings this reconciler owns.
var AutomatedLabels = map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"}

// Reconciler projects manage ClusterRoleBindings into use RoleBindings.
type Reconciler struct {
	client  client.Client
	metrics *metrics.Collector
	log     logr.Logger
}

// New constructs a Reconciler.
func New(c client.Client, m *metrics.Collector, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, metrics: m, log: log}
}

// RoleRefIndexValue is the indexer of RoleRefIndexField: the name of the referenced ClusterRole.
func RoleRefIndexValue(obj client.Object) []string {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok || crb.RoleRef.Kind != "ClusterRole" {
		return nil
	}
	return []string{crb.RoleRef.Name}
}

// IsManageBinding reports whether obj is a ClusterRoleBinding to a manage role, looked up through
// reader (the manager's cache, which holds the manage ClusterRoles only, so a miss means "not a
// manage role").
func IsManageBinding(reader client.Reader, obj client.Object) bool {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok || crb.RoleRef.Kind != "ClusterRole" {
		return false
	}
	role := &rbacv1.ClusterRole{}
	if err := reader.Get(context.Background(), client.ObjectKey{Name: crb.RoleRef.Name}, role); err != nil {
		return false
	}
	return role.Labels[LabelKind] == KindManage
}

// AutomatedIndexValue is the indexer of AutomatedIndexField.
func AutomatedIndexValue(obj client.Object) []string {
	if _, ok := obj.(*rbacv1.RoleBinding); !ok || !isAutomatedUseBinding(obj.GetLabels()) {
		return nil
	}
	return []string{automatedIndexValue}
}

// Register indexes the ClusterRoleBindings by roleRef and the automated use RoleBindings, and wires
// the reconciler onto the manager.
func Register(ctx context.Context, mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.ClusterRoleBinding{}, RoleRefIndexField, RoleRefIndexValue); err != nil {
		return fmt.Errorf("index clusterrolebindings by roleRef: %w", err)
	}
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.RoleBinding{}, AutomatedIndexField, AutomatedIndexValue); err != nil {
		return fmt.Errorf("index automated rolebindings: %w", err)
	}
	r := New(mgr.GetClient(), metrics.Default, mgr.GetLogger().WithName("manage-bindings"))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup manage-bindings controller: %w", err)
	}
	return nil
}

// SetupWithManager watches manage ClusterRoleBindings, manage ClusterRoles and the automated use
// RoleBindings; every event maps to the single request.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	single := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: RequestName}}}
	})

	manageBinding := eitherSide(func(obj client.Object) bool {
		return IsManageBinding(r.client, obj)
	})

	manageRole := eitherSide(func(obj client.Object) bool {
		return obj.GetLabels()[LabelKind] == KindManage
	})

	automatedUseBinding := eitherSide(func(obj client.Object) bool {
		return isAutomatedUseBinding(obj.GetLabels())
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("manage-bindings").
		Watches(&rbacv1.ClusterRoleBinding{}, single, builder.WithPredicates(manageBinding)).
		Watches(&rbacv1.ClusterRole{}, single, builder.WithPredicates(manageRole)).
		Watches(&rbacv1.RoleBinding{}, single, builder.WithPredicates(automatedUseBinding)).
		Complete(r)
}

// eitherSide builds a predicate from match that, on updates, passes when either object matches.
func eitherSide(match func(client.Object) bool) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return match(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return match(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return match(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return match(e.ObjectOld) || match(e.ObjectNew) },
	}
}

// Reconcile recomputes the expected use RoleBindings from the manage ClusterRoleBindings and makes
// the cluster match: missing ones are created, differing ones updated, unexpected automated ones
// deleted. Creation and update come before deletion, and nothing is deleted while an expected
// binding could not be written.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	roles := &rbacv1.ClusterRoleList{}
	if err := r.client.List(ctx, roles, client.MatchingLabels(ManageRoleLabels)); err != nil {
		return reconcile.Result{}, fmt.Errorf("list manage clusterroles: %w", err)
	}
	resolver := NewResolver(roles.Items)

	expected := make(map[client.ObjectKey]*rbacv1.RoleBinding)
	for i := range roles.Items {
		useRole, namespaces := resolver.UseRoleAndNamespaces(roles.Items[i].Name)
		if useRole == "" || len(namespaces) == 0 {
			continue
		}

		bindings := &rbacv1.ClusterRoleBindingList{}
		if err := r.client.List(ctx, bindings, client.MatchingFields{RoleRefIndexField: roles.Items[i].Name}); err != nil {
			return reconcile.Result{}, fmt.Errorf("list clusterrolebindings of %s: %w", roles.Items[i].Name, err)
		}
		for j := range bindings.Items {
			crb := &bindings.Items[j]
			for _, namespace := range namespaces {
				rb := UseBinding(crb, useRole, namespace)
				expected[client.ObjectKeyFromObject(rb)] = rb
			}
		}
	}

	var (
		errs  []error
		drift metrics.Drift
	)
	for _, key := range slices.SortedFunc(maps.Keys(expected), compareKeys) {
		if err := r.ensure(ctx, key, expected[key], &drift); err != nil {
			errs = append(errs, err)
		}
	}
	observe := func() {
		// drift is what is still wrong after the writes: projections that could not be written and
		// stale bindings that could not be removed.
		r.metrics.Observe(metrics.KindManage, RequestName, metrics.Observation{Desired: len(expected), Actual: len(expected) - drift.Missing - drift.Changed, Drift: drift})
	}
	if len(errs) != 0 {
		observe()
		return reconcile.Result{}, errors.Join(errs...)
	}

	automated := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, automated, client.MatchingFields{AutomatedIndexField: automatedIndexValue}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list automated rolebindings: %w", err)
	}
	for i := range automated.Items {
		rb := &automated.Items[i]
		if !isAutomatedUseBinding(rb.Labels) {
			continue
		}
		if _, ok := expected[client.ObjectKeyFromObject(rb)]; ok {
			continue
		}
		err := r.client.Delete(ctx, rb)
		if apierrors.IsNotFound(err) {
			err = nil
		}
		r.metrics.RecordApply(metrics.KindManage, metrics.OpDelete, err)
		if err != nil {
			drift.Extra++
			errs = append(errs, fmt.Errorf("delete use binding %s/%s: %w", rb.Namespace, rb.Name, err))
			continue
		}
		r.log.Info("use binding removed", "namespace", rb.Namespace, "name", rb.Name)
	}

	observe()
	return reconcile.Result{}, errors.Join(errs...)
}

func compareKeys(a, b client.ObjectKey) int {
	if c := strings.Compare(a.Namespace, b.Namespace); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// ensure makes the RoleBinding at key equal to want in everything the reconciler owns: its labels
// and annotation, roleRef and subjects. Foreign labels and annotations are preserved. roleRef is
// immutable, so a binding with a different one is recreated. drift counts what could not be fixed.
func (r *Reconciler) ensure(ctx context.Context, key client.ObjectKey, want *rbacv1.RoleBinding, drift *metrics.Drift) error {
	current := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		if err := r.create(ctx, key, want); err != nil {
			drift.Missing++
			return err
		}
		return nil
	}
	if err != nil {
		drift.Changed++
		return fmt.Errorf("get use binding %s: %w", key, err)
	}

	if !reflect.DeepEqual(current.RoleRef, want.RoleRef) {
		err := r.client.Delete(ctx, current)
		if apierrors.IsNotFound(err) {
			err = nil
		}
		r.metrics.RecordApply(metrics.KindManage, metrics.OpDelete, err)
		if err != nil {
			drift.Changed++
			return fmt.Errorf("delete use binding %s with a stale roleRef: %w", key, err)
		}
		r.log.Info("use binding recreated: roleRef changed", "namespace", key.Namespace, "name", key.Name, "from", current.RoleRef.Name, "to", want.RoleRef.Name)
		if err := r.create(ctx, key, want); err != nil {
			drift.Changed++
			return err
		}
		return nil
	}

	if hasEntries(current.Labels, want.Labels) && hasEntries(current.Annotations, want.Annotations) &&
		reflect.DeepEqual(current.Subjects, want.Subjects) {
		return nil
	}

	updated := current.DeepCopy()
	updated.Labels = merged(current.Labels, want.Labels)
	updated.Annotations = merged(current.Annotations, want.Annotations)
	updated.Subjects = want.Subjects
	err = r.client.Update(ctx, updated)
	r.metrics.RecordApply(metrics.KindManage, metrics.OpUpdate, err)
	if err != nil {
		drift.Changed++
		return fmt.Errorf("update use binding %s: %w", key, err)
	}
	r.log.Info("use binding updated", "namespace", key.Namespace, "name", key.Name)

	return nil
}

func (r *Reconciler) create(ctx context.Context, key client.ObjectKey, want *rbacv1.RoleBinding) error {
	err := r.client.Create(ctx, want)
	if apierrors.IsAlreadyExists(err) {
		err = nil
	}
	r.metrics.RecordApply(metrics.KindManage, metrics.OpCreate, err)
	if err != nil {
		return fmt.Errorf("create use binding %s: %w", key, err)
	}
	r.log.Info("use binding created", "namespace", key.Namespace, "name", key.Name)
	return nil
}

// hasEntries reports whether have contains every entry of want.
func hasEntries(have, want map[string]string) bool {
	for k, v := range want {
		if got, ok := have[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// merged returns have with every entry of want set.
func merged(have, want map[string]string) map[string]string {
	out := maps.Clone(have)
	if out == nil {
		out = make(map[string]string, len(want))
	}
	maps.Copy(out, want)
	return out
}

// Resolver answers, for a manage role, which use role it grants and in which namespaces, following
// the aggregation rules of the manage roles to any depth. Results are memoised per role.
type Resolver struct {
	roles      map[string]*rbacv1.ClusterRole
	selectors  map[string][]labels.Selector
	namespaces map[string][]string
}

// NewResolver indexes the manage ClusterRoles. Aggregation selectors that cannot be parsed are
// ignored, as the API server ignores them.
func NewResolver(roles []rbacv1.ClusterRole) *Resolver {
	r := &Resolver{
		roles:      make(map[string]*rbacv1.ClusterRole, len(roles)),
		selectors:  make(map[string][]labels.Selector, len(roles)),
		namespaces: make(map[string][]string),
	}
	for i := range roles {
		role := &roles[i]
		r.roles[role.Name] = role
		if role.AggregationRule == nil {
			continue
		}
		for j := range role.AggregationRule.ClusterRoleSelectors {
			selector, err := metav1.LabelSelectorAsSelector(&role.AggregationRule.ClusterRoleSelectors[j])
			if err != nil {
				continue
			}
			r.selectors[role.Name] = append(r.selectors[role.Name], selector)
		}
	}
	return r
}

// UseRoleAndNamespaces resolves the use role granted by the manage role roleName and the sorted
// namespaces it applies to. The empty use role means "not a manage role we project".
func (r *Resolver) UseRoleAndNamespaces(roleName string) (string, []string) {
	role, ok := r.roles[roleName]
	if !ok {
		return "", nil
	}
	useRole, ok := role.Labels[LabelUseRole]
	if !ok {
		return "", nil
	}

	if namespaces, done := r.namespaces[roleName]; done {
		return useRole, namespaces
	}

	set := make(map[string]struct{})
	r.collect(roleName, map[string]struct{}{roleName: {}}, set)
	namespaces := slices.Sorted(maps.Keys(set))
	r.namespaces[roleName] = namespaces

	return useRole, namespaces
}

// collect adds to set the namespaces of every role aggregated by roleName, directly or through
// other aggregating roles. visited guards against aggregation cycles.
func (r *Resolver) collect(roleName string, visited, set map[string]struct{}) {
	selectors := r.selectors[roleName]
	if len(selectors) == 0 {
		return
	}
	for name, candidate := range r.roles {
		if _, seen := visited[name]; seen {
			continue
		}
		if !matchesAny(selectors, candidate.Labels) {
			continue
		}
		visited[name] = struct{}{}
		if namespace, ok := candidate.Labels[LabelNamespace]; ok {
			set[namespace] = struct{}{}
		}
		r.collect(name, visited, set)
	}
}

func matchesAny(selectors []labels.Selector, roleLabels map[string]string) bool {
	set := labels.Set(roleLabels)
	for _, selector := range selectors {
		if selector.Matches(set) {
			return true
		}
	}
	return false
}

// UseBinding builds the namespaced projection of a manage ClusterRoleBinding.
func UseBinding(manage *rbacv1.ClusterRoleBinding, useRole, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "RoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("d8:use:%s:binding:%s", useRole, manage.Name),
			Namespace:   namespace,
			Labels:      maps.Clone(AutomatedLabels),
			Annotations: map[string]string{relatedWithAnnotation: manage.Name},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: useRolePrefix + useRole},
		Subjects: slices.Clone(manage.Subjects),
	}
}

// isAutomatedUseBinding matches the RoleBindings this reconciler owns: automated Deckhouse bindings
// that are not dict bindings (those are ClusterRoleBindings anyway, the check is defence in depth).
// The automated label is used by nothing else in Deckhouse, so every such RoleBinding is a
// projection of this reconciler or of the hook it replaced.
func isAutomatedUseBinding(objLabels map[string]string) bool {
	return objLabels[labelHeritage] == "deckhouse" && objLabels[labelAutomated] == "true" && objLabels[labelDict] != "true"
}
