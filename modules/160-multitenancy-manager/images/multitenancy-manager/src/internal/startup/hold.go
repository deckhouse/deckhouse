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

package startup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// HoldControllers returns a manager whose Add holds every runnable that needs leader election, the
// controllers and the Init runnables of the controller packages, until wait returns. wait itself is
// added to mgr as such a runnable, so it starts once this replica holds the lease, after the manager
// has started its webhook server and caches; the webhooks serve while it runs, and readiness does not
// depend on it. Runnables that run on every replica are added to mgr unchanged.
//
// A wait that fails stops the manager, and nothing held starts. A wait cut short because the manager
// stops releases nothing either.
func HoldControllers(mgr manager.Manager, wait func(context.Context) error) (manager.Manager, error) {
	released := make(chan struct{})
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		err := wait(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("hold the controllers: %w", err)
		}
		close(released)
		return nil
	})); err != nil {
		return nil, fmt.Errorf("add the controller hold: %w", err)
	}
	return &heldManager{Manager: mgr, released: released}, nil
}

// heldManager is a manager.Manager that holds the runnables needing leader election until released
// is closed.
type heldManager struct {
	manager.Manager
	released <-chan struct{}
}

// Add adds r to the manager, held when it needs leader election.
func (m *heldManager) Add(r manager.Runnable) error {
	if !needsLeaderElection(r) {
		return m.Manager.Add(r)
	}
	return m.Manager.Add(&heldRunnable{runnable: r, released: m.released})
}

// needsLeaderElection reports whether the manager starts r on the leader only. It follows the way
// controller-runtime sorts runnables into groups when they are added: a webhook server, a runnable
// with a cache and a runnable that says it needs no leader election run on every replica; the rest,
// a plain RunnableFunc included, run on the leader.
func needsLeaderElection(r manager.Runnable) bool {
	switch runnable := r.(type) {
	case webhook.Server:
		return false
	case interface{ GetCache() cache.Cache }:
		return false
	case manager.LeaderElectionRunnable:
		return runnable.NeedLeaderElection()
	default:
		return true
	}
}

// heldRunnable starts runnable once released is closed.
type heldRunnable struct {
	runnable manager.Runnable
	released <-chan struct{}
}

// Start waits for the release and then runs the held runnable until ctx is done. When ctx is done
// first, it returns nil without starting it.
func (h *heldRunnable) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case <-h.released:
	}
	return h.runnable.Start(ctx)
}

// NeedLeaderElection is true: only runnables that need leader election are held.
func (*heldRunnable) NeedLeaderElection() bool { return true }
