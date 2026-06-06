package webhooks

import (
	"context"
	"fmt"
	"slices"

	"controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// policyWhitelist builds the set of granted resource names for an applicable policy:
// the explicit Allowed names, the Default, plus every name of the policy's granted
// resource whose labels match AllowedSelector (union semantics). Reads go through the
// (cached) controller client.
func policyWhitelist(
	ctx context.Context,
	cl client.Client,
	ap v1alpha1.ApplicablePolicy,
	policy *v1alpha1.ClusterObjectGrantPolicy,
) ([]string, error) {
	whitelist := slices.Clone(ap.Allowed)
	if ap.Default != "" && !slices.Contains(whitelist, ap.Default) {
		whitelist = append(whitelist, ap.Default)
	}

	if ap.AllowedSelector == nil {
		return whitelist, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(ap.AllowedSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid allowedSelector: %w", err)
	}

	gv, err := schema.ParseGroupVersion(policy.Spec.GrantedResource.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parse grantedResource apiVersion %q: %w", policy.Spec.GrantedResource.APIVersion, err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    policy.Spec.GrantedResource.Kind + "List",
	})
	if err := cl.List(ctx, list, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("list granted resource %s: %w", policy.Spec.GrantedResource.Kind, err)
	}

	for i := range list.Items {
		name := list.Items[i].GetName()
		if !slices.Contains(whitelist, name) {
			whitelist = append(whitelist, name)
		}
	}

	return whitelist, nil
}
