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
	// Known is whether the desired state has been read at all yet.
	//
	// Separate from Enabled because the two were confused, and the confusion was expensive: a
	// replica that has not yet read its instruction has an empty plan, an empty plan is
	// indistinguishable from a disabled one, and the disabled path waits an hour before looking
	// again. So a fresh replica collected nothing for its first hour no matter what schedule it
	// had been given — measured on a cluster with `*/15 * * * *`, where the first collection
	// could not happen before the hour was up — and nothing in the log said so.
	Known bool

	// Enabled is whether to collect at all.
	Enabled bool

	// Schedule is a five-field cron expression.
	Schedule string

	// FillPending is whether a fill is owed: the cluster has asked for one and this replica does
	// not hold the whole set yet.
	//
	// A reason to collect nothing at all, and the strongest one there is. Collecting reclaims what
	// no kept release declares, and an incomplete store is precisely one whose contents cannot yet
	// be compared with anything: the fill is still putting the set in. Worse, the reclamation makes
	// the registry refuse writes, and a fill IS writes — so a collection on an incomplete store
	// competes with the one activity that has to finish first.
	//
	// Distinct from Filling, which is about this instant: a pass in flight. This is about the state
	// of the store, and it stays true between passes — which is exactly the window a
	// pass-in-flight check leaves open, and the loop sleeps thirty seconds in it.
	FillPending bool
}

// How long to wait before looking again when the instruction is not known yet. Short, because
// this is a state that lasts seconds at startup: the loop stores the desired state on its first
// pass, before doing any of the work that can fail.
const unknownPlanRetry = time.Minute

// fillingRetry is how long the collection waits out a fill.
//
// Short, and deliberately not "until the next slot in the schedule": a fill of a whole release set
// takes longer than the interval an operator is likely to write, so charging it a full period per
// attempt would postpone collection indefinitely on exactly the cluster that needs it — one that
// keeps filling.
const fillingRetry = time.Minute

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

	// Collect does the work.
	Collect func(ctx context.Context) (Report, error)

	// Pushing reports whether somebody is pushing into this store from outside right now. Nil means
	// never.
	//
	// The writer the syncer cannot take turns with. Inside the pod it is the only writer, so a fill
	// and a collection serialise themselves; the publication endpoint is reachable from outside, and
	// what comes through it is an operator pushing a bundle. Reclaiming blobs underneath such a push
	// deletes what it has uploaded and not yet referenced.
	//
	// Asked about activity, not about configuration: the endpoint may stay open for the life of a
	// cluster while pushes happen twice a year, so refusing to collect while it is open would mean
	// never collecting.
	Pushing func() bool

	// Filling reports whether this replica is filling its store right now. Nil means never.
	//
	// Collecting makes the registry refuse writes for the duration, and a fill IS writes: firing
	// while one is in flight does not slow it down, it fails it, reference by reference, and the
	// replica then reports a partial store it has to redo. Filling a cache is what the cache is
	// for and reclaiming is housekeeping, so the order between them is not a matter of taste —
	// the housekeeping waits.
	Filling func() bool

	// Publish records the outcome, whatever it was. Called even for a failure: "it has not
	// collected since Tuesday" is the useful fact, and it is invisible if only successes
	// are recorded.
	Publish func(ctx context.Context, finished time.Time, err error)

	// Window bounds a firing. Zero means DefaultWindow.
	Window time.Duration

	// Now is the clock, so a test does not have to wait for 3am.
	Now func() time.Time

	// Sleep is the waiting, so a test can see how long was waited instead of waiting it.
	// Returns false when the context ended, exactly as the package function does.
	//
	// Injectable because the length of a wait is behaviour here and not an implementation
	// detail: waiting an hour where a minute was meant is the whole of the defect this
	// distinguishes, and it is invisible to a test that cannot observe the duration.
	Sleep func(ctx context.Context, d time.Duration) bool
}

func (s *Scheduler) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// pushing reports whether a push from outside is in flight, defaulting to "no" when nobody said.
func (s *Scheduler) pushing() bool {
	return s.Pushing != nil && s.Pushing()
}

// filling reports whether a fill is in flight, defaulting to "no" when nobody said.
func (s *Scheduler) filling() bool {
	return s.Filling != nil && s.Filling()
}

func (s *Scheduler) sleep(ctx context.Context, d time.Duration) bool {
	if s.Sleep == nil {
		return sleep(ctx, d)
	}
	return s.Sleep(ctx, d)
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

		if !plan.Known {
			// Not read yet, which is not the same as turned off and must not be charged the
			// same hour of waiting. Nothing is logged: at startup this is the ordinary state
			// for a second or two, and a line every minute would say nothing.
			if !s.sleep(ctx, unknownPlanRetry) {
				return
			}
			continue
		}

		if !plan.Enabled {
			// Disabled, which is a configuration and not a problem. Re-read on a timer
			// rather than watched, because turning it on is not urgent.
			//
			// Logged, unlike before. A store that is never reclaimed has exactly one visible
			// symptom — an alert an hour later about a collection that has not happened — and
			// the process that decided not to collect should be the one that says why.
			s.Log.Info("the garbage collection is turned off in the configuration, so nothing is collected")
			if !s.sleep(ctx, time.Hour) {
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
			if !s.sleep(ctx, time.Hour) {
				return
			}
			continue
		}

		now := s.now()
		next := schedule.Next(now)
		wait := next.Sub(now)

		s.Log.Info("the next garbage collection is scheduled", "at", next.Format(time.RFC3339))
		if !s.sleep(ctx, wait) {
			return
		}

		// Asked here rather than before the wait, because the wait is where a fill usually
		// starts: the answer that matters is the one at the moment of firing.
		//
		// Two questions, not one. `filling` is whether a pass is in flight; `FillPending` is
		// whether the store is owed a fill at all — and the second is the one that holds between
		// passes, where the first says "no" for thirty seconds at a time on a store that is far
		// from complete.
		if plan.FillPending || s.filling() || s.pushing() {
			s.Log.Info("the store is being written to, so the collection waits",
				"fill_in_flight", s.filling(), "fill_pending", plan.FillPending,
				"push_in_flight", s.pushing(), "retry_in", fillingRetry.String())
			if !s.sleep(ctx, fillingRetry) {
				return
			}
			continue
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
// collect runs one collection.
//
// It no longer makes the registry read-only around the whole of it, and that is a fix rather than a
// relaxation. Read-only is not a property the deciding needs — it is what blob reclamation needs, so
// that a push cannot slip in between "these blobs are unreferenced" and "these blobs are gone". It is
// gone entirely: inside the storage pod the only writer to the cache is the syncer itself, so it
// simply stops writing while it collects — see Loop.PauseWrites. Serving is never interrupted.
//
// What it cost to take it here: applying read-only means restarting the serving process, and the
// kubelet counts that as the container crashing. Twice per collection, every fifteen minutes,
// whether or not anything was deletable — measured on a cluster as seven restarts per replica,
// exponential backoff, and a store that answered `connection refused` for minutes at a time. In that
// state the followers could not replicate, the leader's own accounting could not read the store, and
// the collection failed on the read it had itself made impossible.
func (s *Scheduler) collect(ctx context.Context) error {
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
