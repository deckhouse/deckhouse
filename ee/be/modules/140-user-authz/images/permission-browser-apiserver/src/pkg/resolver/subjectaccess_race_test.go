/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestSubjectAccessResolver_ConcurrentReports runs reports in parallel while the
// discovery-backed scope cache refreshes underneath them.
//
// Reports are served straight from shared caches, so the resolver must hold no
// mutable state of its own; run with -race this is what proves it.
func TestSubjectAccessResolver_ConcurrentReports(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("admin", nil, rule([]string{"*"}, []string{"*"}, []string{"*"})),
		clusterRoleBinding("admin-binding", "admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}),
		clusterRole("viewer", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
	}

	for i := range 5 {
		namespace := fmt.Sprintf("team-%d", i)
		objs = append(objs, roleBinding("viewer-binding", namespace, "ClusterRole", "viewer",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}))
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	stopRefresh := make(chan struct{})
	var refresher sync.WaitGroup
	refresher.Add(1)

	go func() {
		defer refresher.Done()
		for {
			select {
			case <-stopRefresh:
				return
			default:
				// Mimic the background discovery refresh replacing the map.
				resolver.scopeCache.mu.Lock()
				resolver.scopeCache.scopeMap["/pods"] = true
				resolver.scopeCache.mu.Unlock()
				_ = resolver.scopeCache.ResourcesMatching([]string{"*"}, []string{"*"})
			}
		}
	}()

	var reporters sync.WaitGroup
	for range 8 {
		reporters.Add(1)
		go func() {
			defer reporters.Done()
			for range 10 {
				status, err := resolver.Report(context.Background(), userRequest("root"))
				require.NoError(t, err)
				require.NotEmpty(t, status.Scopes)
			}
		}()
	}

	reporters.Wait()
	close(stopRefresh)
	refresher.Wait()
}
