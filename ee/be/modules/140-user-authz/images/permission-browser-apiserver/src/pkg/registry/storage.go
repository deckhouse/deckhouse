/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

const (
	// maxBulkSARRequests bounds per-request CPU work independently from the
	// generic apiserver's byte-size limit. Existing console clients submit one
	// request containing a comparatively small permission matrix.
	maxBulkSARRequests       = 10_000
	nonSelfReviewSubresource = "nonself"
)

// GetStorage returns the storage map for the authorization API group (legacy, without namespace resolver)
func GetStorage(auth authorizer.Authorizer) map[string]rest.Storage {
	return map[string]rest.Storage{
		"bulksubjectaccessreviews": NewBulkSARStorage(auth),
	}
}

// GetStorageWithResolver returns the storage map including the AccessibleNamespace resource.
// This requires a NamespaceResolver for resolving user-accessible namespaces.
func GetStorageWithResolver(auth authorizer.Authorizer, nsResolver *resolver.NamespaceResolver) map[string]rest.Storage {
	return map[string]rest.Storage{
		"bulksubjectaccessreviews": NewBulkSARStorage(auth),
		"accessiblenamespaces":     NewAccessibleNamespaceStorage(nsResolver),
	}
}

// BulkSARStorage implements the REST storage for BulkSubjectAccessReview
type BulkSARStorage struct {
	authorizer authorizer.Authorizer
}

// NewBulkSARStorage creates a new BulkSARStorage
func NewBulkSARStorage(auth authorizer.Authorizer) *BulkSARStorage {
	return &BulkSARStorage{
		authorizer: auth,
	}
}

//nolint:misspell // Creater is the correct interface name in k8s.io/apiserver
var _ rest.Creater = &BulkSARStorage{}
var _ rest.Scoper = &BulkSARStorage{}
var _ rest.Storage = &BulkSARStorage{}

// New returns a new BulkSubjectAccessReview
func (s *BulkSARStorage) New() runtime.Object {
	return &v1alpha1.BulkSubjectAccessReview{}
}

// Destroy cleans up resources on shutdown
func (s *BulkSARStorage) Destroy() {}

// NamespaceScoped returns false because BulkSubjectAccessReview is cluster-scoped
func (s *BulkSARStorage) NamespaceScoped() bool {
	return false
}

// GetSingularName returns the singular name of the resource
func (s *BulkSARStorage) GetSingularName() string {
	return "bulksubjectaccessreview"
}

// Create handles the creation of a BulkSubjectAccessReview (which evaluates all requests)
func (s *BulkSARStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	bsar, ok := obj.(*v1alpha1.BulkSubjectAccessReview)
	if !ok {
		return nil, apierrors.NewBadRequest("object is not a BulkSubjectAccessReview")
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}
	if len(bsar.Spec.Requests) > maxBulkSARRequests {
		return nil, apierrors.NewBadRequest(
			fmt.Sprintf("spec.requests must contain no more than %d items", maxBulkSARRequests),
		)
	}

	// Get the authenticated user from context
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		// The generic apiserver always populates the user; its absence is a
		// server-side invariant violation, not a client input error.
		return nil, apierrors.NewInternalError(fmt.Errorf("no user info in context"))
	}

	// Resolve subject: if spec.user is set, use non-self mode; otherwise use self mode
	var subjectUser string
	var subjectUID string
	var subjectGroups []string
	var subjectExtra map[string][]string

	if bsar.Spec.User != "" {
		if err := s.authorizeNonSelfReview(ctx, userInfo); err != nil {
			return nil, err
		}
		// Non-self mode: use the provided subject
		subjectUser = bsar.Spec.User
		subjectUID = bsar.Spec.UID
		subjectGroups = bsar.Spec.Groups
		subjectExtra = make(map[string][]string)
		for k, v := range bsar.Spec.Extra {
			subjectExtra[k] = []string(v)
		}
		klog.V(4).Infof("Non-self mode: checking access for user=%s, groups=%v", subjectUser, subjectGroups)
	} else {
		// Self mode: use the authenticated user
		subjectUser = userInfo.GetName()
		subjectUID = userInfo.GetUID()
		subjectGroups = userInfo.GetGroups()
		subjectExtra = userInfo.GetExtra()
		klog.V(4).Infof("Self mode: checking access for user=%s, groups=%v", subjectUser, subjectGroups)
	}

	// Process all requests
	results := make([]v1alpha1.SubjectAccessReviewResult, len(bsar.Spec.Requests))

	for i, req := range bsar.Spec.Requests {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attrs := s.buildAttributes(subjectUser, subjectUID, subjectGroups, subjectExtra, &req)
		decision, reason, err := s.authorizer.Authorize(ctx, attrs)

		result := v1alpha1.SubjectAccessReviewResult{}
		if err != nil {
			result.EvaluationError = err.Error()
			klog.V(5).Infof("Request %d: error=%v", i, err)
		} else {
			switch decision {
			case authorizer.DecisionAllow:
				result.Allowed = true
				result.Reason = reason
				klog.V(5).Infof("Request %d: allowed, reason=%s", i, reason)
			case authorizer.DecisionDeny:
				result.Denied = true
				result.Reason = reason
				klog.V(5).Infof("Request %d: denied, reason=%s", i, reason)
			default:
				// NoOpinion - not explicitly allowed or denied
				result.Reason = reason
				klog.V(5).Infof("Request %d: no opinion, reason=%s", i, reason)
			}
		}

		results[i] = result
	}

	bsar.Status.Results = results
	return bsar, nil
}

func (s *BulkSARStorage) authorizeNonSelfReview(ctx context.Context, caller user.Info) error {
	attrs := &accessAttributes{
		user: caller,
		resourceAttributes: &v1alpha1.ResourceAttributes{
			Verb:        "create",
			Group:       v1alpha1.GroupName,
			Version:     v1alpha1.SchemeGroupVersion.Version,
			Resource:    "bulksubjectaccessreviews",
			Subresource: nonSelfReviewSubresource,
		},
	}

	decision, reason, err := s.authorizer.Authorize(ctx, attrs)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("authorize non-self BulkSubjectAccessReview: %w", err))
	}
	if decision == authorizer.DecisionAllow {
		return nil
	}
	if reason == "" {
		reason = "non-self BulkSubjectAccessReview is not allowed"
	}

	return apierrors.NewForbidden(
		v1alpha1.Resource("bulksubjectaccessreviews/"+nonSelfReviewSubresource),
		"",
		errors.New(reason),
	)
}

// buildAttributes creates authorization.Attributes from the request
func (s *BulkSARStorage) buildAttributes(
	userName string,
	uid string,
	groups []string,
	extra map[string][]string,
	req *v1alpha1.SubjectAccessReviewRequest,
) authorizer.Attributes {
	return &accessAttributes{
		user: &userInfo{
			name:   userName,
			uid:    uid,
			groups: groups,
			extra:  extra,
		},
		resourceAttributes:    req.ResourceAttributes,
		nonResourceAttributes: req.NonResourceAttributes,
	}
}

// accessAttributes implements authorizer.Attributes
type accessAttributes struct {
	user                  user.Info
	resourceAttributes    *v1alpha1.ResourceAttributes
	nonResourceAttributes *v1alpha1.NonResourceAttributes
}

func (a *accessAttributes) GetUser() user.Info {
	return a.user
}

func (a *accessAttributes) GetVerb() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Verb
	}
	if a.nonResourceAttributes != nil {
		return a.nonResourceAttributes.Verb
	}
	return ""
}

func (a *accessAttributes) IsReadOnly() bool {
	verb := a.GetVerb()
	return verb == "get" || verb == "list" || verb == "watch"
}

func (a *accessAttributes) GetNamespace() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Namespace
	}
	return ""
}

func (a *accessAttributes) GetResource() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Resource
	}
	return ""
}

func (a *accessAttributes) GetSubresource() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Subresource
	}
	return ""
}

func (a *accessAttributes) GetName() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Name
	}
	return ""
}

func (a *accessAttributes) GetAPIGroup() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Group
	}
	return ""
}

func (a *accessAttributes) GetAPIVersion() string {
	if a.resourceAttributes != nil {
		return a.resourceAttributes.Version
	}
	return ""
}

func (a *accessAttributes) IsResourceRequest() bool {
	return a.resourceAttributes != nil
}

func (a *accessAttributes) GetPath() string {
	if a.nonResourceAttributes != nil {
		return a.nonResourceAttributes.Path
	}
	return ""
}

func (a *accessAttributes) GetFieldSelector() (fields.Requirements, error) {
	// FieldSelector is not supported in BulkSubjectAccessReview
	return nil, nil
}

func (a *accessAttributes) GetLabelSelector() (labels.Requirements, error) {
	// LabelSelector is not supported in BulkSubjectAccessReview
	return nil, nil
}

// userInfo implements user.Info
type userInfo struct {
	name   string
	uid    string
	groups []string
	extra  map[string][]string
}

func (u *userInfo) GetName() string               { return u.name }
func (u *userInfo) GetUID() string                { return u.uid }
func (u *userInfo) GetGroups() []string           { return u.groups }
func (u *userInfo) GetExtra() map[string][]string { return u.extra }
