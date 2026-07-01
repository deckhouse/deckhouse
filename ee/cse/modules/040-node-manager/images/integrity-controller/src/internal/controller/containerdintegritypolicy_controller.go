/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

// ContainerdIntegrityPolicyReconciler reconciles a ContainerdIntegrityPolicy object.
type ContainerdIntegrityPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *ContainerdIntegrityPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	policy := &deckhousev1alpha1.ContainerdIntegrityPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ContainerdIntegrityPolicy: %w", err)
	}

	matchedNamespaces, err := r.listMatchingNamespaces(ctx, policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list matching namespaces: %w", err)
	}

	if slices.Equal(policy.Status.ProtectedNamespaces, matchedNamespaces) {
		return ctrl.Result{}, nil
	}

	policy.Status.ProtectedNamespaces = matchedNamespaces
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ContainerdIntegrityPolicy status: %w", err)
	}

	logger.Info("Updated protected namespaces", "count", len(matchedNamespaces), "namespaces", matchedNamespaces)

	return ctrl.Result{}, nil
}

func (r *ContainerdIntegrityPolicyReconciler) listMatchingNamespaces(
	ctx context.Context,
	policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
) ([]string, error) {
	selector := labels.SelectorFromSet(policy.Spec.ProtectedNamespaces.MatchLabels)

	namespaceList := &corev1.NamespaceList{}
	if err := r.List(ctx, namespaceList); err != nil {
		return nil, err
	}

	matchedNamespaces := make([]string, 0)
	for i := range namespaceList.Items {
		namespace := &namespaceList.Items[i]
		if selector.Matches(labels.Set(namespace.Labels)) {
			matchedNamespaces = append(matchedNamespaces, namespace.Name)
		}
	}

	sort.Strings(matchedNamespaces)

	return matchedNamespaces, nil
}

func (r *ContainerdIntegrityPolicyReconciler) enqueueAllPolicies(
	ctx context.Context,
	_ client.Object,
) []reconcile.Request {
	policyList := &deckhousev1alpha1.ContainerdIntegrityPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list ContainerdIntegrityPolicies on namespace watch")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(policyList.Items))
	for i := range policyList.Items {
		policy := &policyList.Items[i]
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(policy),
		})
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ContainerdIntegrityPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhousev1alpha1.ContainerdIntegrityPolicy{}).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllPolicies),
		).
		Complete(r)
}
