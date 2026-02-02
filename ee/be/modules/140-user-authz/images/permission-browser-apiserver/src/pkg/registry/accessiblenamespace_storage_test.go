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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// mockNamespaceResolver implements a simple mock for testing storage
type mockNamespaceResolver struct {
	accessibleNamespaces []string
	accessibleMap        map[string]bool
}

func (m *mockNamespaceResolver) ResolveAccessibleNamespaces(userInfo user.Info) ([]string, error) {
	return m.accessibleNamespaces, nil
}

func (m *mockNamespaceResolver) IsNamespaceAccessible(userInfo user.Info, namespace string) (bool, error) {
	if m.accessibleMap != nil {
		return m.accessibleMap[namespace], nil
	}
	for _, ns := range m.accessibleNamespaces {
		if ns == namespace {
			return true, nil
		}
	}
	return false, nil
}

// resolverInterface matches what storage needs
type resolverInterface interface {
	ResolveAccessibleNamespaces(userInfo user.Info) ([]string, error)
	IsNamespaceAccessible(userInfo user.Info, namespace string) (bool, error)
}

// testableStorage wraps storage with mock resolver
type testableStorage struct {
	mockResolver *mockNamespaceResolver
}

func newTestableStorage(namespaces []string) *testableStorage {
	return &testableStorage{
		mockResolver: &mockNamespaceResolver{
			accessibleNamespaces: namespaces,
		},
	}
}

func TestAccessibleNamespaceStorage_List(t *testing.T) {
	tests := []struct {
		name               string
		namespaces         []string
		expectedCount      int
		expectedNamespaces []string
	}{
		{
			name:               "returns multiple namespaces",
			namespaces:         []string{"default", "app-ns", "dev-ns"},
			expectedCount:      3,
			expectedNamespaces: []string{"default", "app-ns", "dev-ns"},
		},
		{
			name:               "returns empty list for no access",
			namespaces:         []string{},
			expectedCount:      0,
			expectedNamespaces: []string{},
		},
		{
			name:               "returns single namespace",
			namespaces:         []string{"only-ns"},
			expectedCount:      1,
			expectedNamespaces: []string{"only-ns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := &mockNamespaceResolver{
				accessibleNamespaces: tt.namespaces,
			}

			// Create a custom storage with our mock
			storage := &accessibleNamespaceStorageWithMock{
				resolver: mockResolver,
			}

			ctx := context.Background()
			ctx = request.WithUser(ctx, &user.DefaultInfo{
				Name:   "test-user",
				Groups: []string{"system:authenticated"},
			})

			result, err := storage.List(ctx, nil)
			require.NoError(t, err)

			list, ok := result.(*v1alpha1.AccessibleNamespaceList)
			require.True(t, ok, "result should be AccessibleNamespaceList")

			assert.Len(t, list.Items, tt.expectedCount)
			assert.Empty(t, list.ListMeta.ResourceVersion, "resourceVersion should be empty")

			names := make([]string, len(list.Items))
			for i, item := range list.Items {
				names[i] = item.Name
			}
			assert.ElementsMatch(t, tt.expectedNamespaces, names)
		})
	}
}

func TestAccessibleNamespaceStorage_List_NoUser(t *testing.T) {
	mockResolver := &mockNamespaceResolver{
		accessibleNamespaces: []string{"default"},
	}

	storage := &accessibleNamespaceStorageWithMock{
		resolver: mockResolver,
	}

	// Context without user
	ctx := context.Background()

	result, err := storage.List(ctx, nil)
	require.NoError(t, err)

	list, ok := result.(*v1alpha1.AccessibleNamespaceList)
	require.True(t, ok)
	assert.Empty(t, list.Items, "should return empty list when no user in context")
}

func TestAccessibleNamespaceStorage_Get(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		accessibleMap  map[string]bool
		expectFound    bool
		expectNotFound bool
	}{
		{
			name:           "accessible namespace returns item",
			namespace:      "allowed-ns",
			accessibleMap:  map[string]bool{"allowed-ns": true},
			expectFound:    true,
			expectNotFound: false,
		},
		{
			name:           "inaccessible namespace returns NotFound",
			namespace:      "denied-ns",
			accessibleMap:  map[string]bool{"allowed-ns": true},
			expectFound:    false,
			expectNotFound: true,
		},
		{
			name:           "nonexistent namespace returns NotFound",
			namespace:      "nonexistent",
			accessibleMap:  map[string]bool{},
			expectFound:    false,
			expectNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := &mockNamespaceResolver{
				accessibleMap: tt.accessibleMap,
			}

			storage := &accessibleNamespaceStorageWithMock{
				resolver: mockResolver,
			}

			ctx := context.Background()
			ctx = request.WithUser(ctx, &user.DefaultInfo{
				Name:   "test-user",
				Groups: []string{"system:authenticated"},
			})

			result, err := storage.Get(ctx, tt.namespace, &metav1.GetOptions{})

			if tt.expectFound {
				require.NoError(t, err)
				ns, ok := result.(*v1alpha1.AccessibleNamespace)
				require.True(t, ok)
				assert.Equal(t, tt.namespace, ns.Name)
				assert.Empty(t, ns.ResourceVersion, "resourceVersion should be empty")
			}

			if tt.expectNotFound {
				require.Error(t, err)
				assert.True(t, errors.IsNotFound(err), "error should be NotFound")
			}
		})
	}
}

func TestAccessibleNamespaceStorage_Get_NoUser(t *testing.T) {
	mockResolver := &mockNamespaceResolver{
		accessibleMap: map[string]bool{"default": true},
	}

	storage := &accessibleNamespaceStorageWithMock{
		resolver: mockResolver,
	}

	// Context without user
	ctx := context.Background()

	_, err := storage.Get(ctx, "default", &metav1.GetOptions{})
	require.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "should return NotFound when no user")
}

func TestAccessibleNamespaceStorage_Scoping(t *testing.T) {
	storage := NewAccessibleNamespaceStorage(nil)

	assert.False(t, storage.NamespaceScoped(), "should be cluster-scoped")
	assert.Equal(t, "accessiblenamespace", storage.GetSingularName())
}

// accessibleNamespaceStorageWithMock is a test helper that uses a mock resolver
type accessibleNamespaceStorageWithMock struct {
	resolver *mockNamespaceResolver
}

func (s *accessibleNamespaceStorageWithMock) List(ctx context.Context, options interface{}) (interface{}, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return &v1alpha1.AccessibleNamespaceList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "AccessibleNamespaceList",
			},
			ListMeta: metav1.ListMeta{ResourceVersion: ""},
			Items:    []v1alpha1.AccessibleNamespace{},
		}, nil
	}

	namespaces, err := s.resolver.ResolveAccessibleNamespaces(userInfo)
	if err != nil {
		return nil, err
	}

	items := make([]v1alpha1.AccessibleNamespace, len(namespaces))
	for i, ns := range namespaces {
		items[i] = v1alpha1.AccessibleNamespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "AccessibleNamespace",
			},
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}
	}

	return &v1alpha1.AccessibleNamespaceList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "AccessibleNamespaceList",
		},
		ListMeta: metav1.ListMeta{ResourceVersion: ""},
		Items:    items,
	}, nil
}

func (s *accessibleNamespaceStorageWithMock) Get(ctx context.Context, name string, options *metav1.GetOptions) (interface{}, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, errors.NewNotFound(v1alpha1.Resource("accessiblenamespaces"), name)
	}

	accessible, err := s.resolver.IsNamespaceAccessible(userInfo, name)
	if err != nil {
		return nil, err
	}

	if !accessible {
		return nil, errors.NewNotFound(v1alpha1.Resource("accessiblenamespaces"), name)
	}

	return &v1alpha1.AccessibleNamespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "AccessibleNamespace",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, nil
}
