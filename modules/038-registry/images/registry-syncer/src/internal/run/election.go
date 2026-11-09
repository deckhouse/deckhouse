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
// The plain election underneath is untouched: this decides when to enter it and when to walk
// out of one already won. Entering only when eligible keeps an empty replica from taking the
// lease from a full one, which in air-gap would leave the leader unable to fill itself while
// the followers replicate its emptiness. Walking out matters because completeness can be
// elsewhere than the lease. Not because of a push: through the publication ingress it lands on
// the lease holder, which is what the endpoint names. But a replica that kept the set can watch
// the lease go to one that did not, and a leader whose registry stops answering or whose disk
// refuses writes is full no longer; without stepping down the cluster sits with that leader
// beside a full follower.
//
// A leader that is genuinely full never becomes ineligible, so a healthy cluster re-elects
// nothing.
type Candidate struct {
	Log *slog.Logger

	// Eligible reports whether this replica ought to hold the lease, and when it ought not,
	// why: the reason is what the replica logs when it steps aside, and, once each time it
	// changes, while it stands aside.
	//
	// An error is treated as "keep doing what you were doing": the answer comes from the
	// API server, and an unreachable one is no reason to hand over a lease that is
	// working — nor to grab one that is not.
	Eligible func(context.Context) (bool, string, error)

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
//
// It says why it does not stand, once each time the reason changes, and when it begins to. A replica
// that was never eligible used to log nothing at all: on a stand the election stalled for four minutes
// while nobody was full, and no log said which replica the others were waiting for. Once rather than at
// every review, because a replica may stand aside for the life of the cluster.
func (c *Candidate) Run(ctx context.Context) {
	said, standing := "", false
	for ctx.Err() == nil {
		ok, why := c.eligible(ctx, false)
		if !ok {
			standing = false
			// Empty when Eligible failed, which eligible has said already; the reason before it stands.
			if why != "" && why != said {
				c.Log.Info("this replica does not stand in the election", "reason", why)
				said = why
			}
			if !sleep(ctx, c.review()) {
				return
			}
			continue
		}

		if !standing {
			c.Log.Info("this replica stands in the election")
		}
		said, standing = "", true
		// A step aside says why, so the loop above does not say it again.
		if why := c.attempt(ctx); why != "" {
			said, standing = why, false
		}
	}
}

// attempt runs the election until this replica should stop standing in it, and returns why it stepped
// aside: empty when the election ended on its own, or the context did.
func (c *Candidate) attempt(ctx context.Context) string {
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
			return ""
		case <-attemptCtx.Done():
			<-done
			return ""
		case <-ticker.C:
			ok, why := c.eligible(attemptCtx, true)
			if ok {
				continue
			}

			c.Log.Info("this replica should not hold the lease any more, so it steps aside", "reason", why)
			// Cancelling releases the lease, rather than leaving the others to wait out
			// its duration. Which matters: the replica taking over is the one with the
			// images, and everything else is waiting on it.
			cancel()
			<-done
			return why
		}
	}
}

// eligible asks the question and decides what an unanswerable one means.
//
// `holding` is what to assume when the answer cannot be had: a replica in the election stays
// in it, and a replica standing aside stays aside. In both cases the alternative would be to
// change the cluster's arrangement on the strength of not knowing.
//
// Standing aside matters as much as staying in. An API server that does not answer for half a
// minute is also one the leader cannot renew through; a follower that read the silence as "go
// ahead" was in the election by the time the lease ran out, and the lease went to whichever
// replica asked first — a full leader replaced by another full replica, for nothing.
//
// The reason is the one Eligible gave, and empty when it gave none.
func (c *Candidate) eligible(ctx context.Context, holding bool) (bool, string) {
	if c.Eligible == nil {
		return true, ""
	}

	ok, why, err := c.Eligible(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.Log.Warn("cannot tell whether this replica should lead, keeping the current arrangement",
				"error", err.Error(), "standing", holding)
		}
		return holding, ""
	}
	return ok, why
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
