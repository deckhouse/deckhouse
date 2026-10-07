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
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func TestHoldControllers(t *testing.T) {
	t.Run("held runnables start once the wait returns", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			inner := &fakeManager{}
			held, err := HoldControllers(inner, func(ctx context.Context) error {
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			require.NoError(t, err)
			controller := &electedRunnable{isElected: true}
			initRunnable := &plainRunnable{}
			require.NoError(t, held.Add(controller))
			require.NoError(t, held.Add(initRunnable))
			require.Len(t, inner.added, 3, "the wait and the two held runnables")
			assert.True(t, needsLeaderElection(inner.added[0]), "the wait runs on the leader only")

			ctx, cancel := context.WithCancel(t.Context())
			results := startAll(ctx, inner.added)
			synctest.Wait()
			assert.False(t, controller.hasStarted(), "a controller is held while the wait runs")
			assert.False(t, initRunnable.hasStarted(), "an Init runnable is held while the wait runs")

			close(release)
			synctest.Wait()
			assert.True(t, controller.hasStarted())
			assert.True(t, initRunnable.hasStarted())

			cancel()
			for i, err := range results() {
				assert.NoError(t, err, "runnable %d", i)
			}
		})
	})

	t.Run("a stop during the wait starts nothing", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			inner := &fakeManager{}
			held, err := HoldControllers(inner, func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			})
			require.NoError(t, err)
			controller := &electedRunnable{isElected: true}
			require.NoError(t, held.Add(controller))

			ctx, cancel := context.WithCancel(t.Context())
			results := startAll(ctx, inner.added)
			synctest.Wait()
			cancel()

			for i, err := range results() {
				assert.NoError(t, err, "runnable %d returns nil on a stop", i)
			}
			assert.False(t, controller.hasStarted())
		})
	})

	t.Run("a failed wait stops the manager and starts nothing", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			errList := errors.New("list pods: forbidden")
			inner := &fakeManager{}
			held, err := HoldControllers(inner, func(context.Context) error { return errList })
			require.NoError(t, err)
			controller := &electedRunnable{isElected: true}
			require.NoError(t, held.Add(controller))

			ctx, cancel := context.WithCancel(t.Context())
			results := startAll(ctx, inner.added)
			synctest.Wait()
			assert.False(t, controller.hasStarted())
			cancel()

			errs := results()
			require.ErrorIs(t, errs[0], errList, "the wait runnable returns the error, which stops the manager")
			assert.NoError(t, errs[1])
			assert.False(t, controller.hasStarted())
		})
	})

	// The same with the manager of controller-runtime, which sorts the runnables into its groups: the
	// wait and the held runnables go with the leader, a runnable of every replica starts without
	// waiting. No API server is needed: the cache and the REST mapper are fakes, and leader election
	// is off, which the manager treats as holding the lease.
	t.Run("controller-runtime manager", func(t *testing.T) {
		mgr, err := manager.New(&rest.Config{Host: "https://127.0.0.1:1"}, manager.Options{
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			NewCache: func(*rest.Config, cache.Options) (cache.Cache, error) {
				return &informertest.FakeInformers{}, nil
			},
			MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
				return meta.NewDefaultRESTMapper(nil), nil
			},
		})
		require.NoError(t, err)
		waitStarted := make(chan struct{})
		release := make(chan struct{})
		held, err := HoldControllers(mgr, func(ctx context.Context) error {
			close(waitStarted)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		require.NoError(t, err)
		controller := &electedRunnable{isElected: true}
		everyReplica := &electedRunnable{isElected: false}
		require.NoError(t, held.Add(controller))
		require.NoError(t, held.Add(everyReplica))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		stopped := make(chan error, 1)
		go func() { stopped <- mgr.Start(ctx) }()

		<-waitStarted
		require.Eventually(t, everyReplica.hasStarted, 5*time.Second, 10*time.Millisecond,
			"a runnable of every replica does not wait")
		assert.False(t, controller.hasStarted(), "a controller is held while the wait runs")

		close(release)
		require.Eventually(t, controller.hasStarted, 5*time.Second, 10*time.Millisecond)

		cancel()
		require.NoError(t, <-stopped)
	})
}

func TestHeldManager_Add(t *testing.T) {
	tests := []struct {
		name         string
		runnable     manager.Runnable
		expectedHeld bool
	}{
		{name: "controller", runnable: &electedRunnable{isElected: true}, expectedHeld: true},
		{name: "runnable func", runnable: manager.RunnableFunc(func(context.Context) error { return nil }), expectedHeld: true},
		{name: "runnable of every replica", runnable: &electedRunnable{isElected: false}},
		{name: "webhook server", runnable: webhook.NewServer(webhook.Options{})},
		{name: "cache", runnable: &cachedRunnable{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &fakeManager{}
			held, err := HoldControllers(inner, func(context.Context) error { return nil })
			require.NoError(t, err)

			require.NoError(t, held.Add(tt.runnable))

			require.Len(t, inner.added, 2)
			added := inner.added[1]
			if !tt.expectedHeld {
				assert.Same(t, tt.runnable, added, "added to the manager as it is")
				return
			}
			assert.IsType(t, &heldRunnable{}, added)
			assert.True(t, needsLeaderElection(added), "a held runnable still runs on the leader only")
		})
	}
}

// fakeManager records the runnables added to it; HoldControllers calls nothing else on it.
type fakeManager struct {
	manager.Manager
	added []manager.Runnable
}

func (m *fakeManager) Add(r manager.Runnable) error {
	m.added = append(m.added, r)
	return nil
}

// startAll starts every runnable the way the manager does, each in its own goroutine, and returns a
// function that waits for all of them to return and gives their errors in order.
func startAll(ctx context.Context, runnables []manager.Runnable) func() []error {
	errs := make([]error, len(runnables))
	var wg sync.WaitGroup
	for i, r := range runnables {
		wg.Go(func() { errs[i] = r.Start(ctx) })
	}
	return func() []error {
		wg.Wait()
		return errs
	}
}

// plainRunnable runs until its context is done, like the Init runnables that implement nothing else.
type plainRunnable struct {
	isStarted atomic.Bool
}

func (r *plainRunnable) Start(ctx context.Context) error {
	r.isStarted.Store(true)
	<-ctx.Done()
	return nil
}

func (r *plainRunnable) hasStarted() bool { return r.isStarted.Load() }

// electedRunnable is a runnable that tells whether it needs leader election, like a controller.
type electedRunnable struct {
	plainRunnable
	isElected bool
}

func (r *electedRunnable) NeedLeaderElection() bool { return r.isElected }

// cachedRunnable is a runnable with a cache, which the manager starts on every replica.
type cachedRunnable struct {
	plainRunnable
}

func (*cachedRunnable) GetCache() cache.Cache { return nil }
