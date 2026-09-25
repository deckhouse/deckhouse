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

package rejoin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	interval    = time.Second
	maxInterval = 4 * time.Second

	pollStep        = 10 * time.Millisecond
	waitLimit       = time.Minute
	attemptLimit    = 10000
	quorumReadLimit = 100000
)

const notMemberVerdict = "this node is not a member of its NodeGroup, rejoin does not join until that changes"

var (
	errNotMember = errors.New("this node is not a member of its NodeGroup any more")
	errTransport = errors.New("no seed accepted the connection")

	testParams = Params{Interval: interval, MaxInterval: maxInterval}
)

type harness struct {
	t      *testing.T
	loop   *Loop
	ctx    context.Context
	cancel context.CancelFunc

	changed chan struct{}

	mu               sync.Mutex
	quorum           bool
	own              bool
	sleeps           []time.Duration
	onAttempt        func(n int64)
	attemptErr       error
	attemptResult    func(ctx context.Context, n int64) error
	onQuorumRead     func(n int64)
	onEpisodeStarted func()
	onOwnRead        func(ctx context.Context, n int64) bool

	attempts    atomic.Int64
	quorumReads atomic.Int64
	ownReads    atomic.Int64
	episodes    atomic.Int64
}

func newHarness(t *testing.T, params Params) *harness {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	h := &harness{t: t, ctx: ctx, cancel: cancel, changed: make(chan struct{}, 1)}

	h.loop = New(params, Deps{
		Attempt: func(ctx context.Context) error {
			n := h.attempts.Add(1)

			if n > attemptLimit {
				h.t.Errorf("the rejoin loop made more than %d attempts, stopping it", attemptLimit)
				h.cancel()

				return context.Canceled
			}

			h.mu.Lock()
			hook, result, err := h.onAttempt, h.attemptResult, h.attemptErr
			h.mu.Unlock()

			if hook != nil {
				hook(n)
			}

			if result != nil {
				return result(ctx, n)
			}

			return err
		},
		EpisodeStarted: func() {
			h.episodes.Add(1)

			h.mu.Lock()
			hook := h.onEpisodeStarted
			h.mu.Unlock()

			if hook != nil {
				hook()
			}
		},
		HasQuorum: func() bool {
			n := h.quorumReads.Add(1)
			if n > quorumReadLimit {
				h.t.Errorf("the rejoin loop read the quorum more than %d times, stopping it", quorumReadLimit)
				h.cancel()
			}

			h.mu.Lock()
			hook := h.onQuorumRead
			h.mu.Unlock()

			if hook != nil {
				hook(n)
			}

			h.mu.Lock()
			defer h.mu.Unlock()

			return h.quorum
		},
		OwnFailedRecord: func(ctx context.Context) bool {
			n := h.ownReads.Add(1)

			h.mu.Lock()
			hook, own := h.onOwnRead, h.own
			h.mu.Unlock()

			if hook != nil {
				return hook(ctx, n)
			}

			return own
		},
		NotMember: func(err error) bool { return errors.Is(err, errNotMember) },
		Changed:   h.changed,
		Sleep: func(ctx context.Context, d time.Duration) bool {
			h.mu.Lock()
			h.sleeps = append(h.sleeps, d)
			h.mu.Unlock()

			select {
			case <-ctx.Done():
				return false
			case <-time.After(d):
				return true
			}
		},
	}, log.NewNop())

	return h
}

func (h *harness) setQuorum(v bool) { h.mu.Lock(); h.quorum = v; h.mu.Unlock() }

func (h *harness) setOwn(v bool) { h.mu.Lock(); h.own = v; h.mu.Unlock() }

func (h *harness) quorumAfter(n int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.onAttempt = func(attempt int64) {
		if attempt == n {
			h.setQuorum(true)
		}
	}
}

func (h *harness) failAttempts(err error) {
	h.mu.Lock()
	h.attemptErr = err
	h.mu.Unlock()
}

func (h *harness) setAttemptResult(result func(ctx context.Context, n int64) error) {
	h.mu.Lock()
	h.attemptResult = result
	h.mu.Unlock()
}

func (h *harness) setOnAttempt(hook func(n int64)) {
	h.mu.Lock()
	h.onAttempt = hook
	h.mu.Unlock()
}

func (h *harness) setOnQuorumRead(hook func(n int64)) {
	h.mu.Lock()
	h.onQuorumRead = hook
	h.mu.Unlock()
}

func (h *harness) setOnEpisodeStarted(hook func()) {
	h.mu.Lock()
	h.onEpisodeStarted = hook
	h.mu.Unlock()
}

func (h *harness) setOnOwnRead(hook func(ctx context.Context, n int64) bool) {
	h.mu.Lock()
	h.onOwnRead = hook
	h.mu.Unlock()
}

const (
	episodeEvent = "episode"
	attemptEvent = "attempt"
)

type eventJournal struct {
	mu     sync.Mutex
	events []string
}

func (j *eventJournal) add(event string) {
	j.mu.Lock()
	j.events = append(j.events, event)
	j.mu.Unlock()
}

func (j *eventJournal) snapshot() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	return slices.Clone(j.events)
}

type attemptClock struct {
	mu    sync.Mutex
	times []time.Time
}

func (c *attemptClock) record() {
	c.mu.Lock()
	c.times = append(c.times, time.Now())
	c.mu.Unlock()
}

func (c *attemptClock) snapshot() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]time.Time(nil), c.times...)
}

func (h *harness) recordedSleeps() []time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]time.Duration(nil), h.sleeps...)
}

func (h *harness) run() func() {
	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = h.loop.Run(h.ctx)
	}()

	return func() {
		h.cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			h.t.Fatal("rejoin loop did not stop after cancel")
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(waitLimit)

	for {
		synctest.Wait()

		if cond() {
			return
		}

		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}

		time.Sleep(pollStep)
	}
}

func idleFor(d time.Duration) {
	time.Sleep(d)
	synctest.Wait()
}

func within(t *testing.T, got, nominal time.Duration) {
	t.Helper()

	lo, hi := nominal*8/10, nominal*12/10
	if got < lo || got > hi {
		t.Errorf("delay %s is outside [%s, %s]", got, lo, hi)
	}
}

func TestRejoinRepeatsUntilQuorumIsBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, testParams)
		h.quorumAfter(3)

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() == 3 })
		idleFor(5 * maxInterval)

		if got := h.attempts.Load(); got != 3 {
			t.Errorf("attempts = %d after quorum returned, want 3", got)
		}

		sleeps := h.recordedSleeps()
		if len(sleeps) != 2 {
			t.Fatalf("sleeps = %v, want one after each attempt that left quorum missing", sleeps)
		}

		within(t, sleeps[0], interval)
		within(t, sleeps[1], 2*interval)
	})
}

func TestBackoffIsCappedAndResetsAfterQuorum(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, testParams)
		h.quorumAfter(5)

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() == 5 })
		idleFor(5 * maxInterval)

		sleeps := h.recordedSleeps()
		if len(sleeps) != 4 {
			t.Fatalf("sleeps = %v, want 4", sleeps)
		}

		within(t, sleeps[2], maxInterval)
		within(t, sleeps[3], maxInterval)

		h.quorumAfter(7)
		h.setQuorum(false)
		h.changed <- struct{}{}

		synctest.Wait()

		if got := h.attempts.Load(); got != 6 {
			t.Fatalf("attempts = %d right after the Changed signal, want 6", got)
		}

		waitFor(t, func() bool { return h.attempts.Load() == 7 })

		sleeps = h.recordedSleeps()
		if len(sleeps) != 5 {
			t.Fatalf("sleeps = %v, want 5", sleeps)
		}

		within(t, sleeps[4], interval)
	})
}

func TestOwnFailedRecordStartsRejoinWhileQuorumHolds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, testParams)
		h.setQuorum(true)
		h.setOwn(true)

		clock := &attemptClock{}
		h.setOnAttempt(func(int64) { clock.record() })

		start := time.Now()

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() >= 5 })

		if first := clock.snapshot()[0].Sub(start); first >= idleTick {
			t.Errorf("the first attempt came %s after the start, want less than %s", first, idleTick)
		}

		if got := h.episodes.Load(); got != 1 {
			t.Errorf("episodes = %d while the failed record stays, want 1", got)
		}
	})
}

func TestOwnFailedRecordAppearingWithoutAGossipChangeIsNoticedWithinATick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			recordAppearsAfter   = 5 * time.Second
			recordReappearsAfter = 12*time.Second + idleTick/2
		)

		h := newHarness(t, testParams)
		h.setQuorum(true)

		clock := &attemptClock{}
		h.setOnAttempt(func(int64) { clock.record() })

		start := time.Now()

		stop := h.run()
		defer stop()

		idleFor(recordAppearsAfter)

		if got := h.attempts.Load(); got != 0 {
			t.Fatalf("attempts = %d while quorum holds and no peer records this node as failed, want 0", got)
		}

		h.setOwn(true)

		waitFor(t, func() bool { return h.attempts.Load() >= 1 })

		if first := clock.snapshot()[0].Sub(start); first > recordAppearsAfter+idleTick {
			t.Errorf("the first attempt came %s after the start, want at most %s", first, recordAppearsAfter+idleTick)
		}

		h.setOwn(false)
		idleFor(time.Until(start.Add(recordReappearsAfter)))

		if got := h.attempts.Load(); got != 1 {
			t.Fatalf("attempts = %d once the failed record is gone, want 1", got)
		}

		h.setOwn(true)

		waitFor(t, func() bool { return h.attempts.Load() >= 2 })

		if gap := clock.snapshot()[1].Sub(start) - recordReappearsAfter; gap > idleTick {
			t.Errorf("the attempt came %s after the failed record reappeared, want at most %s", gap, idleTick)
		}
	})
}

func TestEpisodeSpansBothTriggers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, testParams)

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return len(h.recordedSleeps()) == 2 })

		h.setQuorum(true)
		h.setOwn(true)

		waitFor(t, func() bool { return len(h.recordedSleeps()) == 3 })

		// The episode spans the handover from the lost quorum to the own failed
		// record: it is still the first one, not a second one opened by the record.
		if got := h.episodes.Load(); got != 1 {
			t.Errorf("episodes = %d after the handover, want 1", got)
		}

		h.setOwn(false)

		idleFor(5 * maxInterval)

		if got := h.attempts.Load(); got != 3 {
			t.Errorf("attempts = %d once both triggers are off, want 3", got)
		}

		h.setQuorum(false)

		waitFor(t, func() bool { return len(h.recordedSleeps()) == 4 })

		sleeps := h.recordedSleeps()
		within(t, sleeps[0], interval)
		within(t, sleeps[1], 2*interval)
		within(t, sleeps[2], 4*interval)
		within(t, sleeps[3], interval)

		if got := h.episodes.Load(); got != 2 {
			t.Errorf("episodes = %d after quorum was lost again, want 2", got)
		}
	})
}

func TestAttemptThatClearsBothTriggersEndsTheEpisodeBeforeTheNextOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, testParams)

		clock := &attemptClock{}

		var readAfterFirstAttempt atomic.Int64

		h.setOnAttempt(func(n int64) {
			clock.record()

			if n == 1 {
				readAfterFirstAttempt.Store(h.quorumReads.Load() + 1)
			}
		})
		h.setOnQuorumRead(func(n int64) {
			switch read := readAfterFirstAttempt.Load(); {
			case read == 0:
			case n == read:
				h.setQuorum(true)
			case n == read+1:
				h.setQuorum(false)
			}
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() >= 2 })

		times := clock.snapshot()
		if gap := times[1].Sub(times[0]); gap < idleTick {
			t.Errorf("the second attempt came %s after the first, want at least %s", gap, idleTick)
		}

		sleeps := h.recordedSleeps()
		if len(sleeps) != 1 {
			t.Fatalf("sleeps = %v, want exactly one, after the first attempt of the second episode", sleeps)
		}

		within(t, sleeps[0], interval)

		if got := h.episodes.Load(); got != 2 {
			t.Errorf("episodes = %d, want 2", got)
		}
	})
}

func TestSuccessfulAttemptWhileTheRecordRemainsStillBacksOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			attemptGuard = 100
			observed     = time.Minute
			maxAttempts  = 12
		)

		params := Params{Interval: time.Second, MaxInterval: 10 * time.Second}

		h := newHarness(t, params)
		h.setQuorum(true)
		h.setOwn(true)
		h.setAttemptResult(func(ctx context.Context, n int64) error {
			if n > attemptGuard {
				t.Errorf("the rejoin loop made more than %d successful attempts, stopping it", attemptGuard)
				h.cancel()

				return ctx.Err()
			}

			return nil
		})

		stop := h.run()
		defer stop()

		idleFor(observed)

		attempts := h.attempts.Load()
		if attempts > maxAttempts {
			t.Errorf("attempts = %d in %s of successful attempts while the failed record remains, want at most %d", attempts, observed, maxAttempts)
		}

		sleeps := h.recordedSleeps()
		if int64(len(sleeps)) != attempts {
			t.Fatalf("sleeps = %v after %d attempts, want one sleep after every attempt", sleeps, attempts)
		}

		for i, got := range sleeps {
			within(t, got, min(params.Interval<<i, params.MaxInterval))
		}

		if len(sleeps) == 0 || sleeps[len(sleeps)-1] < params.MaxInterval*8/10 {
			t.Errorf("sleeps = %v, want them to grow to %s", sleeps, params.MaxInterval)
		}
	})
}

func TestEveryAttemptErrorDoublesTheDelay(t *testing.T) {
	params := Params{Interval: interval, MaxInterval: 6 * interval}

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "dial error",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
		},
		{
			name: "attempt deadline while the loop context is alive",
			err:  context.DeadlineExceeded,
		},
		{
			name: "wrapped not a member",
			err:  fmt.Errorf("rejoin join candidates: %w", errNotMember),
		},
		{
			name: "no usable candidate address",
			err:  errors.New("none of the 3 join candidates has a usable address"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, params)
				h.failAttempts(tc.err)

				stop := h.run()
				defer stop()

				waitFor(t, func() bool { return h.attempts.Load() >= 1 })

				if len(h.recordedSleeps()) == 0 {
					t.Fatalf("no sleep after the attempt failed with %v: the loop stopped as if on shutdown", tc.err)
				}

				waitFor(t, func() bool { return len(h.recordedSleeps()) >= 5 })

				sleeps := h.recordedSleeps()
				within(t, sleeps[0], params.Interval)
				within(t, sleeps[1], 2*params.Interval)
				within(t, sleeps[2], 4*params.Interval)
				within(t, sleeps[3], params.MaxInterval)
				within(t, sleeps[4], params.MaxInterval)
			})
		})
	}
}

func TestNilNotMemberIsNeverAVerdict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var attempts atomic.Int64

		loop := New(testParams, Deps{
			Attempt: func(ctx context.Context) error {
				if attempts.Add(1) > attemptLimit {
					t.Errorf("the rejoin loop made more than %d attempts, stopping it", attemptLimit)
					cancel()

					return ctx.Err()
				}

				return fmt.Errorf("rejoin join candidates: %w", errNotMember)
			},
			HasQuorum: func() bool { return false },
			Changed:   make(chan struct{}),
		}, log.NewNop())

		done := make(chan struct{})

		go func() {
			defer close(done)

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Run panicked with NotMember unset: %v", r)
				}
			}()

			_ = loop.Run(ctx)
		}()

		waitFor(t, func() bool {
			select {
			case <-done:
				return true
			default:
				return attempts.Load() >= 2
			}
		})

		cancel()
		<-done

		if got := attempts.Load(); got < 2 {
			t.Errorf("attempts = %d with NotMember unset, want the loop to keep retrying", got)
		}
	})
}

func TestEveryEpisodeStartsOneJoinLogEpisode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const attemptsPerEpisode = 3

		h := newHarness(t, testParams)
		journal := &eventJournal{}

		h.setOnEpisodeStarted(func() { journal.add(episodeEvent) })
		h.setOnAttempt(func(n int64) {
			journal.add(attemptEvent)

			if n%attemptsPerEpisode == 0 {
				h.setQuorum(true)
			}
		})

		stop := h.run()
		defer stop()

		waitFor(t, func() bool { return h.attempts.Load() == attemptsPerEpisode })
		idleFor(3 * idleTick)

		h.setQuorum(false)

		waitFor(t, func() bool { return h.attempts.Load() == 2*attemptsPerEpisode })
		idleFor(3 * idleTick)

		if got := h.episodes.Load(); got != 2 {
			t.Errorf("EpisodeStarted calls = %d over two episodes, want 2", got)
		}

		want := []string{
			episodeEvent, attemptEvent, attemptEvent, attemptEvent,
			episodeEvent, attemptEvent, attemptEvent, attemptEvent,
		}
		if got := journal.snapshot(); !slices.Equal(got, want) {
			t.Errorf("events = %v, want %v: one EpisodeStarted before the first attempt of each episode", got, want)
		}
	})
}

func TestRejoinJitterStaysWithinTwentyPercent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const samples = 200

		params := Params{Interval: maxInterval, MaxInterval: maxInterval}

		h := newHarness(t, params)
		h.setAttemptResult(func(ctx context.Context, n int64) error {
			if n > samples {
				h.cancel()

				return ctx.Err()
			}

			return errTransport
		})

		done := make(chan struct{})

		go func() {
			defer close(done)

			_ = h.loop.Run(h.ctx)
		}()

		<-done

		sleeps := h.recordedSleeps()
		if len(sleeps) != samples {
			t.Fatalf("sleeps = %d, want %d, one after each failed attempt", len(sleeps), samples)
		}

		below, above := 0, 0

		for _, got := range sleeps {
			within(t, got, maxInterval)

			switch {
			case got < maxInterval:
				below++
			case got > maxInterval:
				above++
			}
		}

		if slices.Min(sleeps) == slices.Max(sleeps) {
			t.Errorf("all %d sleeps are %s, want them jittered", samples, sleeps[0])
		}

		if below == 0 || above == 0 {
			t.Errorf("sleeps below the ceiling: %d, above it: %d; want the jitter on both sides", below, above)
		}
	})
}

func TestDepsHaveNoAPIGate(t *testing.T) {
	deps := reflect.TypeOf(Deps{})

	got := make([]string, 0, deps.NumField())
	for i := range deps.NumField() {
		got = append(got, deps.Field(i).Name)
	}

	slices.Sort(got)

	want := []string{"Attempt", "Changed", "EpisodeStarted", "HasQuorum", "NotMember", "OwnFailedRecord", "Sleep"}
	if !slices.Equal(got, want) {
		t.Errorf("Deps fields = %v, want %v", got, want)
	}
}

func cutOwnReadShort(h *harness, read int64) {
	h.setOnOwnRead(func(ctx context.Context, n int64) bool {
		if n < read {
			return true
		}

		select {
		case <-ctx.Done():
		case <-h.ctx.Done():
		}

		if ctx.Err() == nil {
			h.t.Errorf("OwnFailedRecord read %d got a ctx that shutdown does not cancel", n)
		}

		return false
	})
}

func TestRejoinStopsOnCancelDuringSleepAndDuringAnAttempt(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(h *harness)
		reach    func(t *testing.T, h *harness)
		attempts int64
		sleeps   int
	}{
		{
			name:  "cancel during a sleep",
			setup: func(*harness) {},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return len(h.recordedSleeps()) == 2 })
			},
			attempts: 2,
			sleeps:   2,
		},
		{
			name: "cancel while an attempt waits for ctx",
			setup: func(h *harness) {
				h.setAttemptResult(func(ctx context.Context, n int64) error {
					if n == 1 {
						return nil
					}

					<-ctx.Done()

					return ctx.Err()
				})
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.attempts.Load() == 2 })
			},
			attempts: 2,
			sleeps:   1,
		},
		{
			name: "cancel during the trigger read before an attempt",
			setup: func(h *harness) {
				h.setQuorum(true)
				cutOwnReadShort(h, 3)
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.ownReads.Load() == 3 })
			},
			attempts: 1,
			sleeps:   1,
		},
		{
			name: "cancel during the trigger read after a successful attempt",
			setup: func(h *harness) {
				h.setQuorum(true)
				cutOwnReadShort(h, 4)
			},
			reach: func(t *testing.T, h *harness) {
				t.Helper()

				waitFor(t, func() bool { return h.ownReads.Load() == 4 })
			},
			attempts: 2,
			sleeps:   1,
		},
		{
			name:  "cancel while idle without an episode",
			setup: func(h *harness) { h.setQuorum(true) },
			reach: func(*testing.T, *harness) { idleFor(3 * idleTick) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, testParams)
				tc.setup(h)

				done := make(chan struct{})

				var runErr error

				go func() {
					defer close(done)

					runErr = h.loop.Run(h.ctx)
				}()

				tc.reach(t, h)

				h.cancel()

				synctest.Wait()

				select {
				case <-done:
				default:
					t.Fatal("Run did not return right after cancel")
				}

				if runErr != nil {
					t.Errorf("Run = %v on shutdown, want nil", runErr)
				}

				if got := h.attempts.Load(); got != tc.attempts {
					t.Errorf("attempts = %d, want %d", got, tc.attempts)
				}

				if sleeps := h.recordedSleeps(); len(sleeps) != tc.sleeps {
					t.Errorf("sleeps = %v, want %d", sleeps, tc.sleeps)
				}
			})
		})
	}
}
