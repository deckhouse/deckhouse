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
// its rbac.deckhouse.io/namespace label (one level of nesting for subsystem roles aggregating
// module roles). For each manage ClusterRoleBinding and each such namespace a RoleBinding
// d8:use:<use-role>:binding:<binding> is kept; automated use RoleBindings that are no longer
// expected are removed. The whole set is recomputed on every change (single request key).
package managebindings

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// RequestName is the constant key every event is mapped to.
	RequestName = "manage-bindings"

	LabelKind      = "rbac.deckhouse.io/kind"
	LabelUseRole   = "rbac.deckhouse.io/use-role"
	LabelNamespace = "rbac.deckhouse.io/namespace"
	labelHeritage  = "heritage"
	labelAutomated = "rbac.deckhouse.io/automated"
	labelDict      = "rbac.deckhouse.io/dict"

	KindManage = "manage"

	manageRolePrefix     = "d8:manage:"
	useRolePrefix        = "d8:use:role:"
	relatedWithAnnotatio = "rbac.deckhouse.io/related-with"
)

// ManageRoleLabels select the manage ClusterRoles.
var ManageRoleLabels = map[string]string{LabelKind: KindManage}

// AutomatedLabels mark the use RoleBindings this reconciler owns.
var AutomatedLabels = map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"}

// Reconciler projects manage ClusterRoleBindings into use RoleBindings.
type Reconciler struct {
	client client.Client
	log    logr.Logger
}

// New constructs a Reconciler.
func New(c client.Client, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, log: log}
}

// Register wires the reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("manage-bindings"))
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

	manageBinding := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		crb, ok := obj.(*rbacv1.ClusterRoleBinding)
		return ok && crb.RoleRef.Kind == "ClusterRole" && strings.HasPrefix(crb.RoleRef.Name, manageRolePrefix)
	})

	manageRole := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()[LabelKind] == KindManage
	})

	automatedUseBinding := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return isAutomatedUseBinding(obj.GetLabels())
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("manage-bindings").
		Watches(&rbacv1.ClusterRoleBinding{}, single, builder.WithPredicates(manageBinding)).
		Watches(&rbacv1.ClusterRole{}, single, builder.WithPredicates(manageRole)).
		Watches(&rbacv1.RoleBinding{}, single, builder.WithPredicates(automatedUseBinding)).
		Complete(r)
}

// Reconcile recomputes the expected use RoleBindings from the manage ClusterRoleBindings and makes
// the cluster match: missing ones are created, differing ones updated, unexpected automated ones
// deleted. Creation and update come before deletion.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	roles := &rbacv1.ClusterRoleList{}
	if err := r.client.List(ctx, roles, client.MatchingLabels(ManageRoleLabels)); err != nil {
		return reconcile.Result{}, fmt.Errorf("list manage clusterroles: %w", err)
	}
	manageRoles := make(map[string]*rbacv1.ClusterRole, len(roles.Items))
	for i := range roles.Items {
		manageRoles[roles.Items[i].Name] = &roles.Items[i]
	}

	bindings := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, bindings); err != nil {
		return reconcile.Result{}, fmt.Errorf("list clusterrolebindings: %w", err)
	}

	expected := make(map[client.ObjectKey]*rbacv1.RoleBinding)
	for i := range bindings.Items {
		crb := &bindings.Items[i]
		if crb.RoleRef.Kind != "ClusterRole" {
			continue
		}

		useRole, namespaces := UseRoleAndNamespaces(manageRoles, crb.RoleRef.Name)
		if useRole == "" {
			continue
		}

		for namespace := range namespaces {
			rb := UseBinding(crb, useRole, namespace)
			expected[client.ObjectKeyFromObject(rb)] = rb
		}
	}

	for key, want := range expected {
		if err := r.ensure(ctx, key, want); err != nil {
			return reconcile.Result{}, err
		}
	}

	automated := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, automated, client.MatchingLabels(AutomatedLabels)); err != nil {
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
		if err := r.client.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("delete use binding %s/%s: %w", rb.Namespace, rb.Name, err)
		}
		r.log.Info("use binding removed", "namespace", rb.Namespace, "name", rb.Name)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) ensure(ctx context.Context, key client.ObjectKey, want *rbacv1.RoleBinding) error {
	current := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		if err := r.client.Create(ctx, want); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create use binding %s: %w", key, err)
		}
		r.log.Info("use binding created", "namespace", key.Namespace, "name", key.Name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get use binding %s: %w", key, err)
	}

	if reflect.DeepEqual(current.Labels, want.Labels) &&
		reflect.DeepEqual(current.Annotations, want.Annotations) &&
		reflect.DeepEqual(current.RoleRef, want.RoleRef) &&
		reflect.DeepEqual(current.Subjects, want.Subjects) {
		return nil
	}

	updated := current.DeepCopy()
	updated.Labels = want.Labels
	updated.Annotations = want.Annotations
	updated.RoleRef = want.RoleRef
	updated.Subjects = want.Subjects
	if err := r.client.Update(ctx, updated); err != nil {
		return fmt.Errorf("update use binding %s: %w", key, err)
	}
	r.log.Info("use binding updated", "namespace", key.Namespace, "name", key.Name)

	return nil
}

// UseRoleAndNamespaces resolves the use role granted by the manage role roleName and the
// namespaces it applies to. The empty use role means "not a manage role we project".
func UseRoleAndNamespaces(manageRoles map[string]*rbacv1.ClusterRole, roleName string) (string, map[string]struct{}) {
	role, ok := manageRoles[roleName]
	if !ok {
		return "", nil
	}
	useRole, ok := role.Labels[LabelUseRole]
	if !ok {
		return "", nil
	}

	namespaces := make(map[string]struct{})
	for _, candidate := range manageRoles {
		if !matchesAggregation(role.AggregationRule, candidate.Labels) {
			continue
		}

		if candidate.AggregationRule == nil {
			if namespace, ok := candidate.Labels[LabelNamespace]; ok {
				namespaces[namespace] = struct{}{}
			}
			continue
		}

		// A subsystem role aggregating module roles: one more level down.
		for _, nested := range manageRoles {
			if !matchesAggregation(candidate.AggregationRule, nested.Labels) {
				continue
			}
			if namespace, ok := nested.Labels[LabelNamespace]; ok {
				namespaces[namespace] = struct{}{}
			}
		}
	}

	return useRole, namespaces
}

func matchesAggregation(rule *rbacv1.AggregationRule, roleLabels map[string]string) bool {
	if rule == nil {
		return false
	}
	for _, selector := range rule.ClusterRoleSelectors {
		if selector.MatchLabels == nil {
			continue
		}
		if labels.SelectorFromSet(selector.MatchLabels).Matches(labels.Set(roleLabels)) {
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
			Labels:      map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"},
			Annotations: map[string]string{relatedWithAnnotatio: manage.Name},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: useRolePrefix + useRole},
		Subjects: manage.Subjects,
	}
}

// isAutomatedUseBinding matches the RoleBindings this reconciler owns: automated Deckhouse bindings
// that are not dict bindings (those are ClusterRoleBindings anyway, the check is defence in depth).
func isAutomatedUseBinding(objLabels map[string]string) bool {
	return objLabels[labelHeritage] == "deckhouse" && objLabels[labelAutomated] == "true" && objLabels[labelDict] != "true"
}
