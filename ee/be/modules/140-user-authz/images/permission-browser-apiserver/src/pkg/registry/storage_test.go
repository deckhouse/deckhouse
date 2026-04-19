/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// mockAuthorizer implements authorizer.Authorizer for testing
type mockAuthorizer struct {
	decisions map[string]authorizer.Decision
	reasons   map[string]string
}

func newMockAuthorizer() *mockAuthorizer {
	return &mockAuthorizer{
		decisions: make(map[string]authorizer.Decision),
		reasons:   make(map[string]string),
	}
}

func (m *mockAuthorizer) setDecision(verb, resource, namespace string, decision authorizer.Decision, reason string) {
	key := verb + "/" + resource + "/" + namespace
	m.decisions[key] = decision
	m.reasons[key] = reason
}

func (m *mockAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	key := attrs.GetVerb() + "/" + attrs.GetResource() + "/" + attrs.GetNamespace()
	if d, ok := m.decisions[key]; ok {
		return d, m.reasons[key], nil
	}
	return authorizer.DecisionNoOpinion, "", nil
}

func TestBulkSARStorage_Create_SelfMode(t *testing.T) {
	mock := newMockAuthorizer()
	mock.setDecision("get", "pods", "default", authorizer.DecisionAllow, "RBAC allowed")
	mock.setDecision("create", "pods", "default", authorizer.DecisionDeny, "RBAC denied")

	storage := NewBulkSARStorage(mock)

	// Create context with user
	ctx := context.Background()
	ctx = request.WithUser(ctx, &user.DefaultInfo{
		Name:   "test-user",
		Groups: []string{"system:authenticated"},
	})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			// No user specified - self mode
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:      "get",
						Resource:  "pods",
						Namespace: "default",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:      "create",
						Resource:  "pods",
						Namespace: "default",
					},
				},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, nil)
	require.NoError(t, err)

	resultBSAR, ok := result.(*v1alpha1.BulkSubjectAccessReview)
	require.True(t, ok)

	assert.Len(t, resultBSAR.Status.Results, 2)
	assert.True(t, resultBSAR.Status.Results[0].Allowed)
	assert.Equal(t, "RBAC allowed", resultBSAR.Status.Results[0].Reason)
	assert.True(t, resultBSAR.Status.Results[1].Denied)
	assert.Equal(t, "RBAC denied", resultBSAR.Status.Results[1].Reason)
}

func TestBulkSARStorage_Create_NonSelfMode(t *testing.T) {
	mock := newMockAuthorizer()
	mock.setDecision("list", "secrets", "kube-system", authorizer.DecisionAllow, "admin access")

	storage := NewBulkSARStorage(mock)

	ctx := context.Background()
	ctx = request.WithUser(ctx, &user.DefaultInfo{
		Name:   "admin-user",
		Groups: []string{"system:masters"},
	})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			User:   "other-user",
			Groups: []string{"developers"},
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:      "list",
						Resource:  "secrets",
						Namespace: "kube-system",
					},
				},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, nil)
	require.NoError(t, err)

	resultBSAR, ok := result.(*v1alpha1.BulkSubjectAccessReview)
	require.True(t, ok)

	assert.Len(t, resultBSAR.Status.Results, 1)
	assert.True(t, resultBSAR.Status.Results[0].Allowed)
}

func TestAccessAttributes(t *testing.T) {
	tests := []struct {
		name     string
		attrs    *accessAttributes
		expected struct {
			verb              string
			namespace         string
			resource          string
			isResourceRequest bool
			path              string
		}
	}{
		{
			name: "resource request",
			attrs: &accessAttributes{
				user: &userInfo{name: "test"},
				resourceAttributes: &v1alpha1.ResourceAttributes{
					Verb:      "get",
					Namespace: "default",
					Resource:  "pods",
					Group:     "",
					Version:   "v1",
				},
			},
			expected: struct {
				verb              string
				namespace         string
				resource          string
				isResourceRequest bool
				path              string
			}{
				verb:              "get",
				namespace:         "default",
				resource:          "pods",
				isResourceRequest: true,
				path:              "",
			},
		},
		{
			name: "non-resource request",
			attrs: &accessAttributes{
				user: &userInfo{name: "test"},
				nonResourceAttributes: &v1alpha1.NonResourceAttributes{
					Verb: "get",
					Path: "/healthz",
				},
			},
			expected: struct {
				verb              string
				namespace         string
				resource          string
				isResourceRequest bool
				path              string
			}{
				verb:              "get",
				namespace:         "",
				resource:          "",
				isResourceRequest: false,
				path:              "/healthz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected.verb, tt.attrs.GetVerb())
			assert.Equal(t, tt.expected.namespace, tt.attrs.GetNamespace())
			assert.Equal(t, tt.expected.resource, tt.attrs.GetResource())
			assert.Equal(t, tt.expected.isResourceRequest, tt.attrs.IsResourceRequest())
			assert.Equal(t, tt.expected.path, tt.attrs.GetPath())
		})
	}
}
