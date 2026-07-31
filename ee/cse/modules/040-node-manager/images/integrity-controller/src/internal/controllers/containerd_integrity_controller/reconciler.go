/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegritycontroller

import (
	"context"
	"fmt"
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

var _ reconcile.Reconciler = (*reconciler)(nil)

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	policy, err := r.getContainerdIntegrityPolicy(ctx, req.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get ContainerdIntegrityPolicy: %w", err)
	}

	return r.reconcileProtectedNamespaces(ctx, policy)
}

func (r *reconciler) reconcileProtectedNamespaces(
	ctx context.Context,
	policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
) (reconcile.Result, error) {
	matchedNamespaces, err := r.listMatchingNamespaceNames(ctx, policy)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("list matching namespaces: %w", err)
	}

	if isProtectedNamespacesInSync(policy, matchedNamespaces) {
		log.FromContext(ctx).Info("Protected namespaces status is in sync")
		return reconcile.Result{}, nil
	}

	base := policy.DeepCopy()
	applyProtectedNamespacesStatus(policy, matchedNamespaces)

	if err := r.patchContainerdIntegrityPolicyStatus(ctx, base, policy); err != nil {
		return reconcile.Result{}, fmt.Errorf("patch ContainerdIntegrityPolicy status: %w", err)
	}

	log.FromContext(ctx).Info(
		"Patched protected namespaces status",
		"namespaces", matchedNamespaces,
	)

	return reconcile.Result{}, nil
}

func isProtectedNamespacesInSync(
	policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
	matchedNamespaces []string,
) bool {
	return slices.Equal(policy.Status.ProtectedNamespaces, matchedNamespaces)
}

func applyProtectedNamespacesStatus(
	policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
	matchedNamespaces []string,
) {
	policy.Status.ProtectedNamespaces = matchedNamespaces
}

func (r *reconciler) listMatchingNamespaceNames(
	ctx context.Context,
	policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
) ([]string, error) {
	namespaces, err := r.listNamespacesByLabels(ctx, policy.Spec.ProtectedNamespaces.MatchLabels)
	if err != nil {
		return nil, err
	}

	return extractNamespaceNames(namespaces), nil
}

func extractNamespaceNames(namespaces []corev1.Namespace) []string {
	names := make([]string, len(namespaces))
	for i := range namespaces {
		names[i] = namespaces[i].Name
	}

	sort.Strings(names)

	return names
}

// Kubernetes I/O helpers.

func (r *reconciler) getContainerdIntegrityPolicy(
	ctx context.Context,
	name string,
) (*deckhousev1alpha1.ContainerdIntegrityPolicy, error) {
	policy := &deckhousev1alpha1.ContainerdIntegrityPolicy{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, policy)
	return policy, err
}

func (r *reconciler) listNamespacesByLabels(
	ctx context.Context,
	matchLabels map[string]string,
) ([]corev1.Namespace, error) {
	namespaceList := &corev1.NamespaceList{}
	if err := r.client.List(ctx, namespaceList, client.MatchingLabels(matchLabels)); err != nil {
		return nil, err
	}

	return namespaceList.Items, nil
}

func (r *reconciler) patchContainerdIntegrityPolicyStatus(
	ctx context.Context,
	base, policy *deckhousev1alpha1.ContainerdIntegrityPolicy,
) error {
	return r.client.Status().Patch(ctx, policy, client.MergeFrom(base))
}
