/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// TestResponseFormat_ExactJSONStructure verifies that response has exact JSON format
func TestResponseFormat_ExactJSONStructure(t *testing.T) {
	mock := newMockAuthorizer()
	mock.setDecision("get", "pods", "default", authorizer.DecisionAllow, "RBAC: allowed by ClusterRoleBinding \"viewer\"")
	mock.setDecision("delete", "secrets", "production", authorizer.DecisionDeny, "multi-tenancy: user has no access to the namespace")

	storage := NewBulkSARStorage(mock)

	ctx := request.WithUser(context.Background(), &user.DefaultInfo{
		Name:   "alice",
		Groups: []string{"system:authenticated"},
	})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Namespace: "default"}},
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "delete", Resource: "secrets", Namespace: "production"}},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// Serialize to JSON to verify structure
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)

	t.Logf("Response JSON:\n%s", string(jsonBytes))

	// Deserialize back and verify
	var response v1alpha1.BulkSubjectAccessReview
	err = json.Unmarshal(jsonBytes, &response)
	require.NoError(t, err)

	// Verify status.results structure
	require.Len(t, response.Status.Results, 2, "should have 2 results")

	// Result 0: allowed
	assert.True(t, response.Status.Results[0].Allowed, "result 0 should have allowed=true")
	assert.False(t, response.Status.Results[0].Denied, "result 0 should have denied=false")
	assert.Contains(t, response.Status.Results[0].Reason, "RBAC")
	assert.Empty(t, response.Status.Results[0].EvaluationError)

	// Result 1: denied
	assert.False(t, response.Status.Results[1].Allowed, "result 1 should have allowed=false")
	assert.True(t, response.Status.Results[1].Denied, "result 1 should have denied=true")
	assert.Contains(t, response.Status.Results[1].Reason, "multi-tenancy")
	assert.Empty(t, response.Status.Results[1].EvaluationError)
}

// TestResponseFormat_OrderPreserved verifies that results order matches requests order
func TestResponseFormat_OrderPreserved(t *testing.T) {
	mock := &orderTrackingAuthorizer{
		order: make([]string, 0),
		decisions: map[string]authorizer.Decision{
			"get/pods/":               authorizer.DecisionAllow,
			"list/services/":          authorizer.DecisionAllow,
			"create/deployments/apps": authorizer.DecisionDeny,
			"delete/secrets/":         authorizer.DecisionNoOpinion,
			"get//":                   authorizer.DecisionAllow, // non-resource
		},
	}

	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"}},
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "list", Resource: "services"}},
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "create", Resource: "deployments", Group: "apps"}},
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "delete", Resource: "secrets"}},
				{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/healthz"}},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resp := result.(*v1alpha1.BulkSubjectAccessReview)

	// Verify that number of results equals number of requests
	require.Len(t, resp.Status.Results, 5, "number of results should match requests")

	// Verify order: each result corresponds to its request
	assert.True(t, resp.Status.Results[0].Allowed, "request[0] get pods -> allowed")
	assert.True(t, resp.Status.Results[1].Allowed, "request[1] list services -> allowed")
	assert.True(t, resp.Status.Results[2].Denied, "request[2] create deployments -> denied")
	assert.False(t, resp.Status.Results[3].Allowed, "request[3] delete secrets -> no opinion (not allowed)")
	assert.True(t, resp.Status.Results[4].Allowed, "request[4] non-resource /healthz -> allowed")
}

// TestResponseFormat_AllDecisionTypes verifies all decision types
func TestResponseFormat_AllDecisionTypes(t *testing.T) {
	tests := []struct {
		name          string
		decision      authorizer.Decision
		reason        string
		expectAllowed bool
		expectDenied  bool
		expectReason  string
	}{
		{
			name:          "DecisionAllow",
			decision:      authorizer.DecisionAllow,
			reason:        "allowed by rule X",
			expectAllowed: true,
			expectDenied:  false,
			expectReason:  "allowed by rule X",
		},
		{
			name:          "DecisionDeny",
			decision:      authorizer.DecisionDeny,
			reason:        "denied by policy Y",
			expectAllowed: false,
			expectDenied:  true,
			expectReason:  "denied by policy Y",
		},
		{
			name:          "DecisionNoOpinion",
			decision:      authorizer.DecisionNoOpinion,
			reason:        "",
			expectAllowed: false,
			expectDenied:  false,
			expectReason:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &staticDecisionAuthorizer{decision: tt.decision, reason: tt.reason}
			storage := NewBulkSARStorage(mock)
			ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

			bsar := &v1alpha1.BulkSubjectAccessReview{
				Spec: v1alpha1.BulkSubjectAccessReviewSpec{
					Requests: []v1alpha1.SubjectAccessReviewRequest{
						{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"}},
					},
				},
			}

			result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
			require.NoError(t, err)

			resp := result.(*v1alpha1.BulkSubjectAccessReview)
			require.Len(t, resp.Status.Results, 1)

			assert.Equal(t, tt.expectAllowed, resp.Status.Results[0].Allowed, "allowed mismatch")
			assert.Equal(t, tt.expectDenied, resp.Status.Results[0].Denied, "denied mismatch")
			assert.Equal(t, tt.expectReason, resp.Status.Results[0].Reason, "reason mismatch")
		})
	}
}

// TestResponseFormat_EvaluationError verifies that authorizer errors are placed in EvaluationError
func TestResponseFormat_EvaluationError(t *testing.T) {
	mock := &errorAuthorizer{err: fmt.Errorf("internal RBAC error: connection timeout")}
	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"}},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err) // Create should not fail due to authorizer error

	resp := result.(*v1alpha1.BulkSubjectAccessReview)
	require.Len(t, resp.Status.Results, 1)

	assert.False(t, resp.Status.Results[0].Allowed, "on error allowed should be false")
	assert.Equal(t, "internal RBAC error: connection timeout", resp.Status.Results[0].EvaluationError)
}

// TestResponseFormat_SelfModeUsesCallerIdentity verifies that self-mode uses caller identity
func TestResponseFormat_SelfModeUsesCallerIdentity(t *testing.T) {
	var capturedUser string
	mock := &captureUserAuthorizer{capture: func(u user.Info) {
		capturedUser = u.GetName()
	}}

	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{
		Name:   "alice@example.com",
		Groups: []string{"developers"},
	})

	// Self-mode: user not specified in spec
	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"}},
			},
		},
	}

	_, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	assert.Equal(t, "alice@example.com", capturedUser, "self-mode should use caller identity")
}

// TestResponseFormat_NonSelfModeUsesSpecUser verifies that non-self mode uses user from spec
func TestResponseFormat_NonSelfModeUsesSpecUser(t *testing.T) {
	var capturedUser string
	var capturedGroups []string
	mock := &captureUserAuthorizer{capture: func(u user.Info) {
		capturedUser = u.GetName()
		capturedGroups = u.GetGroups()
	}}

	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{
		Name:   "admin@example.com",
		Groups: []string{"system:masters"},
	})

	// Non-self mode: user specified in spec
	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			User:   "bob@example.com",
			Groups: []string{"developers", "team-a"},
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"}},
			},
		},
	}

	_, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	assert.Equal(t, "bob@example.com", capturedUser, "non-self mode should use spec.user")
	assert.Equal(t, []string{"developers", "team-a"}, capturedGroups, "non-self mode should use spec.groups")
}

// TestResponseFormat_LargeRequest verifies processing of large number of requests
func TestResponseFormat_LargeRequest(t *testing.T) {
	mock := &staticDecisionAuthorizer{decision: authorizer.DecisionAllow, reason: "ok"}
	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	// 100 requests
	requests := make([]v1alpha1.SubjectAccessReviewRequest, 100)
	for i := 0; i < 100; i++ {
		requests[i] = v1alpha1.SubjectAccessReviewRequest{
			ResourceAttributes: &v1alpha1.ResourceAttributes{
				Verb:      "get",
				Resource:  fmt.Sprintf("resource-%d", i),
				Namespace: fmt.Sprintf("ns-%d", i%10),
			},
		}
	}

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{Requests: requests},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resp := result.(*v1alpha1.BulkSubjectAccessReview)
	require.Len(t, resp.Status.Results, 100, "should have exactly 100 results")

	// All should be allowed
	for i, r := range resp.Status.Results {
		assert.True(t, r.Allowed, "result %d should be allowed", i)
	}
}

// TestResponseFormat_EmptyRequests verifies processing of empty request list
func TestResponseFormat_EmptyRequests(t *testing.T) {
	mock := &staticDecisionAuthorizer{decision: authorizer.DecisionAllow}
	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{}, // empty list
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resp := result.(*v1alpha1.BulkSubjectAccessReview)
	assert.Len(t, resp.Status.Results, 0, "empty request -> empty result")
}

// TestResponseFormat_NonResourceAttributes verifies non-resource requests
func TestResponseFormat_NonResourceAttributes(t *testing.T) {
	mock := &pathBasedAuthorizer{
		allowedPaths: map[string]bool{
			"/healthz": true,
			"/version": true,
			"/metrics": false,
			"/api":     true,
			"/api/v1":  true,
		},
	}

	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/healthz"}},
				{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/metrics"}},
				{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/version"}},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resp := result.(*v1alpha1.BulkSubjectAccessReview)
	require.Len(t, resp.Status.Results, 3)

	assert.True(t, resp.Status.Results[0].Allowed, "/healthz should be allowed")
	assert.True(t, resp.Status.Results[1].Denied, "/metrics should be denied")
	assert.True(t, resp.Status.Results[2].Allowed, "/version should be allowed")
}

// TestResponseFormat_MixedRequests verifies mixed resource + non-resource requests
func TestResponseFormat_MixedRequests(t *testing.T) {
	mock := &mixedAuthorizer{
		resourceDecisions: map[string]authorizer.Decision{
			"get/pods/default": authorizer.DecisionAllow,
		},
		pathDecisions: map[string]authorizer.Decision{
			"/healthz": authorizer.DecisionAllow,
		},
	}

	storage := NewBulkSARStorage(mock)
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: "test"})

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Namespace: "default"}},
				{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/healthz"}},
				{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "delete", Resource: "secrets"}},
			},
		},
	}

	result, err := storage.Create(ctx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resp := result.(*v1alpha1.BulkSubjectAccessReview)
	require.Len(t, resp.Status.Results, 3)

	assert.True(t, resp.Status.Results[0].Allowed, "resource request should be allowed")
	assert.True(t, resp.Status.Results[1].Allowed, "non-resource request should be allowed")
	assert.False(t, resp.Status.Results[2].Allowed, "unknown resource request should not be allowed")
}

// === Helper authorizers for tests ===

type orderTrackingAuthorizer struct {
	order     []string
	decisions map[string]authorizer.Decision
}

func (o *orderTrackingAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	key := attrs.GetVerb() + "/" + attrs.GetResource() + "/" + attrs.GetAPIGroup()
	o.order = append(o.order, key)
	if d, ok := o.decisions[key]; ok {
		return d, "", nil
	}
	return authorizer.DecisionNoOpinion, "", nil
}

type staticDecisionAuthorizer struct {
	decision authorizer.Decision
	reason   string
}

func (s *staticDecisionAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return s.decision, s.reason, nil
}

type errorAuthorizer struct {
	err error
}

func (e *errorAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", e.err
}

type captureUserAuthorizer struct {
	capture func(user.Info)
}

func (c *captureUserAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if c.capture != nil {
		c.capture(attrs.GetUser())
	}
	return authorizer.DecisionAllow, "", nil
}

type pathBasedAuthorizer struct {
	allowedPaths map[string]bool
}

func (p *pathBasedAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if !attrs.IsResourceRequest() {
		if allowed, ok := p.allowedPaths[attrs.GetPath()]; ok && allowed {
			return authorizer.DecisionAllow, "", nil
		}
		return authorizer.DecisionDeny, "path not allowed", nil
	}
	return authorizer.DecisionNoOpinion, "", nil
}

type mixedAuthorizer struct {
	resourceDecisions map[string]authorizer.Decision
	pathDecisions     map[string]authorizer.Decision
}

func (m *mixedAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if attrs.IsResourceRequest() {
		key := attrs.GetVerb() + "/" + attrs.GetResource() + "/" + attrs.GetNamespace()
		if d, ok := m.resourceDecisions[key]; ok {
			return d, "", nil
		}
	} else {
		if d, ok := m.pathDecisions[attrs.GetPath()]; ok {
			return d, "", nil
		}
	}
	return authorizer.DecisionNoOpinion, "", nil
}
