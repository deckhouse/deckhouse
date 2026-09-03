/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/composite"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

// allowAllAuthorizer grants every request. BulkSAR results then follow only
// the multi-tenancy layer: Deny stays denied, NoOpinion becomes allowed. That
// is the console cani shape once the subject has a CAR accessLevel binding.
type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Authorize(context.Context, authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionAllow, "rbac-allow-all", nil
}

type denyIndependent struct{}

func (denyIndependent) AllowsIndependently(context.Context, authorizer.Attributes) bool {
	return false
}

type staticResourceScope map[string]bool

func (s staticResourceScope) Scope(group, resource string) (namespaced, known bool) {
	namespaced, known = s[group+"/"+resource]
	return namespaced, known
}

func (s staticResourceScope) HasData() bool { return len(s) > 0 }

func writeMTConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func mustMTEngine(t *testing.T, config string) *multitenancy.Engine {
	t.Helper()
	engine, err := multitenancy.NewEngine(writeMTConfig(t, config), nil, nil, staticResourceScope{
		"/pods":  true,
		"/nodes": false,
	})
	require.NoError(t, err)
	engine.SetIndependentRBACChecker(denyIndependent{})
	return engine
}

const (
	bulkSAREditorConfig = `{
		"crds": [{
			"name": "editor",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		}]
	}`
	bulkSARSuperConfig = `{
		"crds": [{
			"name": "super",
			"spec": {
				"accessLevel": "SuperAdmin",
				"allowAccessToSystemNamespaces": true,
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "super@example.io"}]
			}
		}]
	}`
)

func consoleLikeRequests() []v1alpha1.SubjectAccessReviewRequest {
	return []v1alpha1.SubjectAccessReviewRequest{
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "list", Resource: "pods"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Namespace: "ns-in"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Namespace: "ns-out"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "nodes"}},
	}
}

func evalBulkSAR(t *testing.T, engine *multitenancy.Engine, subject string) []v1alpha1.SubjectAccessReviewResult {
	t.Helper()
	storage := NewBulkSARStorage(composite.NewCompositeAuthorizer(engine, allowAllAuthorizer{}))
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{
		Name:   "admin",
		Groups: []string{"system:masters"},
	})
	out, err := storage.Create(ctx, &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			User:     subject,
			Requests: consoleLikeRequests(),
		},
	}, nil, nil)
	require.NoError(t, err)
	got, ok := out.(*v1alpha1.BulkSubjectAccessReview)
	require.True(t, ok)
	require.Len(t, got.Status.Results, 4)
	return got.Status.Results
}

// TestBulkSARStorage_Create_MTFilterContract is the BulkSAR view of the
// Editor vs SuperAdmin matrix. RBAC is allow-all so each item is decided
// only by multi-tenancy. Optimization must keep these four answers.
func TestBulkSARStorage_Create_MTFilterContract(t *testing.T) {
	editor := evalBulkSAR(t, mustMTEngine(t, bulkSAREditorConfig), "editor@example.io")
	assert.True(t, editor[0].Denied, "editor list pods cluster-scoped: %+v", editor[0])
	assert.False(t, editor[0].Allowed)
	assert.Contains(t, editor[0].Reason, "cluster-scoped requests for namespaced resources")
	assert.True(t, editor[1].Allowed, "editor get pods in CAR ns: %+v", editor[1])
	assert.False(t, editor[1].Denied)
	assert.True(t, editor[2].Denied, "editor get pods outside CAR ns: %+v", editor[2])
	assert.False(t, editor[2].Allowed)
	assert.True(t, editor[3].Allowed, "editor get nodes (cluster-scoped resource): %+v", editor[3])
	assert.False(t, editor[3].Denied)

	super := evalBulkSAR(t, mustMTEngine(t, bulkSARSuperConfig), "super@example.io")
	for i, r := range super {
		assert.True(t, r.Allowed, "superadmin request %d: %+v", i, r)
		assert.False(t, r.Denied)
	}
}
