/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

// stubRoleReporter records the request it was asked to answer.
type stubRoleReporter struct {
	got resolver.RoleAccessRequest
}

func (s *stubRoleReporter) Report(_ context.Context, req resolver.RoleAccessRequest) (v1alpha1.RoleAccessReportStatus, error) {
	s.got = req

	return v1alpha1.RoleAccessReportStatus{Snapshot: v1alpha1.ReportSnapshot{Model: req.Model}}, nil
}

func createRoleReport(t *testing.T, spec v1alpha1.RoleAccessReportSpec) (*stubRoleReporter, *v1alpha1.RoleAccessReport, error) {
	t.Helper()

	reporter := &stubRoleReporter{}
	storage := NewRoleAccessStorage(reporter)

	obj, err := storage.Create(context.Background(), &v1alpha1.RoleAccessReport{Spec: spec}, nil, nil)
	if err != nil {
		return reporter, nil, err
	}

	report, ok := obj.(*v1alpha1.RoleAccessReport)
	require.True(t, ok, "storage returned %T", obj)

	return reporter, report, nil
}

// Wildcard expansion is what makes a "*" rule readable, so it stays on unless
// the caller turns it off; the composition is the detailed mode and stays off.
func TestRoleAccessStorage_Defaults(t *testing.T) {
	t.Parallel()

	reporter, report, err := createRoleReport(t, v1alpha1.RoleAccessReportSpec{})
	require.NoError(t, err)

	assert.True(t, reporter.got.ExpandWildcards)
	assert.False(t, reporter.got.IncludeComposition)
	assert.Empty(t, report.Status.Snapshot.Model, "the model the resolver was asked for is echoed by the resolver, not invented here")
}

func TestRoleAccessStorage_PassesTheSelectionThrough(t *testing.T) {
	t.Parallel()

	no := false
	yes := true

	reporter, _, err := createRoleReport(t, v1alpha1.RoleAccessReportSpec{
		Model: resolver.RoleModelPrimary,
		Roles: v1alpha1.RoleSelection{
			ExcludeCustom: true,
			Names:         []string{"d8:namespace:admin"},
			Scopes:        []string{"namespace"},
		},
		ExpandWildcards:    &no,
		IncludeComposition: &yes,
	})
	require.NoError(t, err)

	assert.Equal(t, resolver.RoleModelPrimary, reporter.got.Model)
	assert.Equal(t, []string{"d8:namespace:admin"}, reporter.got.Names)
	assert.Equal(t, []string{"namespace"}, reporter.got.Scopes)
	assert.True(t, reporter.got.ExcludeCustom)
	assert.False(t, reporter.got.ExpandWildcards)
	assert.True(t, reporter.got.IncludeComposition)
}

// A request that cannot be answered has to say so. Answering a question the
// caller did not ask is worse here than refusing: the result becomes a document.
func TestRoleAccessStorage_RefusesAnUnanswerableRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec v1alpha1.RoleAccessReportSpec
		says string
	}{
		{
			name: "an unknown model",
			spec: v1alpha1.RoleAccessReportSpec{Model: "rbacv1"},
			says: "spec.model",
		},
		{
			name: "an unknown scope",
			spec: v1alpha1.RoleAccessReportSpec{Roles: v1alpha1.RoleSelection{Scopes: []string{"cluster"}}},
			says: "unknown scope",
		},
		{
			name: "scopes asked of the legacy model",
			spec: v1alpha1.RoleAccessReportSpec{Model: resolver.RoleModelLegacy, Roles: v1alpha1.RoleSelection{Scopes: []string{"namespace"}}},
			says: "primary model only",
		},
		{
			name: "access levels asked of the primary model",
			spec: v1alpha1.RoleAccessReportSpec{Roles: v1alpha1.RoleSelection{AccessLevels: []string{"Admin"}}},
			says: "legacy model only",
		},
		{
			name: "custom roles excluded from the legacy model, which has none",
			spec: v1alpha1.RoleAccessReportSpec{
				Model: resolver.RoleModelLegacy,
				Roles: v1alpha1.RoleSelection{ExcludeCustom: true},
			},
			says: "primary model only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := createRoleReport(t, tt.spec)
			require.Error(t, err)
			assert.True(t, apierrors.IsBadRequest(err), "want 400, got %v", err)
			assert.Contains(t, err.Error(), tt.says)
		})
	}
}

// The selection bounds exist so one request cannot ask the resolver to walk an
// arbitrary amount of work.
func TestRoleAccessStorage_BoundsTheSelection(t *testing.T) {
	t.Parallel()

	names := make([]string, maxRequestedRoleNames+1)
	for i := range names {
		names[i] = "role"
	}

	_, _, err := createRoleReport(t, v1alpha1.RoleAccessReportSpec{Roles: v1alpha1.RoleSelection{Names: names}})
	require.Error(t, err)
	assert.True(t, apierrors.IsBadRequest(err), "want 400, got %v", err)
	assert.Contains(t, err.Error(), "spec.roles.names")
}
