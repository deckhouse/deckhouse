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
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// DefaultWindow bounds how long a fired schedule keeps trying.
//
// Replicas collect one at a time, so on a three-master cluster the third waits for two
// collections before its own. The bound is what keeps that queue from running into the
// working day if one of them is slow, and what keeps a replica that cannot finish from
// holding the others out forever: the run is abandoned and the next schedule tries again.
const DefaultWindow = 4 * time.Hour

// Plan is what the desired state currently says about collecting. Read at each firing rather
// than once at startup, so that changing the schedule does not need a restart.
type Plan struct {
	// Enabled is whether to collect at all.
	Enabled bool

	// Schedule is a five-field cron expression.
	Schedule string
}

// Store is the replica's own registry, as the scheduler acts on it.
type Store interface {
	// Quiesce makes the registry refuse writes, returning what restores it.
	//
	// Restoring is the caller's responsibility and must happen even when the collection
	// fails, which is why it is a function and not a second method: a replica left
	// read-only by a crashed collection would refuse a `d8 mirror push` with nothing to
	// explain why.
	Quiesce(ctx context.Context) (restore func(), err error)
}

// Lease serialises collection across replicas.
type Lease interface {
	// Hold blocks until this replica may collect, then calls work, then gives the lease
	// up. Returns whatever work returned, or an error if the lease was never obtained.
	Hold(ctx context.Context, work func(context.Context) error) error
}

// Scheduler collects this replica's store on a schedule.
type Scheduler struct {
	Log *slog.Logger

	// Plan reads the current instruction from the desired state.
	Plan func() Plan

	// Lease serialises collection across replicas, so that at most one is read-only at any
	// moment.
	Lease Lease

	// Store is this replica's registry.
	Store Store

	// Collect does the work.
	Collect func(ctx context.Context) (Report, error)

	// Publish records the outcome, whatever it was. Called even for a failure: "it has not
	// collected since Tuesday" is the useful fact, and it is invisible if only successes
	// are recorded.
	Publish func(ctx context.Context, finished time.Time, err error)

	// Window bounds a firing. Zero means DefaultWindow.
	Window time.Duration

	// Now is the clock, so a test does not have to wait for 3am.
	Now func() time.Time
}

func (s *Scheduler) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s *Scheduler) window() time.Duration {
	if s.Window <= 0 {
		return DefaultWindow
	}
	return s.Window
}

// Run collects on schedule until the context ends.
//
// Never returns an error. A replica that cannot collect still serves every image it holds, so
// there is nothing here worth taking the process down for — and taking it down would stop the
// filling and reporting that share it.
func (s *Scheduler) Run(ctx context.Context) {
	for ctx.Err() == nil {
		plan := s.Plan()

		if !plan.Enabled {
			// Disabled, which is a configuration and not a problem. Re-read on a timer
			// rather than watched, because turning it on is not urgent.
			if !sleep(ctx, time.Hour) {
				return
			}
			continue
		}

		schedule, err := cron.ParseStandard(plan.Schedule)
		if err != nil {
			// A schedule that cannot be parsed is reported and retried, not defaulted
			// around: silently collecting at some other hour than the one an operator
			// wrote would be worse than not collecting.
			s.Log.Error("the garbage collection schedule cannot be read, so nothing is collected",
				"schedule", plan.Schedule, "error", err.Error())
			if !sleep(ctx, time.Hour) {
				return
			}
			continue
		}

		now := s.now()
		next := schedule.Next(now)
		wait := next.Sub(now)

		s.Log.Info("the next garbage collection is scheduled", "at", next.Format(time.RFC3339))
		if !sleep(ctx, wait) {
			return
		}

		s.fire(ctx)
	}
}

// fire runs one collection, bounded by the window and serialised against the other replicas.
func (s *Scheduler) fire(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, s.window())
	defer cancel()

	err := s.Lease.Hold(ctx, func(ctx context.Context) error {
		return s.collect(ctx)
	})
	if err == nil {
		return
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// Another replica held the lease for the whole window. Not a failure: the schedule
		// fires again, and the queue moves.
		s.Log.Info("the collection window closed before this replica's turn came")
		return
	}
	s.Log.Error("the garbage collection failed", "error", err.Error())
}

// collect is the sequence that must not be interrupted halfway: refuse writes, collect,
// accept writes again.
func (s *Scheduler) collect(ctx context.Context) error {
	restore, err := s.Store.Quiesce(ctx)
	if err != nil {
		return fmt.Errorf("making the registry read-only: %w", err)
	}
	// Deferred, so that a panic or an early return still hands writes back. A replica left
	// read-only by a crashed collection would refuse a `d8 mirror push` with nothing to
	// explain why.
	defer restore()

	s.Log.Info("the registry is read-only for the duration of a garbage collection")

	report, collectErr := s.Collect(ctx)
	if s.Publish != nil {
		s.Publish(ctx, s.now(), collectErr)
	}

	if collectErr != nil {
		return collectErr
	}

	s.Log.Info("the garbage collection finished",
		"considered", report.Considered, "deleted", len(report.Deleted),
		"failed", len(report.Failed), "kept", report.Kept, "swept", report.Swept)
	return nil
}

// sleep waits, and reports whether the wait finished rather than the context ending.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
