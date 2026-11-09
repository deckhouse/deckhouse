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
	"log/slog"
	"sync/atomic"
	"time"
)

// How often the estimate is redone without anything asking for it, how soon after the last one it may
// be redone at all, and how often the conditions for it are looked at.
//
// Rarely, because what it answers — is a collection worth starting now — moves with the image set
// and with collections, and both are events this watches for. A set that changes four times in twenty
// minutes while modules update is not worth four estimates, hence the floor.
const (
	estimateEvery    = 30 * time.Minute
	estimateFloor    = 5 * time.Minute
	estimateLookEach = time.Minute
)

// Estimator keeps an estimate of what a garbage collection of this replica's store would free.
//
// For a person deciding whether to start one: an estimate of zero says a collection now would do
// nothing. Redone when the image set changes, after a collection, and every half hour otherwise — and
// never while the store is being written to. A store being filled is one whose garbage cannot be told
// from what is still going in, and recounting it on every pass would read the whole store again and
// again for a number nobody can act on until the fill is done. The last estimate stays meanwhile, with
// the time it was made.
type Estimator struct {
	Log *slog.Logger

	// Plan, Filling and Pushing are the worker's: the same reasons to keep away from the store.
	Plan    func() Plan
	Filling func() bool
	Pushing func() bool

	// Collecting reports whether a collection is running on this replica now.
	Collecting func() bool

	// Set is the fingerprint of the image set the cluster needs now, empty while unknown.
	Set func() string

	// Estimate works the figure out.
	Estimate func(ctx context.Context) (int64, error)

	// Publish records it, with when it was made.
	Publish func(bytes int64, at time.Time)

	// Now is the clock.
	Now func() time.Time

	// Sleep is the waiting, so a test can see the decisions without the minutes between them.
	Sleep func(ctx context.Context, d time.Duration) bool

	collected  atomic.Bool
	lastSet    string
	lastAt     time.Time
	lastAsked  time.Time
	lastFailed string
}

// Collected says a collection on this replica has finished, which makes the estimate stale.
func (e *Estimator) Collected() {
	e.collected.Store(true)
}

func (e *Estimator) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

func (e *Estimator) sleep(ctx context.Context, d time.Duration) bool {
	if e.Sleep == nil {
		return sleep(ctx, d)
	}
	return e.Sleep(ctx, d)
}

// Run keeps the estimate until the context ends.
func (e *Estimator) Run(ctx context.Context) {
	for e.sleep(ctx, estimateLookEach) {
		e.step(ctx)
	}
}

// step redoes the estimate if it is owed and the store is quiet.
//
// Whether it is owed is asked first: it costs nothing, while the plan may measure the whole store and
// the push check reads the uploads — once a minute for an estimate due every half hour.
func (e *Estimator) step(ctx context.Context) {
	now := e.now()
	set := e.Set()
	owed := e.lastAt.IsZero() || e.collected.Load() || (set != "" && set != e.lastSet) ||
		now.Sub(e.lastAt) >= estimateEvery
	// The floor holds for asking too: a store being filled measures as a whole store, and asked every
	// minute of an hour-long fill it was walked sixty times for an estimate it could not have.
	if !owed || (!e.lastAsked.IsZero() && now.Sub(e.lastAsked) < estimateFloor) {
		return
	}
	e.lastAsked = now

	plan := e.Plan()
	if !plan.Known || plan.FillPending || isTrue(e.Filling) || isTrue(e.Pushing) || isTrue(e.Collecting) {
		return
	}

	// Taken rather than read: a collection that finishes while this estimate runs makes it stale too.
	// Put back if the estimate fails, so the figure from before that collection does not stand for
	// another half hour.
	collected := e.collected.Swap(false)
	bytes, err := e.Estimate(ctx)
	if err != nil {
		if collected {
			e.collected.Store(true)
		}
		// Said once per distinct failure, and the last estimate is left standing with its time: it is
		// old, and its timestamp says so.
		if message := err.Error(); message != e.lastFailed {
			e.lastFailed = message
			e.Log.Warn("cannot estimate what a garbage collection would free", "error", message)
		}
		return
	}
	e.lastFailed = ""
	e.lastSet, e.lastAt = set, now
	e.Publish(bytes, now)
	e.Log.Info("estimated what a garbage collection would free",
		"bytes", bytes, "took", e.now().Sub(now).Round(time.Millisecond).String())
}

func isTrue(question func() bool) bool {
	return question != nil && question()
}
