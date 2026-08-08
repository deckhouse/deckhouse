/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package scopefilter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type stubAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
}

func (s *stubAuthorizer) Authorize(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
	return s.decision, s.reason, s.err
}

// servedResources answers for a cluster that has the resources listed, keyed
// the way discovery reports them.
type servedResources map[string]struct{}

func (s servedResources) HasResource(group, resource string) bool {
	_, ok := s[group+"/"+resource]
	return ok
}

// projectCRDInstalled stands for a cluster running multitenancy-manager.
func projectCRDInstalled() servedResources {
	return servedResources{"deckhouse.io/projects": {}}
}

func projectAttributes(verb string) authorizer.AttributesRecord {
	return authorizer.AttributesRecord{
		ResourceRequest: true,
		Verb:            verb,
		APIGroup:        "deckhouse.io",
		APIVersion:      "v1alpha2",
		Resource:        "projects",
	}
}

// TestIdentityReadAuthorizerReportsFilteredReads pins the reason the wrapper
// exists: the cluster-wide RBAC grant is withheld so the apiserver's filter can
// engage, and a read that the API answers with the accessible subset must not
// be reported as a denial.
func TestIdentityReadAuthorizerReportsFilteredReads(t *testing.T) {
	for _, verb := range []string{"get", "list", "watch"} {
		t.Run(verb, func(t *testing.T) {
			auth := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionNoOpinion}, projectCRDInstalled())

			decision, reason, err := auth.Authorize(context.Background(), projectAttributes(verb))

			require.NoError(t, err)
			assert.Equal(t, authorizer.DecisionAllow, decision)
			assert.Equal(t, filteredReadReason, reason)
		})
	}
}

// TestIdentityReadAuthorizerOverridesDeny covers the multi-tenancy path: the
// apiserver hands any denied read of an identity resource to the storage
// filter, whether the denial was an explicit Deny or the absence of a grant.
func TestIdentityReadAuthorizerOverridesDeny(t *testing.T) {
	auth := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionDeny, reason: "no access"}, projectCRDInstalled())

	decision, reason, err := auth.Authorize(context.Background(), projectAttributes("list"))

	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Equal(t, filteredReadReason, reason)
}

func TestIdentityReadAuthorizerLeavesOtherRequestsAlone(t *testing.T) {
	tests := []struct {
		name  string
		attrs authorizer.Attributes
	}{
		{
			name: "a write is still governed by RBAC alone",
			attrs: authorizer.AttributesRecord{
				ResourceRequest: true,
				Verb:            "create",
				APIGroup:        "deckhouse.io",
				Resource:        "projects",
			},
		},
		{
			name: "a subresource is not filtered by the storage layer",
			attrs: authorizer.AttributesRecord{
				ResourceRequest: true,
				Verb:            "get",
				APIGroup:        "deckhouse.io",
				Resource:        "projects",
				Subresource:     "status",
			},
		},
		{
			name: "a same-group neighbour is not an identity resource",
			attrs: authorizer.AttributesRecord{
				ResourceRequest: true,
				Verb:            "list",
				APIGroup:        "deckhouse.io",
				Resource:        "projecttemplates",
			},
		},
		{
			name: "the group is part of the match, not just the plural",
			attrs: authorizer.AttributesRecord{
				ResourceRequest: true,
				Verb:            "list",
				APIGroup:        "example.com",
				Resource:        "projects",
			},
		},
		{
			name: "a non-resource request has no identity to derive",
			attrs: authorizer.AttributesRecord{
				Verb: "get",
				Path: "/healthz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionNoOpinion, reason: "nothing to say"}, projectCRDInstalled())

			decision, reason, err := auth.Authorize(context.Background(), tt.attrs)

			require.NoError(t, err)
			assert.Equal(t, authorizer.DecisionNoOpinion, decision)
			assert.Equal(t, "nothing to say", reason)
		})
	}
}

// TestIdentityReadAuthorizerNeedsTheResourceToExist covers a cluster with
// user-authz and the permission browser but no multitenancy-manager: nothing
// serves Projects, the apiserver keeps the plain denial because its bypass is
// gated on the filter being registered, and reporting an allow would put a
// section in the console that the API answers with NotFound.
func TestIdentityReadAuthorizerNeedsTheResourceToExist(t *testing.T) {
	tests := []struct {
		name     string
		registry ResourceRegistry
	}{
		{name: "the Project CRD is absent", registry: servedResources{}},
		{name: "a neighbouring CRD is not a Project", registry: servedResources{"deckhouse.io/projecttemplates": {}}},
		{name: "discovery is unavailable", registry: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionNoOpinion, reason: "no grant"}, tt.registry)

			decision, reason, err := auth.Authorize(context.Background(), projectAttributes("list"))

			require.NoError(t, err)
			assert.Equal(t, authorizer.DecisionNoOpinion, decision)
			assert.Equal(t, "no grant", reason)
		})
	}
}

// TestIdentityReadAuthorizerPassesThroughAllowAndError makes sure the wrapper
// never invents a decision of its own: an RBAC allow keeps its reason, and a
// broken authorizer stays broken rather than turning into an allow.
func TestIdentityReadAuthorizerPassesThroughAllowAndError(t *testing.T) {
	allowed := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionAllow, reason: "RBAC allowed"}, projectCRDInstalled())
	decision, reason, err := allowed.Authorize(context.Background(), projectAttributes("list"))
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Equal(t, "RBAC allowed", reason)

	failing := NewIdentityReadAuthorizer(&stubAuthorizer{decision: authorizer.DecisionNoOpinion, err: errors.New("informer not synced")}, projectCRDInstalled())
	_, _, err = failing.Authorize(context.Background(), projectAttributes("list"))
	assert.Error(t, err)
}
