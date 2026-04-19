/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// TestEngine_ConcurrentAuthorize tests that concurrent calls to Authorize don't race
func TestEngine_ConcurrentAuthorize(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"user1": {
					LimitNamespaces:        []*regexp.Regexp{regexp.MustCompile("^ns-.*$")},
					NamespaceFiltersAbsent: false,
				},
				"user2": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
			},
			"Group": {
				"developers": {
					LimitNamespaces:        []*regexp.Regexp{regexp.MustCompile("^dev-.*$")},
					NamespaceFiltersAbsent: false,
				},
			},
			"ServiceAccount": {},
		},
	}

	ctx := context.Background()
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Alternate between different users and namespaces
				var userName string
				var namespace string
				var groups []string

				switch id % 4 {
				case 0:
					userName = "user1"
					namespace = "ns-test"
				case 1:
					userName = "user2"
					namespace = "kube-system"
				case 2:
					userName = "other"
					namespace = "default"
					groups = []string{"developers"}
				case 3:
					userName = "unknown"
					namespace = "any"
				}

				attrs := &mockAttrs{
					userInfo:   &mockUserInfo{name: userName, groups: groups},
					namespace:  namespace,
					resource:   "pods",
					verb:       "get",
					isResource: true,
				}

				_, _, err := e.Authorize(ctx, attrs)
				require.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}

// TestEngine_ConcurrentDirectoryUpdate tests that updating directory while authorizing doesn't race
func TestEngine_ConcurrentDirectoryUpdate(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User":           make(map[string]DirectoryEntry),
			"Group":          make(map[string]DirectoryEntry),
			"ServiceAccount": make(map[string]DirectoryEntry),
		},
	}

	ctx := context.Background()
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				attrs := &mockAttrs{
					userInfo:   &mockUserInfo{name: "user1"},
					namespace:  "test-ns",
					resource:   "pods",
					verb:       "get",
					isResource: true,
				}

				_, _, _ = e.Authorize(ctx, attrs)
			}
		}()
	}

	// Writers (simulating renewDirectories)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				newDir := map[string]map[string]DirectoryEntry{
					"User": {
						"user1": {
							LimitNamespaces:        []*regexp.Regexp{regexp.MustCompile("^test-.*$")},
							NamespaceFiltersAbsent: false,
						},
					},
					"Group":          make(map[string]DirectoryEntry),
					"ServiceAccount": make(map[string]DirectoryEntry),
				}

				e.mu.Lock()
				e.directory = newDir
				e.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

// TestEngine_ConcurrentNamespacedCacheAccess tests that namespaced cache access doesn't race
func TestEngine_ConcurrentNamespacedCacheAccess(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User":           {},
			"Group":          {},
			"ServiceAccount": {},
		},
		namespacedCache: make(map[string]bool),
	}

	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				cacheKey := "apps/v1/deployments"

				// Read
				e.namespacedCacheMu.RLock()
				_ = e.namespacedCache[cacheKey]
				e.namespacedCacheMu.RUnlock()

				// Write (every 10th iteration)
				if j%10 == 0 {
					e.namespacedCacheMu.Lock()
					e.namespacedCache[cacheKey] = true
					e.namespacedCacheMu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestCompositeAuthorizer_ConcurrentAccess tests composite authorizer under concurrent load
func TestCompositeAuthorizer_ConcurrentAccess(t *testing.T) {
	mt := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted": {
					LimitNamespaces:        []*regexp.Regexp{regexp.MustCompile("^allowed-.*$")},
					NamespaceFiltersAbsent: false,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	rbac := &mockRBACAuthorizer{decision: authorizer.DecisionAllow}

	composite := &compositeAuth{mt: mt, rbac: rbac}

	ctx := context.Background()
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				namespace := "allowed-ns"
				if id%2 == 0 {
					namespace = "denied-ns"
				}

				attrs := &mockAttrs{
					userInfo:   &mockUserInfo{name: "restricted"},
					namespace:  namespace,
					resource:   "pods",
					verb:       "get",
					isResource: true,
				}

				_, _, _ = composite.Authorize(ctx, attrs)
			}
		}(i)
	}

	wg.Wait()
}

// Helper types for testing

type compositeAuth struct {
	mt   *Engine
	rbac authorizer.Authorizer
}

func (c *compositeAuth) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if c.mt != nil {
		decision, reason, err := c.mt.Authorize(ctx, attrs)
		if err != nil || decision == authorizer.DecisionDeny {
			return decision, reason, err
		}
	}
	return c.rbac.Authorize(ctx, attrs)
}

type mockRBACAuthorizer struct {
	decision authorizer.Decision
}

func (m *mockRBACAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, "mock", nil
}
