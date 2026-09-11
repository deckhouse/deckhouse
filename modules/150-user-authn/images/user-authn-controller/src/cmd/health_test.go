/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type stubSyncCache struct {
	cache.Cache
	synced bool
}

func (s stubSyncCache) WaitForCacheSync(context.Context) bool {
	return s.synced
}

func TestCacheSyncCheck(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	if err := cacheSyncCheck(stubSyncCache{synced: true})(req); err != nil {
		t.Fatalf("synced cache: %v", err)
	}
	if err := cacheSyncCheck(stubSyncCache{synced: false})(req); err == nil {
		t.Fatal("unsynced cache: want error")
	}
}

type stubReader struct {
	client.Reader
	err error
}

func (s stubReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return s.err
}

// Readiness has to say whether the controller can work now. A synced informer cache stays synced
// after the controller loses its RBAC, so cache-sync alone left a powerless controller Ready.
func TestAPIAccessCheck(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	if err := apiAccessCheck(stubReader{})(req); err != nil {
		t.Fatalf("a reader that answers: %v", err)
	}
	forbidden := errors.New(`useraccounts.deckhouse.io is forbidden`)
	if err := apiAccessCheck(stubReader{err: forbidden})(req); !errors.Is(err, forbidden) {
		t.Fatalf("a reader that is forbidden: got %v, want the cause", err)
	}
}
