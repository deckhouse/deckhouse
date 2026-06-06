package webhooks

import (
	"context"
	"fmt"

	"controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// applicableGrants returns every ClusterObjectGrant whose projectSelector matches the
// labels of the given project namespace. A missing namespace or a grant with a nil
// selector yields no match (nothing to enforce). Grants with an invalid selector are
// skipped. Shared by the validating and defaulting webhooks so their grant-matching
// semantics cannot drift.
func applicableGrants(
	ctx context.Context,
	cl client.Client,
	namespace string,
) ([]*v1alpha1.ClusterObjectGrant, error) {
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get namespace %s: %w", namespace, err)
	}

	grantList := &v1alpha1.ClusterObjectGrantList{}
	if err := cl.List(ctx, grantList); err != nil {
		return nil, fmt.Errorf("list ClusterObjectGrants: %w", err)
	}

	nsLabels := labels.Set(ns.Labels)
	out := make([]*v1alpha1.ClusterObjectGrant, 0)
	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if grant.Spec.ProjectSelector == nil {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(grant.Spec.ProjectSelector)
		if err != nil {
			continue
		}
		if selector.Matches(nsLabels) {
			out = append(out, grant)
		}
	}

	return out, nil
}
