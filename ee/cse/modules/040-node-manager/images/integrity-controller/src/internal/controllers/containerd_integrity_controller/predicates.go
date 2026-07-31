/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegritycontroller

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

func namespacePredicate() predicate.Predicate {
	return predicate.Or(
		predicate.LabelChangedPredicate{},
		predicate.GenerationChangedPredicate{},
	)
}

func (r *reconciler) mapNamespaceToContainerdIntegrityPolicies(
	ctx context.Context,
	obj client.Object,
) []reconcile.Request {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	policyList := &deckhousev1alpha1.ContainerdIntegrityPolicyList{}
	if err := r.client.List(ctx, policyList); err != nil {
		log.FromContext(ctx).Error(err, "list ContainerdIntegrityPolicies")
		return nil
	}

	namespaceLabels := labels.Set(namespace.Labels)
	requests := make([]reconcile.Request, 0, len(policyList.Items))
	for i := range policyList.Items {
		policy := &policyList.Items[i]
		selector := labels.SelectorFromSet(policy.Spec.ProtectedNamespaces.MatchLabels)
		if selector.Matches(namespaceLabels) ||
			slices.Contains(policy.Status.ProtectedNamespaces, namespace.Name) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(policy),
			})
		}
	}

	return requests
}
