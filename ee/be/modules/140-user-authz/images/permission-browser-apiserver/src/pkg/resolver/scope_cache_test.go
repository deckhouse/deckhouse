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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// TestScope_KnownVsUnknown pins the three-way answer used by multi-tenancy:
// known namespaced, known cluster-scoped, and !known. IsNamespaced must keep
// coercing !known to cluster-scoped so AccessibleNamespaces does not inflate.
func TestScope_KnownVsUnknown(t *testing.T) {
	cache := &ResourceScopeCache{
		scopeMap: map[string]bool{
			"/pods":  true,
			"/nodes": false,
		},
	}

	pods := cache.ScopeOf("", "pods")
	assert.True(t, pods.Namespaced)
	assert.True(t, pods.Known)

	nodes := cache.ScopeOf("", "nodes")
	assert.False(t, nodes.Namespaced)
	assert.True(t, nodes.Known)

	unknown := cache.ScopeOf("custom.example.com", "unknownresource")
	assert.False(t, unknown.Namespaced)
	assert.False(t, unknown.Known)
	assert.False(t, cache.IsNamespaced("custom.example.com", "unknownresource"))
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

// TestRefresh_PartialDiscoveryError_UsesPartialResults tests that the groups discovery did
// return are taken even when other groups failed.
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
	partialErr := &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: "metrics.k8s.io", Version: "v1beta1"}: fmt.Errorf("the server is currently unable to handle the request"),
	}}
	cache := &ResourceScopeCache{
		discoveryClient: newMockDiscovery(partialResources, partialErr),
		scopeMap:        make(map[string]bool),
	}

	cache.refresh()

	// Should use partial results
	assert.True(t, cache.IsNamespaced("", "pods"))
	assert.False(t, cache.IsNamespaced("", "nodes"))
}

// TestRefresh_PartialDiscoveryError_KeepsTheFailedGroups tests that a group whose discovery
// failed keeps its previous entries (a down APIService must not turn its cluster-scoped
// resources into !known, which the multi-tenancy engine treats as namespaced), that a group
// discovery did return is replaced, and that a group that is neither returned nor failed is gone.
func TestRefresh_PartialDiscoveryError_KeepsTheFailedGroups(t *testing.T) {
	cache := &ResourceScopeCache{
		discoveryClient: newMockDiscovery(
			[]*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}},
				},
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}},
				},
			},
			&discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
				{Group: "metrics.k8s.io", Version: "v1beta1"}: fmt.Errorf("the server is currently unable to handle the request"),
			}},
		),
		scopeMap: map[string]bool{
			"/pods":                       true,
			"apps/deployments":            true,
			"apps/gone-in-this-version":   true,  // apps was returned: its old entries are replaced
			"metrics.k8s.io/nodes":        false, // metrics.k8s.io failed: its entries are kept
			"metrics.k8s.io/pods":         true,
			"removed.example.com/widgets": false, // neither returned nor failed: the group is gone
		},
	}

	cache.refresh()

	metricsNodes := cache.ScopeOf("metrics.k8s.io", "nodes")
	assert.True(t, metricsNodes.Known, "the failed group must keep its entries")
	assert.False(t, metricsNodes.Namespaced)
	assert.True(t, cache.ScopeOf("metrics.k8s.io", "pods").Known)

	assert.True(t, cache.ScopeOf("apps", "deployments").Known)
	assert.False(t, cache.ScopeOf("apps", "gone-in-this-version").Known, "a returned group is replaced by what discovery returned")

	assert.False(t, cache.ScopeOf("removed.example.com", "widgets").Known, "a group that discovery neither returned nor reported as failed has left the cluster")

	// Absent is how the multi-tenancy decision tells "this resource does not exist" from "we could
	// not read this group". Getting it wrong is not a missing feature, it is a grant: a group whose
	// entries were never read would look absent, absent means no opinion for a cluster-scoped
	// request, and the rule's cluster-wide binding would then let RBAC serve a cluster-wide list of
	// a namespaced resource to a subject limited to one namespace.
	//
	// Asserted through the one method the decision calls. It used to be asserted through the
	// separate predicate the caller combined itself, which is exactly the shape that let a test
	// fake combine them differently and stay green.
	assert.False(t, cache.ScopeOf("metrics.k8s.io", "never-heard-of").Absent,
		"a group discovery could not read: a resource missing from it is missing because nobody looked")
	assert.True(t, cache.ScopeOf("apps", "never-heard-of").Absent,
		"a group discovery did read: a resource missing from it genuinely does not exist")
	assert.True(t, cache.ScopeOf("", "never-heard-of").Absent, "the core group was read")
	assert.True(t, cache.ScopeOf("removed.example.com", "widgets").Absent,
		"a group that has left the cluster is absent, not unreadable")
}

// A healthy refresh must clear a group that was unavailable before, or the engine keeps denying
// cluster-scoped requests for it after the APIService comes back.
func TestGroupUnavailable_ClearedByAHealthyRefresh(t *testing.T) {
	failing := newMockDiscovery(
		[]*metav1.APIResourceList{{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}},
		}},
		&discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
			{Group: "metrics.k8s.io", Version: "v1beta1"}: fmt.Errorf("the server is currently unable to handle the request"),
		}},
	)
	cache := &ResourceScopeCache{discoveryClient: failing, scopeMap: make(map[string]bool)}
	cache.refresh()
	assert.False(t, cache.ScopeOf("metrics.k8s.io", "nodes").Absent, "the group could not be read, so nothing in it is known to be absent")

	cache.discoveryClient = newMockDiscovery([]*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}},
		},
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{{Name: "nodes", Namespaced: false, Kind: "NodeMetrics"}},
		},
	}, nil)
	cache.refresh()

	nodes := cache.ScopeOf("metrics.k8s.io", "nodes")
	assert.True(t, nodes.Known, "the group answered this time")
	assert.False(t, nodes.Namespaced)
	assert.True(t, cache.ScopeOf("metrics.k8s.io", "never-heard-of").Absent,
		"and a resource missing from a group that answered is absent again, so cluster-scoped requests for it stop being denied")
}

// A typed nil handed to an interface value must answer like a nil interface, which is what
// engine.resourceScopeOf relies on: nothing known, nothing absent, so the decision fails closed.
func TestScopeOf_NilReceiver(t *testing.T) {
	var cache *ResourceScopeCache
	scope := cache.ScopeOf("anything", "at-all")
	assert.False(t, scope.Known)
	assert.False(t, scope.Absent)
	assert.False(t, scope.Namespaced)
}

// TestRefresh_NonDiscoveryError_PreservesCache tests that an error other than a partial group
// failure preserves the whole snapshot even if some lists came back with it.
func TestRefresh_NonDiscoveryError_PreservesCache(t *testing.T) {
	cache := &ResourceScopeCache{
		discoveryClient: newMockDiscovery(
			[]*metav1.APIResourceList{{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true}}}},
			fmt.Errorf("connection refused"),
		),
		scopeMap: map[string]bool{"/nodes": false},
	}

	cache.refresh()

	assert.True(t, cache.ScopeOf("", "nodes").Known, "the previous snapshot must survive an unclassified discovery error")
	assert.False(t, cache.ScopeOf("", "pods").Known)
}

// TestNilCache_AnswersLikeEmpty tests that a nil *ResourceScopeCache, which a caller may hand to
// an interface value by mistake, behaves like an empty cache instead of panicking.
// waitForLoop waits until the refresh loop has started, which is what arms the miss trigger.
func waitForLoop(t *testing.T, cache *ResourceScopeCache) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cache.looping.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the refresh loop did not start")
		}
		time.Sleep(time.Millisecond)
	}
}

// A resource the snapshot does not hold pulls the next refresh forward.
//
// The authorization webhook re-lists a group within ten seconds of being asked about something it
// does not hold; this cache used to wait out its five-minute cycle. Both feed the same decision, so
// for those five minutes this apiserver reported one answer and the API server enforced another.
func TestScopeOf_AMissPullsTheRefreshForward(t *testing.T) {
	discovery := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(discovery)
	// Long enough that a test that passes cannot be passing because of the cycle - the miss is
	// the only thing that can refresh within the next hour. The loop still has to be running,
	// because a miss pulls the next scheduled refresh forward and there is nothing to pull
	// otherwise.
	cache.refreshInterval = time.Hour
	cache.bootstrapInterval = time.Hour
	stop := make(chan struct{})
	defer close(stop)
	go cache.StartRefreshLoop(stop)
	waitForLoop(t, cache)

	// Read the snapshot without going through ScopeOf, which would schedule the refresh this test
	// is about and race with the line below it.
	if cache.HasResource("example.com", "widgets") {
		t.Fatal("before the CRD exists the snapshot must not carry it")
	}

	// The CRD is installed, and then somebody asks about it.
	discovery.serve(append(testAPIResources(), &metav1.APIResourceList{
		GroupVersion: "example.com/v1",
		APIResources: []metav1.APIResource{{Name: "widgets", Namespaced: true}},
	}))
	if scope := cache.ScopeOf("example.com", "widgets"); scope.Known {
		t.Fatalf("the snapshot cannot know it yet: got %+v", scope)
	}

	// That miss scheduled the refresh; it converges without anybody waiting an hour.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if scope := cache.ScopeOf("example.com", "widgets"); scope.Known {
			if !scope.Namespaced {
				t.Fatalf("after the refresh: got %+v, want a namespaced resource", scope)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the miss never produced a refresh: the snapshot is still the old one")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// And the misses are rate-limited: refresh() lists every group in the cluster.
func TestScopeOf_MissesDoNotRefreshOnEveryLookup(t *testing.T) {
	discovery := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(discovery)
	cache.refreshInterval = time.Hour
	cache.bootstrapInterval = time.Hour
	now := time.Now()
	cache.now = func() time.Time { return now }
	stop := make(chan struct{})
	defer close(stop)
	go cache.StartRefreshLoop(stop)
	waitForLoop(t, cache)

	after := func() int {
		// The refresh a miss triggers runs in its own goroutine; give it room to finish before
		// counting, or the test measures scheduling rather than the rate limit.
		time.Sleep(200 * time.Millisecond)
		return discovery.listingCount()
	}

	constructed := discovery.listingCount()
	for i := 0; i < 200; i++ {
		cache.ScopeOf("example.com", "widgets")
		cache.ScopeOf("example.com", "gadgets")
	}
	if got := after() - constructed; got != 1 {
		t.Errorf("400 lookups of resources that do not exist produced %d refreshes, want 1", got)
	}

	// Once the interval has passed, one more, so a CRD installed meanwhile is still noticed.
	now = now.Add(missRefreshInterval)
	cache.ScopeOf("example.com", "widgets")
	if got := after() - constructed; got != 2 {
		t.Errorf("after the interval: %d refreshes in total, want 2", got)
	}
}

func TestNilCache_AnswersLikeEmpty(t *testing.T) {
	var cache *ResourceScopeCache

	scope := cache.ScopeOf("", "pods")
	assert.False(t, scope.Namespaced)
	assert.False(t, scope.Known)
	assert.False(t, scope.Absent, "a nil cache knows nothing, which is not the same as knowing the resource is gone")
	assert.False(t, cache.HasData())
	assert.False(t, cache.HasResource("", "pods"))
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

// TestRefresh_IncludesSubresources tests that subresources participate in RBAC wildcard matching.
func TestRefresh_IncludesSubresources(t *testing.T) {
	client := newMockDiscovery(testAPIResources(), nil)
	cache := NewResourceScopeCache(client)

	assert.True(t, cache.IsNamespaced("", "pods/status"),
		"namespaced subresources must be retained for exact RBAC matching")
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
		discoveryClient:   client,
		refreshInterval:   2 * time.Second,
		bootstrapInterval: 5 * time.Millisecond,
		scopeMap:          make(map[string]bool),
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
//
// It is guarded by a mutex and counts its listings so a test can change what the cluster serves
// while the cache is running and see how often the cache asked.
type mockDiscovery struct {
	mu        sync.Mutex
	resources []*metav1.APIResourceList
	err       error
	listings  int
}

func newMockDiscovery(resources []*metav1.APIResourceList, err error) *mockDiscovery {
	return &mockDiscovery{resources: resources, err: err}
}

// serve replaces what the cluster serves, as installing a CRD would.
func (m *mockDiscovery) serve(resources []*metav1.APIResourceList) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = resources
}

// listingCount is how many times the cache has listed discovery.
func (m *mockDiscovery) listingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listings
}

func (m *mockDiscovery) snapshot() ([]*metav1.APIResourceList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listings++
	return m.resources, m.err
}

func (m *mockDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.snapshot()
}

// The following methods satisfy the discovery.DiscoveryInterface but are not used by ResourceScopeCache.

func (m *mockDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{}, nil
}

func (m *mockDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rl := range m.resources {
		if rl.GroupVersion == groupVersion {
			return rl, nil
		}
	}
	return &metav1.APIResourceList{}, nil
}

func (m *mockDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	resources, err := m.snapshot()
	return nil, resources, err
}

func (m *mockDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return m.snapshot()
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
