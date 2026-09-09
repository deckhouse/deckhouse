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

package gc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingStore records the read-only window, which is the thing that must always close.
type recordingStore struct {
	mutex      sync.Mutex
	quiesced   int
	restored   int
	readOnly   bool
	quiesceErr error
}

func (s *recordingStore) Quiesce(context.Context) (func(), error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.quiesceErr != nil {
		return nil, s.quiesceErr
	}
	s.quiesced++
	s.readOnly = true

	return func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.restored++
		s.readOnly = false
	}, nil
}

func (s *recordingStore) state() (quiesced, restored int, readOnly bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.quiesced, s.restored, s.readOnly
}

// directLease grants immediately, standing in for a lease nobody else holds.
type directLease struct{ held int }

func (l *directLease) Hold(ctx context.Context, work func(context.Context) error) error {
	l.held++
	return work(ctx)
}

// busyLease never grants, standing in for another replica collecting for the whole window.
type busyLease struct{}

func (busyLease) Hold(ctx context.Context, _ func(context.Context) error) error {
	<-ctx.Done()
	return ctx.Err()
}

// newScheduler takes a store it no longer gives to the scheduler: nothing about a collection touches
// the registry any more — the syncer stops writing to the cache instead, so serving is never
// interrupted. The parameter stays so the call sites read as before.
func newScheduler(t *testing.T, _ *recordingStore, lease Lease, collect func(context.Context) (Report, error)) *Scheduler {
	t.Helper()

	return &Scheduler{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plan:    func() Plan { return Plan{Known: true, Enabled: true, Schedule: "* * * * *"} },
		Lease:   lease,
		Collect: collect,
		Window:  100 * time.Millisecond,
	}
}

// TestCollectingDecidesWithoutQuietingTheStore is why the read-only moved.
//
// Applying it restarts the serving process — the registry cannot reload its configuration, so it is
// signalled and the kubelet brings it back, which the kubelet counts as a crash. Around the whole
// collection that happened twice every fifteen minutes whether or not anything was deletable:
// measured on a cluster as seven restarts per replica, exponential backoff, and a store answering
// `connection refused` for minutes. In that state followers could not replicate and the collection
// failed on a read it had itself made impossible.
func TestCollectingDecidesWithoutQuietingTheStore(t *testing.T) {
	store := &recordingStore{}
	collected := false
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		collected = true
		return Report{}, nil
	})

	require.NoError(t, scheduler.collect(context.Background()))
	assert.True(t, collected)

	quiesced, _, _ := store.state()
	assert.Zero(t, quiesced,
		"a collection that deletes nothing must not restart the registry twice for the privilege")
}

// TestCollectPublishesFailuresToo: "it has not collected since Tuesday" is the useful fact,
// and it is invisible if only successes are recorded.
func TestCollectPublishesFailuresToo(t *testing.T) {
	var published error
	calls := 0

	store := &recordingStore{}
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, errors.New("the sweep failed")
	})
	scheduler.Publish = func(_ context.Context, _ time.Time, err error) {
		calls++
		published = err
	}

	require.Error(t, scheduler.collect(context.Background()))
	assert.Equal(t, 1, calls)
	require.Error(t, published)
	assert.Contains(t, published.Error(), "the sweep failed")
}

// TestFireGivesUpWhenAnotherReplicaHoldsTheLease is not a failure: replicas collect one at a
// time, and a window that closes before this replica's turn simply means the next schedule
// takes it.
func TestFireGivesUpWhenAnotherReplicaHoldsTheLease(t *testing.T) {
	store := &recordingStore{}
	collected := false
	scheduler := newScheduler(t, store, busyLease{}, func(context.Context) (Report, error) {
		collected = true
		return Report{}, nil
	})

	start := time.Now()
	scheduler.fire(context.Background())

	assert.False(t, collected)
	assert.Less(t, time.Since(start), time.Second, "the window was not respected")
	quiesced, _, _ := store.state()
	assert.Zero(t, quiesced, "the store went read-only without the lease")
}

// TestRunSkipsWhileDisabled keeps a disabled collection from being a busy loop, and from
// collecting anyway.
func TestRunSkipsWhileDisabled(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	// Known, and says no. Distinct from an instruction that has not been read, which is a
	// different branch and must not be tested by accident here.
	scheduler.Plan = func() Plan { return Plan{Known: true, Enabled: false, Schedule: "* * * * *"} }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	scheduler.Run(ctx)

	assert.Zero(t, lease.held, "a disabled collection took the lease")
}

// TestRunWaitsBrieflyForAnInstructionItHasNotReadYet is the fix for a store that was never
// reclaimed for the first hour of its life.
//
// At startup the loop has not yet stored the desired state, so the plan is empty. An empty plan
// used to be indistinguishable from a disabled collection, and the disabled branch waits an hour
// before looking again — so a replica given `*/15 * * * *` collected nothing until the hour was up,
// and there was no log line anywhere to say why. Measured on a cluster: forty-two minutes after the
// pod started, with collection enabled and a fifteen-minute schedule, no collection had been
// scheduled at all and the reclaim alert was about to fire.
//
// What is asserted is the DURATION of the wait, which is why the sleeping is injectable: the bug was
// never that the scheduler waited, it was how long.
func TestRunWaitsBrieflyForAnInstructionItHasNotReadYet(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	// Never known, as it is for the first moments after startup.
	scheduler.Plan = func() Plan { return Plan{} }

	var waited time.Duration
	scheduler.Sleep = func(_ context.Context, d time.Duration) bool {
		waited = d
		// Stop after the first wait: what is being measured is its length.
		return false
	}

	scheduler.Run(context.Background())

	assert.Equal(t, unknownPlanRetry, waited,
		"an instruction that has not been read yet was not waited on for the short interval")
	assert.Less(t, waited, time.Hour,
		"an unread instruction was charged the disabled path's hour, so a fresh replica would collect nothing for its first hour")
	assert.Zero(t, lease.held, "a collection ran on an instruction that had not been read")
}

// TestRunStillLeavesADisabledCollectionAlone pins the other side of that distinction: turning the
// collection off must remain cheap, and must not become a poll every minute.
func TestRunStillLeavesADisabledCollectionAlone(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	scheduler.Plan = func() Plan { return Plan{Known: true, Enabled: false, Schedule: "* * * * *"} }

	var waited time.Duration
	scheduler.Sleep = func(_ context.Context, d time.Duration) bool {
		waited = d
		return false
	}

	scheduler.Run(context.Background())

	assert.Equal(t, time.Hour, waited, "a disabled collection is re-read on a slow timer, not polled")
	assert.Zero(t, lease.held)
}

// TestRunDoesNotGuessAtAnUnreadableSchedule: collecting at some other hour than the one an
// operator wrote would be worse than not collecting, because read-only in the working day is
// exactly what the schedule exists to avoid.
func TestRunDoesNotGuessAtAnUnreadableSchedule(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	scheduler.Plan = func() Plan { return Plan{Known: true, Enabled: true, Schedule: "not a cron expression"} }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	scheduler.Run(ctx)

	assert.Zero(t, lease.held)
	quiesced, _, _ := store.state()
	assert.Zero(t, quiesced)
}

// TestRunCollectsWhenTheScheduleFires walks the whole path once, on a schedule that is due
// immediately.
func TestRunCollectsWhenTheScheduleFires(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}

	fired := make(chan struct{})
	var once sync.Once
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		once.Do(func() { close(fired) })
		return Report{Considered: 1}, nil
	})
	// A minute-granularity cron would make this test wait a minute, so the clock is held one
	// second before the next firing. It is held there rather than advanced, which means the
	// schedule keeps firing — hence the wait for Run to return before anything is asserted
	// about the store, rather than reading it mid-collection.
	scheduler.Now = func() time.Time { return time.Now().Truncate(time.Minute).Add(59 * time.Second) }

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); scheduler.Run(ctx) }()

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("the schedule never fired")
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}

	assert.Positive(t, lease.held)
	// The store is not quietened by the firing itself: that belongs to the reclamation, and this
	// collection reclaimed nothing. What must hold either way is that the replica is left serving.
	_, _, readOnly := store.state()
	assert.False(t, readOnly, "the replica was left read-only")
}

// TestRunWaitsOutAFillRatherThanFailingIt is the priority between the two things that touch this
// store: filling it, and reclaiming it.
//
// Collecting makes the registry refuse writes for the duration, and a fill IS writes. Firing while
// one is in flight does not slow the fill down — it fails it, reference by reference, and the
// replica then reports a partial store it has to redo from the start. On a cluster whose fill takes
// longer than the interval an operator wrote, that is not an edge case but the steady state.
//
// The wait is short on purpose, and this test pins that too: charging a full schedule period per
// attempt would postpone collection indefinitely on exactly the cluster that needs it — the one
// that keeps filling.
func TestRunWaitsOutAFillRatherThanFailingIt(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}

	var collections atomic.Int32
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		collections.Add(1)
		return Report{}, nil
	})
	scheduler.Now = func() time.Time { return time.Now().Truncate(time.Minute).Add(59 * time.Second) }

	var filling atomic.Bool
	filling.Store(true)
	scheduler.Filling = filling.Load

	waits := make(chan time.Duration, 8)
	scheduler.Sleep = func(ctx context.Context, d time.Duration) bool {
		select {
		case waits <- d:
		default:
		}
		return ctx.Err() == nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); scheduler.Run(ctx) }()

	// While the fill is in flight the schedule keeps arriving and keeps standing down.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case waited := <-waits:
			if waited == fillingRetry {
				goto stoodDown
			}
		case <-deadline:
			cancel()
			t.Fatal("the collection never stood down for the fill")
		}
	}

stoodDown:
	assert.Zero(t, collections.Load(), "a fill must not be interrupted by housekeeping")
	quiesced, _, _ := store.state()
	assert.Zero(t, quiesced, "the store was never put read-only, which is what would have failed the fill")

	// And once the fill is done, the very next firing collects.
	filling.Store(false)

	collected := time.After(5 * time.Second)
	for collections.Load() == 0 {
		select {
		case <-collected:
			cancel()
			t.Fatal("the collection never resumed after the fill finished")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}
}

// TestRunWaitsOutAPushFromOutside is the writer the syncer cannot take turns with: an operator
// pushing a bundle through the publication endpoint.
//
// Not "the endpoint is open" — that may be true for the life of a cluster while pushes happen twice a
// year, and refusing to collect on it would mean never collecting. What holds the collection off is a
// push actually in flight, because reclaiming blobs underneath one deletes what it has uploaded and
// not yet referenced by a manifest.
func TestRunWaitsOutAPushFromOutside(t *testing.T) {
	store := &recordingStore{}

	var collections atomic.Int32
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		collections.Add(1)
		return Report{}, nil
	})
	scheduler.Now = func() time.Time { return time.Now().Truncate(time.Minute).Add(59 * time.Second) }

	var pushing atomic.Bool
	pushing.Store(true)
	scheduler.Pushing = pushing.Load

	waits := make(chan time.Duration, 8)
	scheduler.Sleep = func(ctx context.Context, d time.Duration) bool {
		select {
		case waits <- d:
		default:
		}
		return ctx.Err() == nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); scheduler.Run(ctx) }()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case waited := <-waits:
			if waited == fillingRetry {
				goto waited
			}
		case <-deadline:
			cancel()
			t.Fatal("the collection never stood down for a push in flight")
		}
	}

waited:
	assert.Zero(t, collections.Load(), "a push must not be swept from under")

	pushing.Store(false)
	collected := time.After(5 * time.Second)
	for collections.Load() == 0 {
		select {
		case <-collected:
			cancel()
			t.Fatal("the collection never resumed after the push finished")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}
}

// TestRunHoldsBackOnAnIncompleteStore is the rule stated by the operator: while a fill is owed, the
// collection does not run at all — the cache has to be filled first.
//
// Distinct from waiting out a pass in flight, and the distinction is the whole point. A pass runs for
// seconds and the loop then sleeps for thirty, so "is a fill happening right now" answers "no" most
// of the time on a store that is nowhere near complete — and in that window the collection fired,
// reclaimed against a set that was still going in, and made the registry refuse the writes the fill
// consists of.
func TestRunHoldsBackOnAnIncompleteStore(t *testing.T) {
	store := &recordingStore{}
	lease := &directLease{}

	var collections atomic.Int32
	scheduler := newScheduler(t, store, lease, func(context.Context) (Report, error) {
		collections.Add(1)
		return Report{}, nil
	})
	scheduler.Now = func() time.Time { return time.Now().Truncate(time.Minute).Add(59 * time.Second) }

	// Owed a fill, and no pass in flight — the window that used to be open.
	var pending atomic.Bool
	pending.Store(true)
	scheduler.Plan = func() Plan {
		return Plan{Known: true, Enabled: true, Schedule: "* * * * *", FillPending: pending.Load()}
	}
	scheduler.Filling = func() bool { return false }

	waits := make(chan time.Duration, 8)
	scheduler.Sleep = func(ctx context.Context, d time.Duration) bool {
		select {
		case waits <- d:
		default:
		}
		return ctx.Err() == nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); scheduler.Run(ctx) }()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case waited := <-waits:
			if waited == fillingRetry {
				goto held
			}
		case <-deadline:
			cancel()
			t.Fatal("the collection never stood down for an unfilled store")
		}
	}

held:
	assert.Zero(t, collections.Load(), "an incomplete store must not be collected")

	// Filled, and the next firing collects.
	pending.Store(false)

	collected := time.After(5 * time.Second)
	for collections.Load() == 0 {
		select {
		case <-collected:
			cancel()
			t.Fatal("the collection never resumed once the store was filled")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}
}
