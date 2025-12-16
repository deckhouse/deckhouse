/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// GetStorage returns the storage map for the authorization API group
func GetStorage(auth authorizer.Authorizer) map[string]rest.Storage {
	return map[string]rest.Storage{
		"bulksubjectaccessreviews": NewBulkSARStorage(auth),
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
		return nil, fmt.Errorf("object is not a BulkSubjectAccessReview")
	}

	// Get the authenticated user from context
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("no user info in context")
	}

	// Resolve subject: if spec.user is set, use non-self mode; otherwise use self mode
	var subjectUser string
	var subjectGroups []string
	var subjectExtra map[string][]string

	if bsar.Spec.User != "" {
		// Non-self mode: use the provided subject
		subjectUser = bsar.Spec.User
		subjectGroups = bsar.Spec.Groups
		subjectExtra = make(map[string][]string)
		for k, v := range bsar.Spec.Extra {
			subjectExtra[k] = []string(v)
		}
		klog.V(4).Infof("Non-self mode: checking access for user=%s, groups=%v", subjectUser, subjectGroups)
	} else {
		// Self mode: use the authenticated user
		subjectUser = userInfo.GetName()
		subjectGroups = userInfo.GetGroups()
		subjectExtra = userInfo.GetExtra()
		klog.V(4).Infof("Self mode: checking access for user=%s, groups=%v", subjectUser, subjectGroups)
	}

	// Process all requests
	results := make([]v1alpha1.SubjectAccessReviewResult, len(bsar.Spec.Requests))

	for i, req := range bsar.Spec.Requests {
		attrs := s.buildAttributes(subjectUser, subjectGroups, subjectExtra, &req)
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

// buildAttributes creates authorization.Attributes from the request
func (s *BulkSARStorage) buildAttributes(userName string, groups []string, extra map[string][]string, req *v1alpha1.SubjectAccessReviewRequest) authorizer.Attributes {
	return &accessAttributes{
		user: &userInfo{
			name:   userName,
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
