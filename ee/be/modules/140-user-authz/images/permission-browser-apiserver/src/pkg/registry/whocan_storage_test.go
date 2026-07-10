/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
)

// mockWhoCanResolver is a stub WhoCanResolver that records the attributes it
// was called with and returns a canned result (and optional error).
type mockWhoCanResolver struct {
	gotAttrs authorizer.Attributes
	result   rbacadapter.WhoCanResult
	err      error
}

func (m *mockWhoCanResolver) WhoCan(_ context.Context, attrs authorizer.Attributes) (rbacadapter.WhoCanResult, error) {
	m.gotAttrs = attrs
	return m.result, m.err
}

func TestWhoCanStorage_Create_ResourceAttributes(t *testing.T) {
	mock := &mockWhoCanResolver{
		result: rbacadapter.WhoCanResult{
			Users:  []string{"alice"},
			Groups: []string{"netops"},
			ServiceAccounts: []rbacadapter.ServiceAccountRef{
				{Namespace: "kube-system", Name: "controller"},
			},
		},
	}

	storage := NewWhoCanStorage(mock)

	wc := &v1alpha1.WhoCan{
		Spec: v1alpha1.WhoCanSpec{
			ResourceAttributes: &v1alpha1.ResourceAttributes{
				Namespace: "myproject",
				Verb:      "create",
				Group:     "networking.k8s.io",
				Resource:  "networkpolicies",
			},
		},
	}

	result, err := storage.Create(context.Background(), wc, nil, nil)
	require.NoError(t, err)

	out, ok := result.(*v1alpha1.WhoCan)
	require.True(t, ok)

	assert.Equal(t, []string{"alice"}, out.Status.Users)
	assert.Equal(t, []string{"netops"}, out.Status.Groups)
	require.Len(t, out.Status.ServiceAccounts, 1)
	assert.Equal(t, "kube-system", out.Status.ServiceAccounts[0].Namespace)
	assert.Equal(t, "controller", out.Status.ServiceAccounts[0].Name)

	// Ensure the storage forwarded the request attributes correctly.
	require.NotNil(t, mock.gotAttrs)
	assert.Equal(t, "create", mock.gotAttrs.GetVerb())
	assert.Equal(t, "networkpolicies", mock.gotAttrs.GetResource())
	assert.Equal(t, "networking.k8s.io", mock.gotAttrs.GetAPIGroup())
	assert.Equal(t, "myproject", mock.gotAttrs.GetNamespace())
	assert.True(t, mock.gotAttrs.IsResourceRequest())
}

func TestWhoCanStorage_Create_NonResourceAttributes(t *testing.T) {
	mock := &mockWhoCanResolver{
		result: rbacadapter.WhoCanResult{Groups: []string{"monitoring"}},
	}

	storage := NewWhoCanStorage(mock)

	wc := &v1alpha1.WhoCan{
		Spec: v1alpha1.WhoCanSpec{
			NonResourceAttributes: &v1alpha1.NonResourceAttributes{
				Path: "/metrics",
				Verb: "get",
			},
		},
	}

	result, err := storage.Create(context.Background(), wc, nil, nil)
	require.NoError(t, err)

	out := result.(*v1alpha1.WhoCan)
	assert.Equal(t, []string{"monitoring"}, out.Status.Groups)
	assert.False(t, mock.gotAttrs.IsResourceRequest())
	assert.Equal(t, "/metrics", mock.gotAttrs.GetPath())
}

func TestWhoCanStorage_Create_EmptySpecIsRejected(t *testing.T) {
	storage := NewWhoCanStorage(&mockWhoCanResolver{})

	wc := &v1alpha1.WhoCan{Spec: v1alpha1.WhoCanSpec{}}

	_, err := storage.Create(context.Background(), wc, nil, nil)
	require.Error(t, err)
}

func TestWhoCanStorage_Create_WrongType(t *testing.T) {
	storage := NewWhoCanStorage(&mockWhoCanResolver{})

	_, err := storage.Create(context.Background(), &v1alpha1.BulkSubjectAccessReview{}, nil, nil)
	require.Error(t, err)
}

func TestWhoCanStorage_Create_EvaluationErrorIsSurfaced(t *testing.T) {
	// A resolver error must be surfaced in Status.EvaluationError (not as a hard
	// error), while whatever partial subjects were found are still returned.
	mock := &mockWhoCanResolver{
		result: rbacadapter.WhoCanResult{Users: []string{"alice"}},
		err:    errors.New("listing ClusterRoleBindings: boom"),
	}
	storage := NewWhoCanStorage(mock)

	wc := &v1alpha1.WhoCan{
		Spec: v1alpha1.WhoCanSpec{
			ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"},
		},
	}

	result, err := storage.Create(context.Background(), wc, nil, nil)
	require.NoError(t, err)

	out := result.(*v1alpha1.WhoCan)
	assert.Equal(t, []string{"alice"}, out.Status.Users)
	assert.Equal(t, "listing ClusterRoleBindings: boom", out.Status.EvaluationError)
}

func TestWhoCanStorage_Create_RunsCreateValidation(t *testing.T) {
	storage := NewWhoCanStorage(&mockWhoCanResolver{})

	wc := &v1alpha1.WhoCan{
		Spec: v1alpha1.WhoCanSpec{
			ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods"},
		},
	}

	wantErr := errors.New("denied by admission")
	called := false
	_, err := storage.Create(context.Background(), wc, func(_ context.Context, _ runtime.Object) error {
		called = true
		return wantErr
	}, nil)

	require.ErrorIs(t, err, wantErr)
	assert.True(t, called)
}

func TestWhoCanStorage_Scoping(t *testing.T) {
	storage := NewWhoCanStorage(&mockWhoCanResolver{})
	assert.False(t, storage.NamespaceScoped())
	assert.Equal(t, "whocan", storage.GetSingularName())
	assert.NotNil(t, storage.New())
}
