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

// Package retry is the backoff of every execution failure in the lifecycle and the Clock its
// deadlines are read with.
package retry

import "time"

// Default bounds of a backoff: the delay doubles from Min up to Max. Max is every engine backoff's
// cap: a package recovers within it after a transient outage.
const (
	Min = time.Second
	Max = 2 * time.Minute
)

// nextBackoff returns the delay after a failure that followed a delay of previous, within lo and hi.
func nextBackoff(previous, lo, hi time.Duration) time.Duration {
	return min(max(2*previous, lo), hi)
}

// Backoff delays a retry of one failed attempt; any other attempt is not delayed. The key names the
// attempt (an operation and its Inputs, a configuration): new Inputs or a new configuration are
// never delayed by the backoff of an older attempt, and a failure of another key starts the delay
// again from the first delay. The zero value holds nothing and doubles from Min up to Max. Not safe
// for concurrent use: one loop owns it.
type Backoff struct {
	// Initial is the first delay instead of Min when set.
	Initial time.Duration

	key   string
	delay time.Duration
	until time.Time
}

// Failed records a failure of key at now and returns until when a retry of key is held. The delay
// doubles while the same key keeps failing.
func (b *Backoff) Failed(key string, now time.Time) time.Time {
	if key != b.key {
		b.delay = 0
	}

	lo := Min
	if b.Initial > 0 {
		lo = b.Initial
	}

	b.key = key
	b.delay = nextBackoff(b.delay, lo, Max)
	b.until = now.Add(b.delay)

	return b.until
}

// Holds reports whether an attempt of key must still wait at now: only the key that failed last,
// and only before its deadline.
func (b *Backoff) Holds(key string, now time.Time) bool {
	return b.key == key && !b.until.IsZero() && now.Before(b.until)
}

// Until returns the deadline of the last failure; zero when nothing failed since the last success.
// The owner wakes at it to retry.
func (b *Backoff) Until() time.Time {
	return b.until
}

// Earliest returns the earliest deadline of backoffs after now; zero when none is. The owner's one
// wake timer fires then (Arm); a deadline of an attempt the owner no longer wants only costs one
// more decision.
func Earliest(now time.Time, backoffs ...Backoff) time.Time {
	var earliest time.Time

	for _, b := range backoffs {
		if b.until.After(now) && (earliest.IsZero() || b.until.Before(earliest)) {
			earliest = b.until
		}
	}

	return earliest
}

// Arm sets t to fire at at on c, or stops it when at is zero. at is a deadline computed once, from
// the time the owner decided at; one that passed since fires at once.
func Arm(t Timer, c Clock, at time.Time) {
	if at.IsZero() {
		t.Stop()
		return
	}

	t.Reset(at.Sub(c.Now()))
}

// Succeeded forgets the failure, keeping Initial: the next failure starts from the first delay.
func (b *Backoff) Succeeded() {
	*b = Backoff{Initial: b.Initial}
}
