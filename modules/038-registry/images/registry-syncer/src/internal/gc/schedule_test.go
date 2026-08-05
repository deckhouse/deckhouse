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

func newScheduler(t *testing.T, store Store, lease Lease, collect func(context.Context) (Report, error)) *Scheduler {
	t.Helper()

	return &Scheduler{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plan:    func() Plan { return Plan{Enabled: true, Schedule: "* * * * *"} },
		Lease:   lease,
		Store:   store,
		Collect: collect,
		Window:  100 * time.Millisecond,
	}
}

// TestCollectRestoresWritesAfterwards is the property that matters most here: a replica left
// read-only would refuse a `d8 mirror push` with nothing to explain why.
func TestCollectRestoresWritesAfterwards(t *testing.T) {
	store := &recordingStore{}
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		_, _, readOnly := store.state()
		assert.True(t, readOnly, "the store was collected while still accepting writes")
		return Report{Considered: 3}, nil
	})

	require.NoError(t, scheduler.collect(context.Background()))

	quiesced, restored, readOnly := store.state()
	assert.Equal(t, 1, quiesced)
	assert.Equal(t, 1, restored)
	assert.False(t, readOnly)
}

// TestCollectRestoresWritesAfterAFailure: the same property, on the path where it is easiest
// to get wrong.
func TestCollectRestoresWritesAfterAFailure(t *testing.T) {
	store := &recordingStore{}
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, errors.New("the registry refused the deletion")
	})

	err := scheduler.collect(context.Background())
	require.Error(t, err)

	_, restored, readOnly := store.state()
	assert.Equal(t, 1, restored, "a failed collection left the replica read-only")
	assert.False(t, readOnly)
}

// TestCollectRestoresWritesAfterAPanic covers the case a deferred restore exists for.
func TestCollectRestoresWritesAfterAPanic(t *testing.T) {
	store := &recordingStore{}
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		panic("the collector exploded")
	})

	assert.Panics(t, func() { _ = scheduler.collect(context.Background()) })

	_, restored, readOnly := store.state()
	assert.Equal(t, 1, restored, "a panicking collection left the replica read-only")
	assert.False(t, readOnly)
}

// TestCollectDoesNotTouchTheStoreWhenItCannotQuiesce: collecting a store that still accepts
// writes is the documented way to delete a blob somebody is uploading.
func TestCollectDoesNotTouchTheStoreWhenItCannotQuiesce(t *testing.T) {
	store := &recordingStore{quiesceErr: errors.New("the configuration is not writable")}
	collected := false
	scheduler := newScheduler(t, store, &directLease{}, func(context.Context) (Report, error) {
		collected = true
		return Report{}, nil
	})

	err := scheduler.collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
	assert.False(t, collected, "the store was collected while accepting writes")
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
	scheduler.Plan = func() Plan { return Plan{Enabled: false, Schedule: "* * * * *"} }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	scheduler.Run(ctx)

	assert.Zero(t, lease.held, "a disabled collection took the lease")
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
	scheduler.Plan = func() Plan { return Plan{Enabled: true, Schedule: "not a cron expression"} }

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
	quiesced, restored, readOnly := store.state()
	assert.Positive(t, quiesced)
	assert.Equal(t, quiesced, restored, "a collection did not hand writes back")
	assert.False(t, readOnly, "the replica was left read-only")
}
