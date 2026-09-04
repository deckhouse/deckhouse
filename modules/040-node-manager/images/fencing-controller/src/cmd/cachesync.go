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
	"sync/atomic"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

var errCacheNotSynced = errors.New("informer cache is not synced yet")

type cacheSyncGate struct {
	cache  cache.Cache
	synced atomic.Bool
}

func newCacheSyncGate(c cache.Cache) *cacheSyncGate {
	return &cacheSyncGate{cache: c}
}

// The readiness check must run on both the leader and standby replicas.
func (g *cacheSyncGate) NeedLeaderElection() bool {
	return false
}

func (g *cacheSyncGate) Start(ctx context.Context) error {
	if !g.cache.WaitForCacheSync(ctx) {
		return errCacheNotSynced
	}

	g.synced.Store(true)

	<-ctx.Done()

	return nil
}

func (g *cacheSyncGate) Check(_ *http.Request) error {
	if !g.synced.Load() {
		return errCacheNotSynced
	}

	return nil
}
