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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	interval    = time.Millisecond
	maxInterval = 4 * time.Millisecond
)

type harness struct {
	loop *Loop

	mu         sync.Mutex
	quorum     bool
	api        bool
	attempts   atomic.Int64
	sleeps     []time.Duration
	changed    chan struct{}
	onAttempt  func(n int64)
	attemptErr error
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{api: true, changed: make(chan struct{}, 1)}

	h.loop = New(Params{Interval: interval, MaxInterval: maxInterval}, Deps{
		Attempt: func(context.Context) error {
			n := h.attempts.Add(1)

			h.mu.Lock()
			hook, err := h.onAttempt, h.attemptErr
			h.mu.Unlock()

			if hook != nil {
				hook(n)
			}

			return err
		},
		HasQuorum:    func() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.quorum },
		APIReachable: func() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.api },
		Changed:      h.changed,
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
func (h *harness) setAPI(v bool)    { h.mu.Lock(); h.api = v; h.mu.Unlock() }

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

func (h *harness) recordedSleeps() []time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]time.Duration(nil), h.sleeps...)
}

func (h *harness) run(t *testing.T) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		_ = h.loop.Run(ctx)
		close(done)
	}()

	return func() {
		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("rejoin loop did not stop after cancel")
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}

		time.Sleep(200 * time.Microsecond)
	}
}

func within(t *testing.T, got, nominal time.Duration) {
	t.Helper()

	lo, hi := nominal*8/10, nominal*12/10
	if got < lo || got > hi {
		t.Errorf("delay %s is outside [%s, %s]", got, lo, hi)
	}
}

func TestRejoinRepeatsUntilQuorumIsBack(t *testing.T) {
	h := newHarness(t)
	h.quorumAfter(3)

	stop := h.run(t)
	defer stop()

	waitFor(t, func() bool { return h.attempts.Load() == 3 })
	time.Sleep(5 * maxInterval)

	if got := h.attempts.Load(); got != 3 {
		t.Errorf("attempts = %d after quorum returned, want 3", got)
	}

	sleeps := h.recordedSleeps()
	if len(sleeps) != 2 {
		t.Fatalf("sleeps = %v, want one after each attempt that left quorum missing", sleeps)
	}

	within(t, sleeps[0], interval)
	within(t, sleeps[1], 2*interval)
}

func TestBackoffIsCappedAndResetsAfterQuorum(t *testing.T) {
	h := newHarness(t)
	h.quorumAfter(5)

	stop := h.run(t)
	defer stop()

	waitFor(t, func() bool { return h.attempts.Load() == 5 })
	time.Sleep(5 * maxInterval)

	sleeps := h.recordedSleeps()
	if len(sleeps) != 4 {
		t.Fatalf("sleeps = %v, want 4", sleeps)
	}

	within(t, sleeps[2], maxInterval)
	within(t, sleeps[3], maxInterval)

	h.quorumAfter(7)
	h.setQuorum(false)
	h.changed <- struct{}{}

	waitFor(t, func() bool { return h.attempts.Load() == 7 })

	sleeps = h.recordedSleeps()
	if len(sleeps) != 5 {
		t.Fatalf("sleeps = %v, want 5", sleeps)
	}

	within(t, sleeps[4], interval)
}

func TestNoRejoinWithoutTheAPI(t *testing.T) {
	h := newHarness(t)
	h.setAPI(false)

	stop := h.run(t)
	defer stop()

	waitFor(t, func() bool { return len(h.recordedSleeps()) >= 3 })

	if got := h.attempts.Load(); got != 0 {
		t.Fatalf("attempts = %d while the API is unreachable, want none", got)
	}

	h.setAPI(true)

	waitFor(t, func() bool { return h.attempts.Load() >= 1 })
}

func TestAttemptErrorsBackOffLikeAFailedRejoin(t *testing.T) {
	h := newHarness(t)
	h.failAttempts(errors.New("no seed accepted the connection"))

	stop := h.run(t)
	defer stop()

	waitFor(t, func() bool { return len(h.recordedSleeps()) >= 3 })

	sleeps := h.recordedSleeps()
	within(t, sleeps[0], interval)
	within(t, sleeps[1], 2*interval)
	within(t, sleeps[2], 4*interval)
}
