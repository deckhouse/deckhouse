/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"fmt"
	"sync"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	restclient "k8s.io/client-go/rest"
)

// TestNewResourceScopeCache tests that a new cache is created and populated
func TestNewResourceScopeCache(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(client)

	require.NotNil(t, cache)
	assert.NotEmpty(t, cache.scopeMap, "scope map should be populated after creation")

	// Verify known resources from mock discovery
	assert.True(t, cache.IsNamespaced("", "pods"), "pods should be namespaced")
	assert.True(t, cache.IsNamespaced("", "services"), "services should be namespaced")
	assert.False(t, cache.IsNamespaced("", "namespaces"), "namespaces should be cluster-scoped")
	assert.False(t, cache.IsNamespaced("", "nodes"), "nodes should be cluster-scoped")
	assert.True(t, cache.IsNamespaced("apps", "deployments"), "deployments should be namespaced")
}

// TestNewResourceScopeCache_NilDiscovery tests creation with nil discovery client
func TestNewResourceScopeCache_NilDiscovery(t *testing.T) {
	cache := NewResourceScopeCache(nil)

	require.NotNil(t, cache)
	assert.Empty(t, cache.scopeMap, "scope map should be empty with nil discovery")
}

// TestIsNamespaced_KnownResources tests lookup for known resources
func TestIsNamespaced_KnownResources(t *testing.T) {
	cache := &ResourceScopeCache{
		scopeMap: map[string]bool{
			"/pods":            true,
			"/namespaces":      false,
			"apps/deployments": true,
			"/nodes":           false,
		},
	}

	tests := []struct {
		name     string
		group    string
		resource string
		expected bool
	}{
		{"pods are namespaced", "", "pods", true},
		{"namespaces are cluster-scoped", "", "namespaces", false},
		{"deployments are namespaced", "apps", "deployments", true},
		{"nodes are cluster-scoped", "", "nodes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, cache.IsNamespaced(tt.group, tt.resource))
		})
	}
}

// TestIsNamespaced_UnknownResource tests fail-closed behavior for unknown resources
func TestIsNamespaced_UnknownResource(t *testing.T) {
	cache := &ResourceScopeCache{
		scopeMap: map[string]bool{
			"/pods": true,
		},
	}

	// Unknown resource should return false (fail-closed: assume cluster-scoped)
	assert.False(t, cache.IsNamespaced("custom.example.com", "unknownresource"),
		"unknown resource should be assumed cluster-scoped")
	assert.False(t, cache.IsNamespaced("", "unknownresource"),
		"unknown core resource should be assumed cluster-scoped")
}

// TestRefresh_UpdatesCache tests that refresh updates the cache with new data
func TestRefresh_UpdatesCache(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := &ResourceScopeCache{
		discoveryClient: client,
		scopeMap:        make(map[string]bool),
	}

	// Initially empty
	assert.Empty(t, cache.scopeMap)

	// After refresh, should be populated
	cache.refresh()
	assert.NotEmpty(t, cache.scopeMap)
	assert.True(t, cache.IsNamespaced("", "pods"))
	assert.False(t, cache.IsNamespaced("", "nodes"))
}

// TestRefresh_DiscoveryError_PreservesCache tests that on error, old cache is preserved
func TestRefresh_DiscoveryError_PreservesCache(t *testing.T) {
	// Start with a populated cache
	cache := &ResourceScopeCache{
		discoveryClient: newMockDiscovery(nil, fmt.Errorf("discovery unavailable")),
		scopeMap: map[string]bool{
			"/pods":       true,
			"/namespaces": false,
		},
	}

	// Refresh with failing discovery should preserve existing cache
	cache.refresh()

	assert.True(t, cache.IsNamespaced("", "pods"), "pods should still be in cache after failed refresh")
	assert.False(t, cache.IsNamespaced("", "namespaces"), "namespaces should still be in cache after failed refresh")
}

// TestRefresh_PartialDiscoveryError_UsesPartialResults tests that partial results are used
func TestRefresh_PartialDiscoveryError_UsesPartialResults(t *testing.T) {
	// Discovery returns partial results with an error
	partialResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "nodes", Namespaced: false, Kind: "Node"},
			},
		},
	}
	cache := &ResourceScopeCache{
		discoveryClient: newMockDiscovery(partialResources, fmt.Errorf("partial error")),
		scopeMap:        make(map[string]bool),
	}

	cache.refresh()

	// Should use partial results
	assert.True(t, cache.IsNamespaced("", "pods"))
	assert.False(t, cache.IsNamespaced("", "nodes"))
}

// TestRefresh_NilDiscovery tests refresh with nil discovery client
func TestRefresh_NilDiscovery(t *testing.T) {
	cache := &ResourceScopeCache{
		discoveryClient: nil,
		scopeMap: map[string]bool{
			"/pods": true,
		},
	}

	// Should not panic and should preserve existing cache
	cache.refresh()
	assert.True(t, cache.IsNamespaced("", "pods"))
}

// TestRefresh_SkipsSubresources tests that subresources are not included in the cache
func TestRefresh_SkipsSubresources(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(client)

	// "pods/status" should not be in the cache
	assert.False(t, cache.IsNamespaced("", "pods/status"),
		"subresources should not be in the cache")
}

// TestStartRefreshLoop_StopsOnCancel tests that the refresh loop stops when stopCh is closed
func TestStartRefreshLoop_StopsOnCancel(t *testing.T) {
	cache := &ResourceScopeCache{
		discoveryClient: nil,
		refreshInterval: 10 * time.Millisecond,
		scopeMap:        make(map[string]bool),
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})

	go func() {
		cache.StartRefreshLoop(stopCh)
		close(done)
	}()

	// Let it run a few cycles
	time.Sleep(50 * time.Millisecond)

	// Stop the loop
	close(stopCh)

	// Should stop within a reasonable time
	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("StartRefreshLoop did not stop within timeout")
	}
}

func TestStartRefreshLoop_BootstrapRefreshesBeforeRegularInterval(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)

	// Ensure we don't wait for refreshInterval (2s) while the cache is empty.
	cache := &ResourceScopeCache{
		discoveryClient:    client,
		refreshInterval:    2 * time.Second,
		bootstrapInterval:  5 * time.Millisecond,
		scopeMap:           make(map[string]bool),
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	go cache.StartRefreshLoop(stopCh)

	require.Eventually(t, func() bool {
		return cache.HasData()
	}, 250*time.Millisecond, 5*time.Millisecond, "cache should be populated via bootstrap refresh")

	// Basic sanity: after bootstrap refresh, known resources should be available.
	assert.True(t, cache.IsNamespaced("", "pods"))
	assert.False(t, cache.IsNamespaced("", "namespaces"))
}

// TestRefresh_ParsesGroupVersionCorrectly tests correct parsing of GroupVersion strings
func TestRefresh_ParsesGroupVersionCorrectly(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(client)

	// Core API (GroupVersion = "v1") should have group = ""
	assert.True(t, cache.IsNamespaced("", "pods"), "core API pods should be namespaced")
	assert.False(t, cache.IsNamespaced("", "nodes"), "core API nodes should be cluster-scoped")

	// apps/v1 should have group = "apps"
	assert.True(t, cache.IsNamespaced("apps", "deployments"), "apps/deployments should be namespaced")

	// rbac.authorization.k8s.io/v1 should have group = "rbac.authorization.k8s.io"
	assert.True(t, cache.IsNamespaced("rbac.authorization.k8s.io", "roles"), "roles should be namespaced")
	assert.False(t, cache.IsNamespaced("rbac.authorization.k8s.io", "clusterroles"), "clusterroles should be cluster-scoped")
}

// --- Race tests ---

// TestResourceScopeCache_ConcurrentReads tests that concurrent IsNamespaced calls don't race
func TestResourceScopeCache_ConcurrentReads(t *testing.T) {
	cache := &ResourceScopeCache{
		scopeMap: map[string]bool{
			"/pods":            true,
			"/namespaces":      false,
			"apps/deployments": true,
			"/nodes":           false,
		},
	}

	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				switch id % 4 {
				case 0:
					cache.IsNamespaced("", "pods")
				case 1:
					cache.IsNamespaced("", "namespaces")
				case 2:
					cache.IsNamespaced("apps", "deployments")
				case 3:
					cache.IsNamespaced("unknown", "resource")
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestResourceScopeCache_ConcurrentReadWrite tests IsNamespaced concurrent with refresh
func TestResourceScopeCache_ConcurrentReadWrite(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(client)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				cache.IsNamespaced("", "pods")
				cache.IsNamespaced("", "namespaces")
				cache.IsNamespaced("apps", "deployments")
				cache.IsNamespaced("unknown", "resource")
			}
		}()
	}

	// Writers (simulating refresh)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				cache.refresh()
			}
		}()
	}

	wg.Wait()
}

// TestResourceScopeCache_ConcurrentRefreshLoop tests refresh loop concurrent with reads
func TestResourceScopeCache_ConcurrentRefreshLoop(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := &ResourceScopeCache{
		discoveryClient: client,
		refreshInterval: 5 * time.Millisecond,
		scopeMap:        make(map[string]bool),
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	go cache.StartRefreshLoop(stopCh)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				cache.IsNamespaced("", "pods")
				cache.IsNamespaced("apps", "deployments")
				cache.IsNamespaced("", "nodes")
			}
		}()
	}

	wg.Wait()
}

// --- Test helpers ---

// testAPIResources returns a realistic set of API resources for testing.
func testAPIResources() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "pods/status", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"get", "patch"}},
				{Name: "services", Namespaced: true, Kind: "Service", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "secrets", Namespaced: true, Kind: "Secret", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "serviceaccounts", Namespaced: true, Kind: "ServiceAccount", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "namespaces", Namespaced: false, Kind: "Namespace", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "persistentvolumes", Namespaced: false, Kind: "PersistentVolume", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "roles", Namespaced: true, Kind: "Role", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "rolebindings", Namespaced: true, Kind: "RoleBinding", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "clusterroles", Namespaced: false, Kind: "ClusterRole", Verbs: metav1.Verbs{"get", "list"}},
				{Name: "clusterrolebindings", Namespaced: false, Kind: "ClusterRoleBinding", Verbs: metav1.Verbs{"get", "list"}},
			},
		},
	}
}

// mockDiscovery implements discovery.DiscoveryInterface for testing.
// Only ServerPreferredResources is implemented; other methods return zero values.
type mockDiscovery struct {
	resources []*metav1.APIResourceList
	err       error
}

func newMockDiscovery(resources []*metav1.APIResourceList, err error) *mockDiscovery {
	return &mockDiscovery{resources: resources, err: err}
}

func (m *mockDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.resources, m.err
}

// The following methods satisfy the discovery.DiscoveryInterface but are not used by ResourceScopeCache.

func (m *mockDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{}, nil
}

func (m *mockDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, rl := range m.resources {
		if rl.GroupVersion == groupVersion {
			return rl, nil
		}
	}
	return &metav1.APIResourceList{}, nil
}

func (m *mockDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, m.resources, m.err
}

func (m *mockDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return m.resources, m.err
}

func (m *mockDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (m *mockDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (m *mockDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (m *mockDiscovery) RESTClient() restclient.Interface {
	return nil
}

func (m *mockDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return m
}
