/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

// mockReporter records the request it was called with and returns a canned status.
type mockReporter struct {
	gotRequest resolver.SubjectAccessRequest
	status     v1alpha1.SubjectAccessReportStatus
	err        error
}

func (m *mockReporter) Report(_ context.Context, req resolver.SubjectAccessRequest) (v1alpha1.SubjectAccessReportStatus, error) {
	m.gotRequest = req
	return m.status, m.err
}

// staticAuthorizer answers every check with the same decision.
type staticAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
	gotAttrs authorizer.Attributes
}

func (a *staticAuthorizer) Authorize(_ context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	a.gotAttrs = attrs
	return a.decision, a.reason, a.err
}

func contextWithUser(name string, groups ...string) context.Context {
	return request.WithUser(context.Background(), &user.DefaultInfo{Name: name, Groups: groups})
}

func TestSubjectAccessStorage_SelfModeNeedsNoExtraPermission(t *testing.T) {
	reporter := &mockReporter{}
	// A denying authorizer proves self mode never consults the non-self gate.
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionDeny})

	report := &v1alpha1.SubjectAccessReport{}

	result, err := storage.Create(contextWithUser("alice", "netops"), report, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, v1alpha1.SubjectKindUser, reporter.gotRequest.Subject.Kind)
	assert.Equal(t, "alice", reporter.gotRequest.Subject.Name)
	assert.Equal(t, []string{"netops"}, reporter.gotRequest.CallerGroups)
	// The caller's token already lists its groups, so re-resolving them would
	// only risk disagreeing with it.
	assert.False(t, reporter.gotRequest.ResolveGroups)
}

// A self report is open to every authenticated user precisely because it discloses nothing about
// anyone else. Honouring spec.groups would have handed out any named group's whole permission map --
// the answer a Kind: Group report gates behind the nonself subresource -- so the field is dropped.
func TestSubjectAccessStorage_SelfModeIgnoresRequestedGroups(t *testing.T) {
	reporter := &mockReporter{}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionDeny})

	report := &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{Groups: []string{"d8:some-group", "system:masters"}},
	}

	_, err := storage.Create(contextWithUser("alice", "netops"), report, nil, nil)
	require.NoError(t, err)

	assert.Empty(t, reporter.gotRequest.ExtraGroups)
	assert.Equal(t, []string{"netops"}, reporter.gotRequest.CallerGroups)
}

// The same field is the point of a non-self report, so there it survives.
func TestSubjectAccessStorage_NonSelfKeepsRequestedGroups(t *testing.T) {
	reporter := &mockReporter{}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionAllow})

	report := &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{
			Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: "bob"},
			Groups:  []string{"d8:some-group"},
		},
	}

	_, err := storage.Create(contextWithUser("alice"), report, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"d8:some-group"}, reporter.gotRequest.ExtraGroups)
}

func TestSubjectAccessStorage_SelfModeRecognisesServiceAccount(t *testing.T) {
	reporter := &mockReporter{}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionDeny})

	_, err := storage.Create(contextWithUser("system:serviceaccount:d8-monitoring:prometheus"), &v1alpha1.SubjectAccessReport{}, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, v1alpha1.SubjectKindServiceAccount, reporter.gotRequest.Subject.Kind)
	assert.Equal(t, "prometheus", reporter.gotRequest.Subject.Name)
	assert.Equal(t, "d8-monitoring", reporter.gotRequest.Subject.Namespace)
}

func TestSubjectAccessStorage_NonSelfRequiresSubresource(t *testing.T) {
	reporter := &mockReporter{}
	auth := &staticAuthorizer{decision: authorizer.DecisionDeny, reason: "not allowed"}
	storage := NewSubjectAccessStorage(reporter, auth)

	report := &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{
			Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: "bob"},
		},
	}

	_, err := storage.Create(contextWithUser("alice"), report, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "reporting on another subject must be forbidden without the grant")

	require.NotNil(t, auth.gotAttrs)
	assert.Equal(t, "create", auth.gotAttrs.GetVerb())
	assert.Equal(t, subjectAccessResource, auth.gotAttrs.GetResource())
	assert.Equal(t, nonSelfReviewSubresource, auth.gotAttrs.GetSubresource())
}

func TestSubjectAccessStorage_NonSelfAllowed(t *testing.T) {
	reporter := &mockReporter{}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionAllow})

	report := &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{
			Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindGroup, Name: "netops"},
			Groups:  []string{"extra"},
		},
	}

	_, err := storage.Create(contextWithUser("alice"), report, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, v1alpha1.SubjectKindGroup, reporter.gotRequest.Subject.Kind)
	assert.Equal(t, "netops", reporter.gotRequest.Subject.Name)
	assert.Equal(t, []string{"extra"}, reporter.gotRequest.ExtraGroups)
	assert.True(t, reporter.gotRequest.ResolveGroups, "group resolution is on by default")
	assert.True(t, reporter.gotRequest.ExpandWildcards, "wildcard expansion is on by default")
}

func TestSubjectAccessStorage_DefaultsCanBeDisabled(t *testing.T) {
	reporter := &mockReporter{}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionAllow})

	off := false
	report := &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{
			Subject:         &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: "bob"},
			ResolveGroups:   &off,
			ExpandWildcards: &off,
		},
	}

	_, err := storage.Create(contextWithUser("alice"), report, nil, nil)
	require.NoError(t, err)

	assert.False(t, reporter.gotRequest.ResolveGroups)
	assert.False(t, reporter.gotRequest.ExpandWildcards)
}

func TestSubjectAccessStorage_StatusIsReturned(t *testing.T) {
	reporter := &mockReporter{
		status: v1alpha1.SubjectAccessReportStatus{
			Subject: v1alpha1.ResolvedSubject{Kind: v1alpha1.SubjectKindUser, Name: "alice"},
			Scopes:  []v1alpha1.AccessScope{{Cluster: true}},
			Notes:   []string{"note"},
		},
	}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionAllow})

	result, err := storage.Create(contextWithUser("alice"), &v1alpha1.SubjectAccessReport{}, nil, nil)
	require.NoError(t, err)

	report, ok := result.(*v1alpha1.SubjectAccessReport)
	require.True(t, ok)
	assert.Equal(t, "alice", report.Status.Subject.Name)
	require.Len(t, report.Status.Scopes, 1)
	assert.True(t, report.Status.Scopes[0].Cluster)
	assert.Equal(t, []string{"note"}, report.Status.Notes)
}

func TestSubjectAccessStorage_ReporterFailureIsInternalError(t *testing.T) {
	reporter := &mockReporter{err: errors.New("informer cache is cold")}
	storage := NewSubjectAccessStorage(reporter, &staticAuthorizer{decision: authorizer.DecisionAllow})

	_, err := storage.Create(contextWithUser("alice"), &v1alpha1.SubjectAccessReport{}, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsInternalError(err))
}

func TestSubjectAccessStorage_Validation(t *testing.T) {
	tests := []struct {
		name string
		spec v1alpha1.SubjectAccessReportSpec
	}{
		{
			name: "unknown subject kind",
			spec: v1alpha1.SubjectAccessReportSpec{
				Subject: &v1alpha1.SubjectReference{Kind: "Robot", Name: "r2d2"},
			},
		},
		{
			name: "empty subject name",
			spec: v1alpha1.SubjectAccessReportSpec{
				Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser},
			},
		},
		{
			name: "ServiceAccount without namespace",
			spec: v1alpha1.SubjectAccessReportSpec{
				Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindServiceAccount, Name: "builder"},
			},
		},
		{
			name: "namespace on a User subject",
			spec: v1alpha1.SubjectAccessReportSpec{
				Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: "alice", Namespace: "team-a"},
			},
		},
		{
			name: "too many namespaces",
			spec: v1alpha1.SubjectAccessReportSpec{Namespaces: make([]string, maxRequestedNamespaces+1)},
		},
		{
			name: "too many groups",
			spec: v1alpha1.SubjectAccessReportSpec{Groups: make([]string, maxRequestedGroups+1)},
		},
	}

	storage := NewSubjectAccessStorage(&mockReporter{}, &staticAuthorizer{decision: authorizer.DecisionAllow})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := storage.Create(contextWithUser("alice"), &v1alpha1.SubjectAccessReport{Spec: tt.spec}, nil, nil)
			require.Error(t, err)
			assert.True(t, apierrors.IsBadRequest(err), "expected a bad request, got %v", err)
		})
	}
}

func TestSubjectAccessStorage_RejectsForeignObject(t *testing.T) {
	storage := NewSubjectAccessStorage(&mockReporter{}, &staticAuthorizer{decision: authorizer.DecisionAllow})

	_, err := storage.Create(contextWithUser("alice"), &v1alpha1.WhoCan{}, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsBadRequest(err))
}

func TestSubjectAccessStorage_RequiresUserInContext(t *testing.T) {
	storage := NewSubjectAccessStorage(&mockReporter{}, &staticAuthorizer{decision: authorizer.DecisionAllow})

	_, err := storage.Create(context.Background(), &v1alpha1.SubjectAccessReport{}, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsInternalError(err))
}

func TestSubjectAccessStorage_Metadata(t *testing.T) {
	storage := NewSubjectAccessStorage(&mockReporter{}, &staticAuthorizer{decision: authorizer.DecisionAllow})

	assert.False(t, storage.NamespaceScoped())
	assert.Equal(t, "subjectaccessreport", storage.GetSingularName())
	assert.IsType(t, &v1alpha1.SubjectAccessReport{}, storage.New())
	storage.Destroy()
}

func TestGetStorage_RegistersOptionalResources(t *testing.T) {
	auth := &staticAuthorizer{decision: authorizer.DecisionAllow}

	minimal := GetStorage(Storages{Authorizer: auth})
	assert.Contains(t, minimal, "bulksubjectaccessreviews")
	assert.NotContains(t, minimal, subjectAccessResource)
	assert.NotContains(t, minimal, "whocans")

	full := GetStorage(Storages{Authorizer: auth, SubjectAccess: &mockReporter{}})
	assert.Contains(t, full, subjectAccessResource)
}
