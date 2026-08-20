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
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type cacheStub struct {
	cache.Cache

	release chan struct{}
	synced  bool
}

func (c *cacheStub) WaitForCacheSync(ctx context.Context) bool {
	select {
	case <-c.release:
		return c.synced
	case <-ctx.Done():
		return false
	}
}

func TestCacheSyncGateBecomesReadyAfterSync(t *testing.T) {
	stub := &cacheStub{release: make(chan struct{}), synced: true}
	gate := newCacheSyncGate(stub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	stopped := make(chan error, 1)
	go func() { stopped <- gate.Start(ctx) }()

	if err := gate.Check(nil); err == nil {
		t.Error("gate is ready before the cache synced")
	}

	close(stub.release)
	waitFor(t, func() bool { return gate.Check(nil) == nil })
	cancel()

	if err := <-stopped; err != nil {
		t.Errorf("start returned %v after a clean shutdown", err)
	}
}

func TestCacheSyncGateFailsWhenCacheDoesNotSync(t *testing.T) {
	stub := &cacheStub{release: make(chan struct{}), synced: false}
	close(stub.release)

	gate := newCacheSyncGate(stub)

	if err := gate.Start(t.Context()); !errors.Is(err, errCacheNotSynced) {
		t.Fatalf("start returned %v, want %v", err, errCacheNotSynced)
	}

	if err := gate.Check(nil); err == nil {
		t.Error("gate is ready although the cache never synced")
	}
}

func TestCacheSyncGateRunsWithoutLeadership(t *testing.T) {
	if newCacheSyncGate(nil).NeedLeaderElection() {
		t.Error("gate requires leadership")
	}
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()

	deadline := time.After(5 * time.Second)

	for {
		if done() {
			return
		}

		select {
		case <-deadline:
			t.Fatal("timed out waiting for the gate to become ready")
		case <-time.After(time.Millisecond):
		}
	}
}
