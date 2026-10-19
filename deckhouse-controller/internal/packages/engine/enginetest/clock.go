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

package enginetest

import (
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
)

// Epoch is where the kit's clock starts: a Monday, 10:00 UTC.
var Epoch = time.Date(2026, time.January, 5, 10, 0, 0, 0, time.UTC)

// Clock is the engine's clock in the kit (the package Reconcilers, Currents and Global): time moves
// only when a test advances it, and every armed timer wakes the kit's waiters.
type Clock struct {
	mu    sync.Mutex
	now   time.Time
	armed map[*fakeTimer]time.Time // armed maps each armed timer to its deadline.
	// read is the time each owner (Owned) last read; its timers count from it.
	read  map[string]time.Time
	onArm func() // onArm wakes the kit's waiters; called outside mu.
}

var _ retry.Clock = (*Clock)(nil)

func newClock(onArm func()) *Clock {
	return &Clock{now: Epoch, armed: make(map[*fakeTimer]time.Time), read: make(map[string]time.Time), onArm: onArm}
}

// Now returns the kit's time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// NewTimer returns a timer armed at now+d.
func (c *Clock) NewTimer(d time.Duration) retry.Timer {
	return c.newTimer("", d)
}

// Owned returns the clock as owner sees it: its timers carry owner, so a test can wait for them
// (NextOf) rather than for whichever timer is armed first. A Reset counts from the time owner last
// read, as retry.Arm means it: a test advancing between the owner's Now and its Reset would
// otherwise move the deadline by the whole advance and lose the fire Reset discards.
func (c *Clock) Owned(owner string) retry.Clock {
	return ownedClock{clock: c, owner: owner}
}

func (c *Clock) newTimer(owner string, d time.Duration) *fakeTimer {
	t := &fakeTimer{clock: c, owner: owner, ch: make(chan time.Time, 1)}
	t.Reset(d)

	return t
}

// Advance moves the time by d and fires every timer due by then.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.fireLocked()
	c.mu.Unlock()
}

// Next returns the earliest deadline of the armed timers; false when none is armed.
func (c *Clock) Next() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var next time.Time
	for _, at := range c.armed {
		if next.IsZero() || at.Before(next) {
			next = at
		}
	}

	return next, !next.IsZero()
}

// NextOf is Next over the timers of owner (Owned).
func (c *Clock) NextOf(owner string) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var next time.Time
	for t, at := range c.armed {
		if t.owner == owner && (next.IsZero() || at.Before(next)) {
			next = at
		}
	}

	return next, !next.IsZero()
}

// ArmedAt returns how many timers are armed for exactly at.
func (c *Clock) ArmedAt(at time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := 0
	for _, a := range c.armed {
		if a.Equal(at) {
			n++
		}
	}

	return n
}

// AdvanceTo moves the time to at (never back) and fires every timer due by then.
func (c *Clock) AdvanceTo(at time.Time) {
	c.mu.Lock()
	if at.After(c.now) {
		c.now = at
	}

	c.fireLocked()
	c.mu.Unlock()
}

// fireLocked fires and disarms every timer due at now; c.mu held.
func (c *Clock) fireLocked() {
	for t, at := range c.armed {
		if at.After(c.now) {
			continue
		}

		delete(c.armed, t)

		select {
		case t.ch <- c.now:
		default:
		}
	}
}

// ownedClock is a Clock whose timers carry owner.
type ownedClock struct {
	clock *Clock
	owner string
}

func (o ownedClock) Now() time.Time {
	c := o.clock

	c.mu.Lock()
	defer c.mu.Unlock()

	c.read[o.owner] = c.now

	return c.now
}

func (o ownedClock) NewTimer(d time.Duration) retry.Timer { return o.clock.newTimer(o.owner, d) }

// fakeTimer has time.Timer semantics since Go 1.23: Reset and Stop discard a pending fire.
type fakeTimer struct {
	clock *Clock
	owner string // owner is the Owned clock's owner; empty for the plain Clock.
	ch    chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Reset(d time.Duration) bool {
	c := t.clock

	c.mu.Lock()
	_, was := c.armed[t]
	t.drain()

	from := c.now
	if read, ok := c.read[t.owner]; ok && t.owner != "" {
		from = read
	}

	c.armed[t] = from.Add(d)
	c.fireLocked()
	c.mu.Unlock()

	c.onArm()

	return was
}

func (t *fakeTimer) Stop() bool {
	c := t.clock

	c.mu.Lock()
	defer c.mu.Unlock()

	_, was := c.armed[t]
	delete(c.armed, t)
	t.drain()

	return was
}

func (t *fakeTimer) drain() {
	select {
	case <-t.ch:
	default:
	}
}
