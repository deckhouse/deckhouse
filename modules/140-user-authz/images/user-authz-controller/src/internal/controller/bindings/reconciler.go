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

// Package bindings reconciles the (Cluster)RoleBindings of ClusterAuthorizationRules and
// AuthorizationRules.
//
// For every rule the controller computes the desired bindings (package desired) and makes the
// cluster match: missing bindings are created, differing ones are updated in place, and bindings
// of the rule that are no longer desired are deleted. Only objects carrying the module labels
// (heritage=deckhouse, module=user-authz) and named user-authz:<rule>:* are considered ours; that
// covers the bindings the Helm chart used to render, so they are adopted in place, and the
// per-custom-role bindings of older versions, which are removed once the aggregated binding is
// applied (creation always happens before deletion within one reconcile).
package bindings

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
	"user-authz-controller/internal/desired"
)

const (
	// ConditionReady is the single condition the controller maintains on a rule.
	ConditionReady = "Ready"

	ReasonBindingsApplied = "BindingsApplied"
	ReasonInvalidSpec     = "InvalidSpec"
	ReasonApplyError      = "ApplyError"

	// DefaultMaxConcurrentReconciles bounds parallel reconciles; each is a handful of cached reads
	// and at most a few writes, so a burst of rule changes is absorbed quickly.
	DefaultMaxConcurrentReconciles = 4
)

// ModuleLabels select every binding the module ever created, whether by Helm or by the controller.
var ModuleLabels = client.MatchingLabels{
	desired.LabelHeritage: desired.HeritageValue,
	desired.LabelModule:   desired.ModuleName,
}

// Options tunes a Reconciler.
type Options struct {
	MaxConcurrentReconciles int
}

// Reconciler reconciles bindings of one rule kind: cluster-scoped (ClusterAuthorizationRule →
// ClusterRoleBinding) or namespaced (AuthorizationRule → RoleBinding).
type Reconciler struct {
	client     client.Client
	recorder   events.EventRecorder
	log        logr.Logger
	namespaced bool
}

// NewCluster constructs the ClusterAuthorizationRule reconciler.
func NewCluster(c client.Client, recorder events.EventRecorder, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, recorder: recorder, log: log, namespaced: false}
}

// NewNamespaced constructs the AuthorizationRule reconciler.
func NewNamespaced(c client.Client, recorder events.EventRecorder, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, recorder: recorder, log: log, namespaced: true}
}

// Register wires both reconcilers onto the manager.
func Register(mgr manager.Manager, opts Options) error {
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = DefaultMaxConcurrentReconciles
	}

	cluster := NewCluster(mgr.GetClient(), mgr.GetEventRecorder("user-authz-controller"), mgr.GetLogger().WithName("clusterauthorizationrule"))
	if err := cluster.SetupWithManager(mgr, opts); err != nil {
		return fmt.Errorf("setup clusterauthorizationrule controller: %w", err)
	}

	namespaced := NewNamespaced(mgr.GetClient(), mgr.GetEventRecorder("user-authz-controller"), mgr.GetLogger().WithName("authorizationrule"))
	if err := namespaced.SetupWithManager(mgr, opts); err != nil {
		return fmt.Errorf("setup authorizationrule controller: %w", err)
	}

	return nil
}

// SetupWithManager registers the watches: the rule kind itself and the bindings carrying the
// module labels, mapped back to their rule so that drift and leftovers are repaired.
func (r *Reconciler) SetupWithManager(mgr manager.Manager, opts Options) error {
	ours := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		labels := obj.GetLabels()
		if labels[desired.LabelHeritage] != desired.HeritageValue || labels[desired.LabelModule] != desired.ModuleName {
			return false
		}
		_, ok := desired.RuleNameOf(obj.GetName())
		return ok
	})

	mapBinding := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		name, ok := desired.RuleNameOf(obj.GetName())
		if !ok {
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: name, Namespace: obj.GetNamespace()}}}
	})

	if r.namespaced {
		return ctrl.NewControllerManagedBy(mgr).
			Named("authorizationrule-bindings").
			WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
			For(&v1alpha1.AuthorizationRule{}).
			Watches(&rbacv1.RoleBinding{}, mapBinding, builder.WithPredicates(ours)).
			Complete(r)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("clusterauthorizationrule-bindings").
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		For(&v1.ClusterAuthorizationRule{}).
		Watches(&rbacv1.ClusterRoleBinding{}, mapBinding, builder.WithPredicates(ours)).
		Complete(r)
}

// Reconcile brings the bindings of the rule named in req to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	rule, obj, found, err := r.getRule(ctx, req.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}

	existing, err := r.existingBindings(ctx, req.Name, req.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !found {
		// The rule is gone: whatever is left with its prefix is a leftover (ownerReferences take
		// care of controller-created bindings, this covers the ones inherited from the chart).
		return reconcile.Result{}, r.deleteAll(ctx, existing)
	}

	wanted, err := desired.Bindings(rule)
	if errors.Is(err, desired.ErrInvalidSpec) {
		r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, ReasonInvalidSpec, "Reconcile", "%s", err.Error())
		return reconcile.Result{}, r.setStatus(ctx, obj, rule, metav1.ConditionFalse, ReasonInvalidSpec, err.Error(), int32(len(existing)))
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.apply(ctx, rule, wanted, existing); err != nil {
		if statusErr := r.setStatus(ctx, obj, rule, metav1.ConditionFalse, ReasonApplyError, err.Error(), int32(len(wanted))); statusErr != nil {
			r.log.Error(statusErr, "set status after apply error", "rule", req.NamespacedName)
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.setStatus(ctx, obj, rule, metav1.ConditionTrue, ReasonBindingsApplied, fmt.Sprintf("%d bindings applied", len(wanted)), int32(len(wanted)))
}

// getRule loads the rule of the reconciler's kind. found is false when it does not exist.
func (r *Reconciler) getRule(ctx context.Context, key client.ObjectKey) (desired.Rule, client.Object, bool, error) {
	if r.namespaced {
		ar := &v1alpha1.AuthorizationRule{}
		if err := r.client.Get(ctx, key, ar); err != nil {
			if apierrors.IsNotFound(err) {
				return desired.Rule{}, nil, false, nil
			}
			return desired.Rule{}, nil, false, fmt.Errorf("get authorizationrule %s: %w", key, err)
		}
		return desired.FromAuthorizationRule(ar), ar, true, nil
	}

	car := &v1.ClusterAuthorizationRule{}
	if err := r.client.Get(ctx, key, car); err != nil {
		if apierrors.IsNotFound(err) {
			return desired.Rule{}, nil, false, nil
		}
		return desired.Rule{}, nil, false, fmt.Errorf("get clusterauthorizationrule %s: %w", key, err)
	}
	return desired.FromClusterAuthorizationRule(car), car, true, nil
}

// existingBindings lists the module-labeled bindings of the rule (by name prefix) in its scope.
func (r *Reconciler) existingBindings(ctx context.Context, ruleName, namespace string) ([]client.Object, error) {
	prefix := desired.RulePrefix(ruleName)
	var out []client.Object

	if r.namespaced {
		list := &rbacv1.RoleBindingList{}
		if err := r.client.List(ctx, list, client.InNamespace(namespace), ModuleLabels); err != nil {
			return nil, fmt.Errorf("list rolebindings in %s: %w", namespace, err)
		}
		for i := range list.Items {
			if strings.HasPrefix(list.Items[i].Name, prefix) {
				out = append(out, &list.Items[i])
			}
		}
		return out, nil
	}

	list := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, list, ModuleLabels); err != nil {
		return nil, fmt.Errorf("list clusterrolebindings: %w", err)
	}
	for i := range list.Items {
		if strings.HasPrefix(list.Items[i].Name, prefix) {
			out = append(out, &list.Items[i])
		}
	}
	return out, nil
}

// apply creates or updates every wanted binding, then deletes the existing ones that are not
// wanted. Creation first: an old binding is only removed once its replacement is in place.
func (r *Reconciler) apply(ctx context.Context, rule desired.Rule, wanted []desired.Binding, existing []client.Object) error {
	owner := desired.OwnerReference(rule)

	byName := make(map[string]client.Object, len(existing))
	for _, obj := range existing {
		byName[obj.GetName()] = obj
	}

	keep := make(map[string]struct{}, len(wanted))
	for _, b := range wanted {
		keep[b.Name] = struct{}{}

		var want client.Object
		if r.namespaced {
			want = desired.RoleBinding(b, owner)
		} else {
			want = desired.ClusterRoleBinding(b, owner)
		}

		current, exists := byName[b.Name]
		if !exists {
			if err := r.client.Create(ctx, want); err != nil {
				return fmt.Errorf("create %s: %w", b.Name, err)
			}
			r.log.V(1).Info("binding created", "name", b.Name, "namespace", b.Namespace)
			continue
		}

		if !needsUpdate(current, want) {
			continue
		}

		updated := merge(current, want)
		if err := r.client.Update(ctx, updated); err != nil {
			return fmt.Errorf("update %s: %w", b.Name, err)
		}
		r.log.V(1).Info("binding updated", "name", b.Name, "namespace", b.Namespace)
	}

	for _, obj := range existing {
		if _, wantedStill := keep[obj.GetName()]; wantedStill {
			continue
		}
		if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s: %w", obj.GetName(), err)
		}
		r.log.Info("binding removed", "name", obj.GetName(), "namespace", obj.GetNamespace())
	}

	return nil
}

func (r *Reconciler) deleteAll(ctx context.Context, existing []client.Object) error {
	for _, obj := range existing {
		if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s: %w", obj.GetName(), err)
		}
		r.log.Info("binding of a deleted rule removed", "name", obj.GetName(), "namespace", obj.GetNamespace())
	}
	return nil
}

// needsUpdate reports whether current differs from want in anything the controller owns: roleRef,
// subjects, labels, owner references, or leftover annotations (Helm ownership, resource-policy).
func needsUpdate(current, want client.Object) bool {
	if !reflect.DeepEqual(current.GetLabels(), want.GetLabels()) {
		return true
	}
	if len(current.GetAnnotations()) != 0 {
		return true
	}
	if !reflect.DeepEqual(current.GetOwnerReferences(), want.GetOwnerReferences()) {
		return true
	}

	switch c := current.(type) {
	case *rbacv1.ClusterRoleBinding:
		w := want.(*rbacv1.ClusterRoleBinding)
		return !reflect.DeepEqual(c.RoleRef, w.RoleRef) || !reflect.DeepEqual(c.Subjects, w.Subjects)
	case *rbacv1.RoleBinding:
		w := want.(*rbacv1.RoleBinding)
		return !reflect.DeepEqual(c.RoleRef, w.RoleRef) || !reflect.DeepEqual(c.Subjects, w.Subjects)
	default:
		return true
	}
}

// merge returns current with everything the controller owns replaced by want. Annotations are
// dropped entirely: the only ones these objects ever had were Helm ownership markers and
// helm.sh/resource-policy, both obsolete once the controller owns the object.
//
// roleRef is immutable on bindings; a differing roleRef is surfaced as an Update error by the
// API server and reported in the rule status rather than worked around by recreation.
func merge(current, want client.Object) client.Object {
	updated := current.DeepCopyObject().(client.Object)
	updated.SetLabels(want.GetLabels())
	updated.SetAnnotations(nil)
	updated.SetOwnerReferences(want.GetOwnerReferences())

	switch u := updated.(type) {
	case *rbacv1.ClusterRoleBinding:
		w := want.(*rbacv1.ClusterRoleBinding)
		u.RoleRef = w.RoleRef
		u.Subjects = w.Subjects
	case *rbacv1.RoleBinding:
		w := want.(*rbacv1.RoleBinding)
		u.RoleRef = w.RoleRef
		u.Subjects = w.Subjects
	}

	return updated
}

// setStatus writes the Ready condition, the binding count and the observed generation to the
// rule, skipping the write when nothing changed.
func (r *Reconciler) setStatus(ctx context.Context, obj client.Object, rule desired.Rule, status metav1.ConditionStatus, reason, message string, bindings int32) error {
	var current *v1.RuleStatus
	switch o := obj.(type) {
	case *v1.ClusterAuthorizationRule:
		current = &o.Status
	case *v1alpha1.AuthorizationRule:
		current = &o.Status
	default:
		return fmt.Errorf("unexpected rule type %T", obj)
	}

	before := obj.DeepCopyObject().(client.Object)

	changed := apimeta.SetStatusCondition(&current.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: rule.Generation,
	})
	if current.Bindings != bindings {
		current.Bindings = bindings
		changed = true
	}
	if current.ObservedGeneration != rule.Generation {
		current.ObservedGeneration = rule.Generation
		changed = true
	}
	if !changed {
		return nil
	}

	if err := r.client.Status().Patch(ctx, obj, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch status of %s/%s: %w", rule.Namespace, rule.Name, err)
	}
	return nil
}
