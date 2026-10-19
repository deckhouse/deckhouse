// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The lane: a FIFO of items with one goroutine.

package hooklane

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// State is what a lane is doing.
type State string

const (
	StateIdle     State = "idle"     // StateIdle: nothing waits.
	StateRunning  State = "running"  // StateRunning: the head runs.
	StateRetrying State = "retrying" // StateRetrying: the head failed and waits for its retry.
	// StateThrottled: the head's hook ran as often as its executionMinInterval allows; it waits,
	// and what arrives for it meanwhile joins it.
	StateThrottled State = "throttled"
	StateStopping  State = "stopping" // StateStopping: closed; the run in flight, if any, ends it.
)

// lane runs the items of one queue of a package, one at a time, in delivery order. A failed head
// stays and blocks the lane until its retry succeeds.
type lane struct {
	lanes  *Lanes
	queue  string
	wake   chan struct{} // wake has capacity 1: a delivery or a close.
	exited chan struct{} // exited is closed when the goroutine returned.

	// mu guards everything below; never held while a hook runs.
	mu     sync.Mutex
	closed bool
	// items are the waiting items; items[0] is the head, running while state is StateRunning.
	items     []*item
	seq       uint64 // seq numbers the delivered items: the head's identity for its backoff.
	state     State
	startedAt time.Time // startedAt is when the head started running.
	failure   failure   // failure is the head's, while it waits for its retry.
	// backoff holds the failed head's retry: retryInitial, doubling up to retry.Max, keyed by the head.
	backoff retry.Backoff
	stats   Stats
}

// failure is the head's failed attempts.
type failure struct {
	err      string
	attempts int
}

// retryInitial is a failed head's first retry delay; the cap is retry.Max, as for every backoff.
const retryInitial = 5 * time.Second

func newLane(lanes *Lanes, queue string) *lane {
	return &lane{
		lanes:   lanes,
		queue:   queue,
		wake:    make(chan struct{}, 1),
		exited:  make(chan struct{}),
		state:   StateIdle,
		backoff: retry.Backoff{Initial: retryInitial},
	}
}

// deliver appends it, or merges it into the last waiting item when that is a run of the same hook
// (mergeable): the hook then runs once with every context, in order (compacted by group), and the
// order between hooks is kept. The head merges too while it waits for its retry or its rate, so a failed batch is
// retried with what arrived since; a running head never does. False when the lane is closed, so a
// delivery after the stop began is dropped. Never blocks.
func (ln *lane) deliver(it *item) bool {
	ln.mu.Lock()
	if ln.closed {
		ln.mu.Unlock()
		return false
	}

	if n := len(ln.items); n > 0 && (n > 1 || ln.startedAt.IsZero()) && mergeable(ln.items[n-1], it) {
		last := ln.items[n-1]
		last.contexts = appendCompact(last.contexts, it.contexts...)
		ln.mu.Unlock()

		ln.signal()

		return true
	}

	ln.seq++
	it.seq = ln.seq
	it.delivered = ln.lanes.router.clock.Now()
	ln.items = append(ln.items, it)
	ln.mu.Unlock()

	ln.signal()

	return true
}

func (ln *lane) signal() {
	select {
	case ln.wake <- struct{}{}:
	default:
	}
}

// close drops the waiting items, refuses every later delivery and wakes the goroutine; the run in
// flight, if any, finishes.
func (ln *lane) close() {
	ln.mu.Lock()
	ln.closed = true
	ln.state = StateStopping

	for _, it := range ln.items {
		it.leave()
	}

	if ln.startedAt.IsZero() {
		ln.items = nil
	} else {
		ln.items = ln.items[:1]
	}
	ln.mu.Unlock()

	ln.signal()
}

// loop runs the head whenever it is due, until ctx ends. A run that ends because ctx ended is not
// recorded: the item dies with the lane.
func (ln *lane) loop(ctx context.Context) {
	defer close(ln.exited)

	clock := ln.lanes.router.clock

	timer := clock.NewTimer(time.Hour)
	timer.Stop()

	defer timer.Stop()

	for {
		it, retryAt := ln.take()
		if it == nil {
			retry.Arm(timer, clock, retryAt)

			select {
			case <-ctx.Done():
				return
			case <-ln.wake:
			case <-timer.C():
			}

			continue
		}

		err := ln.lanes.exec(ctx, it)
		if ctx.Err() != nil {
			return
		}

		ln.finish(it, err)
	}
}

// take starts the head if it is due; otherwise nil and when the head's retry is due (zero: wait
// for a delivery).
func (ln *lane) take() (*item, time.Time) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.closed || len(ln.items) == 0 {
		if !ln.closed {
			ln.state = StateIdle
		}

		return nil, time.Time{}
	}

	now := ln.lanes.router.clock.Now()
	head := ln.items[0]

	if ln.backoff.Holds(head.key(), now) {
		ln.state = StateRetrying
		return nil, ln.backoff.Until()
	}

	if lim := ln.lanes.limiters[head.hook]; lim != nil && head.runsHook() {
		if at, ok := lim.take(now); !ok {
			ln.state = StateThrottled
			return nil, at
		}
	}

	ln.state = StateRunning
	ln.startedAt = now

	return head, time.Time{}
}

// finish ends the head's run: a success, or a failure the item allows (allowedFailure), pops it; a
// failure keeps it at the head until its retry.
func (ln *lane) finish(it *item, err error) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	now := ln.lanes.router.clock.Now()
	ln.startedAt = time.Time{}
	ln.stats.Runs++

	var allowed allowedFailure
	if err == nil || errors.As(err, &allowed) {
		ln.items = ln.items[1:]
		ln.failure = failure{}
		ln.backoff.Succeeded()
		ln.state = StateIdle

		if err != nil {
			ln.stats.AllowedFailures++
			ln.stats.LastAllowedFailure = it.hook + ": " + err.Error()
		} else {
			ln.stats.LastSuccess = now
		}

		it.leave()

		return
	}

	ln.stats.Failures++
	ln.failure.err = err.Error()
	ln.failure.attempts++

	until := ln.backoff.Failed(it.key(), now)
	ln.state = StateRetrying

	ln.lanes.logger.Warn("hook failed, the lane waits for its retry", slog.String("queue", ln.queue),
		slog.String("hook", it.hook), slog.Duration("retry_in", until.Sub(now)), log.Err(err))
}

// inFlightHook is the hook the lane runs now; empty while none runs.
func (ln *lane) inFlightHook() string {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.startedAt.IsZero() {
		return ""
	}

	return ln.items[0].hook
}
