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
	"net/http"
	"net/http/httptest"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
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
