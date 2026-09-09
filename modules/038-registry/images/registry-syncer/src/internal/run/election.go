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

package run

import (
	"context"
	"log/slog"
	"time"
)

// Candidate stands in the storage leader election, but only while this replica ought to
// hold the lease.
//
// The plain election underneath is untouched — this decides when to enter it, and when to
// walk out of one already won. Both halves are needed:
//
//   - Entering only when eligible keeps an empty replica from winning the lease from a
//     full one, which in air-gap would leave the leader unable to fill itself while the
//     followers replicate its emptiness.
//   - Walking out matters because completeness can appear elsewhere while this replica
//     holds the lease: `d8 mirror push` arrives through the publication endpoint and lands
//     on whichever replica the ingress chose, not necessarily the leader. Without stepping
//     down, the cluster would sit with an empty leader and a full follower and recover on
//     its own from neither.
//
// A leader that is genuinely full never becomes ineligible, so a healthy cluster
// re-elects nothing.
type Candidate struct {
	Log *slog.Logger

	// Eligible reports whether this replica ought to hold the lease.
	//
	// An error is treated as "keep doing what you were doing": the answer comes from the
	// API server, and an unreachable one is no reason to hand over a lease that is
	// working — nor to grab one that is not.
	Eligible func(context.Context) (bool, error)

	// Elect runs one attempt at the election, returning when the context is cancelled
	// or leadership is lost. Called again for each attempt, because an elector cannot
	// be reused once its context is done.
	Elect func(context.Context)

	// Review is how often eligibility is re-read, while standing and while leading.
	Review time.Duration
}

func (c *Candidate) review() time.Duration {
	if c.Review <= 0 {
		return 15 * time.Second
	}
	return c.Review
}

// Run stands in the election for as long as this replica should, until the context ends.
func (c *Candidate) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if !c.eligible(ctx, false) {
			if !sleep(ctx, c.review()) {
				return
			}
			continue
		}

		c.attempt(ctx)
	}
}

// attempt runs the election until this replica should stop standing in it.
func (c *Candidate) attempt(ctx context.Context) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Elect(attemptCtx)
	}()

	ticker := time.NewTicker(c.review())
	defer ticker.Stop()

	for {
		select {
		case <-done:
			// The election ended on its own: leadership lost, or the lease could not be
			// renewed. Either way it is over and the outer loop reconsiders.
			return
		case <-attemptCtx.Done():
			<-done
			return
		case <-ticker.C:
			if c.eligible(attemptCtx, true) {
				continue
			}

			c.Log.Info("another replica holds the whole image set, so this one steps aside")
			// Cancelling releases the lease, rather than leaving the others to wait out
			// its duration. Which matters: the replica taking over is the one with the
			// images, and everything else is waiting on it.
			cancel()
			<-done
			return
		}
	}
}

// eligible asks the question and decides what an unanswerable one means.
//
// `holding` is what to assume when the answer cannot be had: a replica that is standing
// stays standing, a replica that is leading keeps leading. In both cases the alternative
// would be to change the cluster's arrangement on the strength of not knowing.
func (c *Candidate) eligible(ctx context.Context, holding bool) bool {
	if c.Eligible == nil {
		return true
	}

	ok, err := c.Eligible(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.Log.Warn("cannot tell whether this replica should lead, keeping the current arrangement",
				"error", err.Error(), "leading", holding)
		}
		return true
	}
	return ok
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
