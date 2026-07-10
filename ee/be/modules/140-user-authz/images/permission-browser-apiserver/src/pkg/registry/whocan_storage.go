/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
)

// WhoCanResolver resolves the subjects allowed to perform a given action.
// It is implemented by *rbacadapter.RBACAuthorizer. The returned error is
// non-fatal: a partial result may accompany it (see rbacadapter.WhoCan).
type WhoCanResolver interface {
	WhoCan(ctx context.Context, attrs authorizer.Attributes) (rbacadapter.WhoCanResult, error)
}

// WhoCanStorage implements the REST storage for WhoCan.
// WhoCan is an ephemeral, create-only, cluster-scoped resource that answers the
// reverse-RBAC question ("who can perform this action?").
type WhoCanStorage struct {
	resolver WhoCanResolver
}

// NewWhoCanStorage creates a new WhoCanStorage.
func NewWhoCanStorage(resolver WhoCanResolver) *WhoCanStorage {
	return &WhoCanStorage{
		resolver: resolver,
	}
}

//nolint:misspell // Creater is the correct interface name in k8s.io/apiserver
var _ rest.Creater = &WhoCanStorage{}
var _ rest.Scoper = &WhoCanStorage{}
var _ rest.Storage = &WhoCanStorage{}
var _ rest.SingularNameProvider = &WhoCanStorage{}

// New returns a new WhoCan.
func (s *WhoCanStorage) New() runtime.Object {
	return &v1alpha1.WhoCan{}
}

// Destroy cleans up resources on shutdown.
func (s *WhoCanStorage) Destroy() {}

// NamespaceScoped returns false because WhoCan is cluster-scoped (the target
// namespace, if any, is carried in spec.resourceAttributes.namespace).
func (s *WhoCanStorage) NamespaceScoped() bool {
	return false
}

// GetSingularName returns the singular name of the resource.
func (s *WhoCanStorage) GetSingularName() string {
	return "whocan"
}

// Create resolves the subjects allowed to perform the requested action.
func (s *WhoCanStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	wc, ok := obj.(*v1alpha1.WhoCan)
	if !ok {
		return nil, apierrors.NewBadRequest("object is not a WhoCan")
	}

	if wc.Spec.ResourceAttributes == nil && wc.Spec.NonResourceAttributes == nil {
		return nil, apierrors.NewBadRequest("spec must set either resourceAttributes or nonResourceAttributes")
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	// The "user" is irrelevant for reverse RBAC resolution: we match rules
	// against the requested action, not against a particular subject.
	attrs := &accessAttributes{
		user:                  &userInfo{},
		resourceAttributes:    wc.Spec.ResourceAttributes,
		nonResourceAttributes: wc.Spec.NonResourceAttributes,
	}

	result, err := s.resolver.WhoCan(ctx, attrs)

	wc.Status = v1alpha1.WhoCanStatus{
		Users:           result.Users,
		Groups:          result.Groups,
		ServiceAccounts: toServiceAccountReferences(result.ServiceAccounts),
	}
	if err != nil {
		// Resolution failures are non-fatal: the subjects above are whatever
		// could be resolved before the failure. Surface the error in the status
		// so a failed informer list is distinguishable from "nobody can".
		wc.Status.EvaluationError = err.Error()
		klog.Warningf("WhoCan: partial result for verb=%q resource=%q namespace=%q: %v",
			attrs.GetVerb(), attrs.GetResource(), attrs.GetNamespace(), err)
	}

	klog.V(4).Infof("WhoCan: verb=%q resource=%q namespace=%q -> %d users, %d groups, %d serviceaccounts",
		attrs.GetVerb(), attrs.GetResource(), attrs.GetNamespace(),
		len(wc.Status.Users), len(wc.Status.Groups), len(wc.Status.ServiceAccounts))

	return wc, nil
}

// toServiceAccountReferences maps engine results to API types.
func toServiceAccountReferences(refs []rbacadapter.ServiceAccountRef) []v1alpha1.ServiceAccountReference {
	if len(refs) == 0 {
		return nil
	}
	out := make([]v1alpha1.ServiceAccountReference, len(refs))
	for i, ref := range refs {
		out[i] = v1alpha1.ServiceAccountReference{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}
	}
	return out
}
