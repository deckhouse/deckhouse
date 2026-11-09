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
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// directLease grants immediately, standing in for a lease nobody else holds.
type directLease struct{ held atomic.Int32 }

func (l *directLease) Hold(ctx context.Context, work func(context.Context) error) error {
	l.held.Add(1)
	return work(ctx)
}

// busyLease never grants, standing in for another replica collecting for the whole window.
type busyLease struct{}

func (busyLease) Hold(ctx context.Context, _ func(context.Context) error) error {
	<-ctx.Done()
	return ctx.Err()
}

// fakeOperations is one operation in memory, owed until a final progress is written.
type fakeOperations struct {
	mutex    sync.Mutex
	present  bool
	owed     bool
	canceled bool
	passed   bool
	deadline time.Time
	written  []Progress
}

func newFakeOperations() *fakeOperations {
	return &fakeOperations{present: true, owed: true}
}

func (f *fakeOperations) current() *Operation {
	if !f.present || !f.owed {
		return nil
	}
	return &Operation{Name: "gc-manual-1", Deadline: f.deadline, Canceled: f.canceled, PassedGate: f.passed}
}

func (f *fakeOperations) Next(context.Context) (*Operation, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.current(), nil
}

func (f *fakeOperations) Get(context.Context, string) (*Operation, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.current(), nil
}

func (f *fakeOperations) Report(_ context.Context, _ string, progress Progress) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if !f.present || !f.owed {
		return ErrNotOwed
	}
	f.written = append(f.written, progress)
	if progress.Phase.Finished() {
		f.owed = false
	}
	return nil
}

func (f *fakeOperations) set(change func(*fakeOperations)) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	change(f)
}

func (f *fakeOperations) phases() []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	var phases []string
	for _, progress := range f.written {
		phases = append(phases, string(progress.Phase)+"/"+progress.Reason)
	}
	return phases
}

func (f *fakeOperations) last() Progress {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.written[len(f.written)-1]
}

func (f *fakeOperations) settled() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return !f.owed
}

// newWorker is a worker over operations whose every wait takes a millisecond, so a loop that keeps
// waiting does not spin, and whose operations are looked at often enough for a test.
func newWorker(
	t *testing.T, operations Operations, lease Lease, collect func(context.Context) (Report, error),
) *Worker {
	t.Helper()

	previous := operationPoll
	operationPoll = 5 * time.Millisecond
	t.Cleanup(func() { operationPoll = previous })

	return &Worker{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plan:       func() Plan { return Plan{Known: true} },
		Operations: operations,
		Lease:      lease,
		Collect:    collect,
		Window:     time.Hour,
		Sleep:      func(ctx context.Context, _ time.Duration) bool { return sleep(ctx, time.Millisecond) },
	}
}

// start runs the worker until the returned stop is called.
func start(t *testing.T, worker *Worker) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); worker.Run(ctx) }()

	return func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not stop with its context")
		}
	}
}

// runUntil runs the worker until done reports true.
func runUntil(t *testing.T, worker *Worker, done func() bool) {
	t.Helper()

	stop := start(t, worker)
	defer stop()
	require.Eventually(t, done, 5*time.Second, time.Millisecond)
}

func TestAnOperationIsCollected(t *testing.T) {
	operations := newFakeOperations()
	lease := &directLease{}
	var collections atomic.Int32
	reclaimed := int64(4 << 30)

	worker := newWorker(t, operations, lease, func(context.Context) (Report, error) {
		collections.Add(1)
		return Report{
			Deleted:         []string{"a", "b", "c"},
			RemovedVersions: []string{"v1.71.2"},
			ReclaimedBytes:  &reclaimed,
		}, nil
	})

	runUntil(t, worker, operations.settled)

	assert.EqualValues(t, 1, collections.Load())
	assert.EqualValues(t, 1, lease.held.Load(), "on the lease, one replica at a time")
	assert.Equal(t, []string{"Waiting/Turn", "Running/", "Succeeded/"}, operations.phases())

	final := operations.last()
	require.NotNil(t, final.StartedAt)
	require.NotNil(t, final.FinishedAt)
	require.NotNil(t, final.Report)
	assert.Len(t, final.Report.Deleted, 3)
	assert.Equal(t, []string{"v1.71.2"}, final.Report.RemovedVersions)
	assert.Equal(t, &reclaimed, final.Report.ReclaimedBytes)
}

// TestNothingIsCollectedUnasked: when to collect is the controller's decision, and a replica with no
// operation never collects on its own.
func TestNothingIsCollectedUnasked(t *testing.T) {
	operations := newFakeOperations()
	operations.present = false

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		t.Error("collected without an operation")
		return Report{}, nil
	})

	stop := start(t, worker)
	time.Sleep(30 * time.Millisecond)
	stop()

	assert.Empty(t, operations.phases())
}

// TestAnOperationWaitsOutAFill: a store still being filled is not one whose contents can be compared
// with anything.
func TestAnOperationWaitsOutAFill(t *testing.T) {
	operations := newFakeOperations()
	var collections atomic.Int32

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		collections.Add(1)
		return Report{}, nil
	})
	var fillPending atomic.Bool
	fillPending.Store(true)
	worker.Plan = func() Plan { return Plan{Known: true, FillPending: fillPending.Load()} }

	stop := start(t, worker)
	defer stop()

	require.Eventually(t, func() bool {
		phases := operations.phases()
		return len(phases) > 0 && phases[0] == "Waiting/Filling"
	}, 5*time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, collections.Load(), "a fill must not be collected under")
	assert.Equal(t, []string{"Waiting/Filling"}, operations.phases(), "the same wait is written once")

	fillPending.Store(false)
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)

	assert.EqualValues(t, 1, collections.Load())
	assert.Equal(t, []string{"Waiting/Filling", "Waiting/Turn", "Running/", "Succeeded/"}, operations.phases())
}

// TestAnOperationWaitsOutAPushFromOutside: reclaiming blobs underneath a push deletes what it has
// uploaded and not yet referenced by a manifest.
func TestAnOperationWaitsOutAPushFromOutside(t *testing.T) {
	operations := newFakeOperations()
	var pushing atomic.Bool
	pushing.Store(true)

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	worker.Pushing = pushing.Load
	worker.Filling = func() bool { return false }

	stop := start(t, worker)
	defer stop()

	require.Eventually(t, func() bool {
		phases := operations.phases()
		return len(phases) > 0 && phases[0] == "Waiting/Pushing"
	}, 5*time.Second, time.Millisecond)

	pushing.Store(false)
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)
	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, operations.last().Phase)
}

// TestAnOperationGivesUpOnAStoreStillWrittenToAtTheDeadline: a store that is written to for longer
// than the deadline is not one to collect now, and whoever asked has to hear that.
func TestAnOperationGivesUpOnAStoreStillWrittenToAtTheDeadline(t *testing.T) {
	operations := newFakeOperations()
	operations.deadline = time.Now().Add(-time.Second)

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		t.Error("collected under a push")
		return Report{}, nil
	})
	worker.Pushing = func() bool { return true }

	runUntil(t, worker, operations.settled)

	final := operations.last()
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, final.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationDeadlineExceeded, final.Reason)
	assert.Contains(t, final.Message, registryv1alpha1.ReasonOperationPushing)
}

// TestAnInstructionNotReadYetIsWaitedFor: a replica that has not looked at its store yet knows
// nothing about whether it is being filled.
func TestAnInstructionNotReadYetIsWaitedFor(t *testing.T) {
	operations := newFakeOperations()
	var known atomic.Bool

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	worker.Plan = func() Plan { return Plan{Known: known.Load()} }

	stop := start(t, worker)
	defer stop()

	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, operations.phases())

	known.Store(true)
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)
}

func TestACanceledOperationIsNotCollected(t *testing.T) {
	operations := newFakeOperations()
	operations.canceled = true

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		t.Error("collected a canceled operation")
		return Report{}, nil
	})

	runUntil(t, worker, operations.settled)

	assert.Equal(t, []string{"Canceled/Canceled"}, operations.phases())
}

// TestCancelReachesARunningCollection: the label stops a replica that is already collecting, and the
// stop is not recorded as the replica's last collection error.
func TestCancelReachesARunningCollection(t *testing.T) {
	operations := newFakeOperations()
	running := make(chan struct{})
	var published atomic.Int32

	worker := newWorker(t, operations, &directLease{}, func(ctx context.Context) (Report, error) {
		close(running)
		<-ctx.Done()
		return Report{Deleted: []string{"a"}}, ctx.Err()
	})
	worker.Publish = func(context.Context, time.Time, error) { published.Add(1) }

	stop := start(t, worker)
	defer stop()

	select {
	case <-running:
	case <-time.After(5 * time.Second):
		t.Fatal("the collection never started")
	}
	operations.set(func(f *fakeOperations) { f.canceled = true })
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)

	final := operations.last()
	assert.Equal(t, registryv1alpha1.ReplicaOperationCanceled, final.Phase)
	require.NotNil(t, final.StartedAt, "it did start, and what it deleted is reported")
	require.NotNil(t, final.Report)
	assert.Len(t, final.Report.Deleted, 1)
	assert.Zero(t, published.Load(), "a stopped collection is not a failed one")
}

// TestAFailedCollectionFailsTheReplica carries the error to whoever reads the operation, and to the
// storage status as the replica's last collection error.
func TestAFailedCollectionFailsTheReplica(t *testing.T) {
	operations := newFakeOperations()
	var published error

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, errors.New("the sweep failed")
	})
	var publishing sync.Mutex
	worker.Publish = func(_ context.Context, _ time.Time, err error) {
		publishing.Lock()
		defer publishing.Unlock()
		published = err
	}

	runUntil(t, worker, operations.settled)

	final := operations.last()
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, final.Phase)
	assert.Equal(t, registryv1alpha1.ReasonOperationFailed, final.Reason)
	assert.Equal(t, "the sweep failed", final.Message)

	publishing.Lock()
	defer publishing.Unlock()
	require.Error(t, published)
	assert.Contains(t, published.Error(), "the sweep failed")
}

// TestATurnThatNeverComesIsWaitedForAgain: another replica holding the lease for a whole window is a
// queue, not a failure, and the operation stays owed.
func TestATurnThatNeverComesIsWaitedForAgain(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, busyLease{}, func(context.Context) (Report, error) {
		t.Error("collected without the lease")
		return Report{}, nil
	})
	worker.Window = 10 * time.Millisecond

	stop := start(t, worker)
	time.Sleep(100 * time.Millisecond)
	stop()

	assert.False(t, operations.settled())
	assert.Equal(t, []string{"Waiting/Turn"}, operations.phases())
}

// delayedLease grants after a delay, standing in for another replica collecting before this one.
type delayedLease struct{ delay time.Duration }

func (l delayedLease) Hold(ctx context.Context, work func(context.Context) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(l.delay):
	}
	return work(ctx)
}

// TestAnOperationDeletedWhileWaitingForTheTurnIsNotCollected: deleting an operation cancels it before
// a replica starts, as the CRD says — including a replica already waiting for its turn.
func TestAnOperationDeletedWhileWaitingForTheTurnIsNotCollected(t *testing.T) {
	operations := newFakeOperations()

	lease := &releasingLease{released: make(chan struct{})}
	worker := newWorker(t, operations, lease, func(context.Context) (Report, error) {
		t.Error("collected an operation that was deleted")
		return Report{}, nil
	})

	stop := start(t, worker)
	defer stop()

	require.Eventually(t, func() bool {
		phases := operations.phases()
		return len(phases) > 0 && phases[0] == "Waiting/Turn"
	}, 5*time.Second, time.Millisecond)
	operations.set(func(f *fakeOperations) { f.present = false })

	// Given up on, not waited out: the window is an hour.
	select {
	case <-lease.released:
	case <-time.After(5 * time.Second):
		t.Fatal("the replica kept waiting for the turn of an operation that is gone")
	}
	assert.Equal(t, []string{"Waiting/Turn"}, operations.phases(), "nothing settled for an operation that is gone")
}

// releasingLease never grants, and says when the replica stopped waiting for it.
type releasingLease struct {
	released chan struct{}
	once     sync.Once
}

func (l *releasingLease) Hold(ctx context.Context, _ func(context.Context) error) error {
	<-ctx.Done()
	l.once.Do(func() { close(l.released) })
	return ctx.Err()
}

// TestTheTurnAndTheCollectionAreBoundedApart: the last of three replicas waited most of the window for
// its turn and had its collection killed by what was left of it.
func TestTheTurnAndTheCollectionAreBoundedApart(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, delayedLease{delay: 150 * time.Millisecond}, func(ctx context.Context) (Report, error) {
		select {
		case <-ctx.Done():
			return Report{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return Report{}, nil
		}
	})
	worker.Window = 200 * time.Millisecond

	runUntil(t, worker, operations.settled)

	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, operations.last().Phase,
		"150 ms waiting and 100 ms collecting both fit a 200 ms window each")
}

// TestTheDeadlineDoesNotApplyOnceTheGateWasPassed: a replica that waited for its turn, or restarted
// after it had, is not failed for a fill that started after it got past the store.
func TestTheDeadlineDoesNotApplyOnceTheGateWasPassed(t *testing.T) {
	operations := newFakeOperations()
	operations.deadline = time.Now().Add(-time.Hour)
	operations.passed = true

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		return Report{}, nil
	})
	var filling atomic.Bool
	filling.Store(true)
	worker.Plan = func() Plan { return Plan{Known: true, FillPending: filling.Load()} }

	stop := start(t, worker)
	defer stop()

	// Said as the turn it still waits for, so that a syncer restarted now reads from the entry that
	// the gate was passed — Waiting/Filling there would not say it.
	require.Eventually(t, func() bool {
		phases := operations.phases()
		return len(phases) > 0 && phases[0] == "Waiting/Turn"
	}, 5*time.Second, time.Millisecond)
	assert.Contains(t, operations.last().Message, registryv1alpha1.ReasonOperationFilling)
	time.Sleep(30 * time.Millisecond)
	assert.False(t, operations.settled(), "past the deadline, and still not failed for it")

	filling.Store(false)
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)
	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, operations.last().Phase)
}

// TestACollectionCutByTheWindowIsPublished: the context it ran on is over, and publishing on it left
// the storage reporting a collection the replica had not finished.
func TestACollectionCutByTheWindowIsPublished(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, &directLease{}, func(ctx context.Context) (Report, error) {
		<-ctx.Done()
		return Report{}, ctx.Err()
	})
	worker.Window = 20 * time.Millisecond
	var published atomic.Bool
	var publishedOnLiveContext atomic.Bool
	worker.Publish = func(ctx context.Context, _ time.Time, err error) {
		published.Store(true)
		publishedOnLiveContext.Store(ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded))
	}

	runUntil(t, worker, operations.settled)

	assert.True(t, published.Load())
	assert.True(t, publishedOnLiveContext.Load(), "published, on a context that can still write")
	assert.Equal(t, registryv1alpha1.ReplicaOperationFailed, operations.last().Phase)
}

// TestACollectionStoppedByShutdownIsNotPublished: a rollout stops the pod mid-collection, and recording
// that as a failed collection made every rollout read as a broken store.
func TestACollectionStoppedByShutdownIsNotPublished(t *testing.T) {
	operations := newFakeOperations()

	running := make(chan struct{})
	worker := newWorker(t, operations, &directLease{}, func(ctx context.Context) (Report, error) {
		close(running)
		<-ctx.Done()
		return Report{}, ctx.Err()
	})
	var published atomic.Bool
	worker.Publish = func(context.Context, time.Time, error) { published.Store(true) }

	stop := start(t, worker)
	<-running
	stop()

	assert.False(t, published.Load())
}

// TestAnOperationGoneWhileCollectingDoesNotStopIt: deleting an operation stops a replica before it
// starts, and only then — a collection under way is not cut between a deletion and its sweep.
func TestAnOperationGoneWhileCollectingDoesNotStopIt(t *testing.T) {
	operations := newFakeOperations()

	var cut, finished atomic.Bool
	worker := newWorker(t, operations, &directLease{}, func(ctx context.Context) (Report, error) {
		operations.set(func(f *fakeOperations) { f.present = false })
		select {
		case <-ctx.Done():
			cut.Store(true)
			return Report{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			finished.Store(true)
			return Report{}, nil
		}
	})

	runUntil(t, worker, finished.Load)

	assert.False(t, cut.Load())
	assert.Equal(t, []string{"Waiting/Turn", "Running/"}, operations.phases(),
		"the outcome has nowhere to go, and nothing else was written")
}

// losingLease runs the work and loses the lease under it, the way ClusterLease reports that.
type losingLease struct{}

func (losingLease) Hold(ctx context.Context, work func(context.Context) error) error {
	held, lose := context.WithCancel(ctx)
	lose()
	if err := work(held); err != nil {
		return fmt.Errorf("the collection lease was lost while collecting: %w", err)
	}
	return nil
}

// TestALostLeaseIsTheCauseThatIsPublished: the storage said "context canceled" where the operation
// said the lease was lost.
func TestALostLeaseIsTheCauseThatIsPublished(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, losingLease{}, func(ctx context.Context) (Report, error) {
		<-ctx.Done()
		return Report{}, ctx.Err()
	})
	var publishedErr atomic.Value
	worker.Publish = func(_ context.Context, _ time.Time, err error) { publishedErr.Store(err.Error()) }

	runUntil(t, worker, operations.settled)

	assert.Contains(t, publishedErr.Load(), "lease was lost")
	assert.Contains(t, operations.last().Message, "lease was lost")
}

// TestAPushStartedDuringTheWaitGivesTheTurnBack: the store is looked at again once the turn comes. A
// push that started while the replica waited has its layers on disk and no manifest yet, and a sweep
// would remove them.
func TestAPushStartedDuringTheWaitGivesTheTurnBack(t *testing.T) {
	operations := newFakeOperations()
	var pushing atomic.Bool
	var collected atomic.Int32

	lease := &directLease{}
	worker := newWorker(t, operations, leaseFunc(func(ctx context.Context, work func(context.Context) error) error {
		// The push begins while the replica waits for its turn, after the check before the wait.
		if lease.held.Add(1) == 1 {
			pushing.Store(true)
		}
		return work(ctx)
	}), func(context.Context) (Report, error) {
		require.False(t, pushing.Load(), "collected while a push was under way")
		collected.Add(1)
		return Report{}, nil
	})
	worker.Pushing = pushing.Load

	stop := start(t, worker)
	defer stop()

	require.Eventually(t, func() bool {
		for _, phase := range operations.phases() {
			if strings.HasPrefix(phase, "Waiting/Turn") && strings.Contains(operations.last().Message, "Pushing") {
				return true
			}
		}
		return false
	}, 5*time.Second, time.Millisecond, "waits for the push, as one past the gate")
	assert.Zero(t, collected.Load())
	assert.NotContains(t, operations.phases(), "Running/", "Running is said only by a replica that collects")

	pushing.Store(false)
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond)
	assert.EqualValues(t, 1, collected.Load())
	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, operations.last().Phase)
}

// TestAStoreItsOwnWritersHoldIsNotWaitedOnWithTheTurn: a fill holds the store for its whole pass. A
// replica that waited for it with the lease in hand would hold the others out, and could not be
// canceled meanwhile.
func TestAStoreItsOwnWritersHoldIsNotWaitedOnWithTheTurn(t *testing.T) {
	operations := newFakeOperations()
	var held atomic.Bool
	held.Store(true)
	var collected atomic.Int32

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		collected.Add(1)
		return Report{}, nil
	})
	worker.Pause = func() (func(), bool) {
		if held.Load() {
			return nil, false
		}
		return func() {}, true
	}

	stop := start(t, worker)
	defer stop()

	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, collected.Load())
	assert.NotContains(t, operations.phases(), "Running/")

	operations.set(func(f *fakeOperations) { f.canceled = true })
	require.Eventually(t, operations.settled, 5*time.Second, time.Millisecond, "a cancel reaches it")
	assert.Equal(t, registryv1alpha1.ReplicaOperationCanceled, operations.last().Phase)
	assert.Zero(t, collected.Load())
}

// TestAnEntrySettledDuringTheWaitIsNotCollected: the controller settles an entry it gives up on, or
// one of a canceled operation. A replica whose turn comes after that does not collect on it.
func TestAnEntrySettledDuringTheWaitIsNotCollected(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, leaseFunc(func(ctx context.Context, work func(context.Context) error) error {
		operations.set(func(f *fakeOperations) { f.owed = false })
		return work(ctx)
	}), func(context.Context) (Report, error) {
		t.Error("collected an entry that was settled")
		return Report{}, nil
	})

	stop := start(t, worker)
	defer stop()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []string{"Waiting/Turn"}, operations.phases())
}

// TestACancelAfterTheCollectionFinishedLeavesItSucceeded: the cancellation came too late to stop
// anything, so the outcome is what happened, and it is published.
func TestACancelAfterTheCollectionFinishedLeavesItSucceeded(t *testing.T) {
	operations := newFakeOperations()

	worker := newWorker(t, operations, &directLease{}, func(context.Context) (Report, error) {
		operations.set(func(f *fakeOperations) { f.canceled = true })
		// Long enough for the watcher to see the label before the collection returns.
		time.Sleep(50 * time.Millisecond)
		return Report{RemovedManifests: 3, RemovedBlobs: 9}, nil
	})
	var published atomic.Bool
	worker.Publish = func(context.Context, time.Time, error) { published.Store(true) }

	runUntil(t, worker, operations.settled)

	assert.Equal(t, registryv1alpha1.ReplicaOperationSucceeded, operations.last().Phase)
	assert.True(t, published.Load())
}

// leaseFunc is a Lease written inline.
type leaseFunc func(ctx context.Context, work func(context.Context) error) error

func (f leaseFunc) Hold(ctx context.Context, work func(context.Context) error) error {
	return f(ctx, work)
}
