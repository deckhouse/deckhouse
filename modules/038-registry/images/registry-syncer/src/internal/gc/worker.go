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
	"sync"
	"sync/atomic"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// DefaultWindow bounds one wait for this replica's turn and the collection after it.
//
// Replicas collect one at a time, so on a three-master cluster the third waits for two collections
// before its own. The bound is what keeps a replica that cannot finish from holding the others out
// forever: its run is abandoned, and a replica that closed its window waiting simply waits again.
const DefaultWindow = 4 * time.Hour

// Plan is what this replica's state says about collecting now.
type Plan struct {
	// Known is whether the desired state has been read at all yet.
	//
	// Separate from FillPending because an empty plan reads as "nothing pending", and a replica that
	// has not read its instruction yet knows nothing about its store.
	Known bool

	// FillPending is whether a fill is owed: the cluster has asked for one and this replica does
	// not hold the whole set yet.
	//
	// A reason to collect nothing at all, and the strongest one there is. Collecting reclaims what
	// no kept release declares, and an incomplete store is precisely one whose contents cannot yet
	// be compared with anything: the fill is still putting the set in.
	//
	// Distinct from Filling, which is about this instant: a pass in flight. This is about the state
	// of the store, and it stays true between passes — which is exactly the window a
	// pass-in-flight check leaves open, and the loop sleeps thirty seconds in it.
	FillPending bool
}

// How long to wait before looking again when the instruction is not known yet. Short, because this is
// a state that lasts seconds at startup: the loop stores the desired state on its first pass, before
// doing any of the work that can fail.
const unknownPlanRetry = time.Minute

// operationPoll is how often a replica looks for an operation, how often one it is held back from is
// tried again, and how often one it is running is read again for a cancellation. A person may have
// pressed a button and be looking at the result, so seconds matter; and what is read is a handful of
// small objects.
//
// A variable so a test can see a cancellation reach a running collection without waiting for it.
var operationPoll = 15 * time.Second

// Lease serialises collection across replicas.
type Lease interface {
	// Hold blocks until this replica may collect, then calls work, then gives the lease
	// up. Returns whatever work returned, or an error if the lease was never obtained.
	Hold(ctx context.Context, work func(context.Context) error) error
}

// Operation is a RegistryOperation, as this replica sees it.
type Operation struct {
	// Name of the RegistryOperation.
	Name string

	// Type is what the operation does. Empty reads as a garbage collection.
	Type registryv1alpha1.RegistryOperationType

	// Digests are what a Verify or a Repair is about. Empty means the whole store.
	Digests []string

	// Deadline is when a replica still held back by a store that is being written to gives up. Zero
	// means none.
	Deadline time.Time

	// Canceled is whether the operation was canceled.
	Canceled bool

	// PassedGate is whether this replica got past waiting for its store before — it waited for its
	// turn or collected — as its entry in the status says. The deadline is for the store to stop being
	// written to, and does not apply again to a replica that has seen that happen: not after a window
	// closed on its turn, nor after a restart.
	PassedGate bool
}

// Progress is this replica's part of an operation, as it is written into the operation's status.
type Progress struct {
	Phase   registryv1alpha1.ReplicaOperationPhase
	Reason  string
	Message string

	StartedAt  *time.Time
	FinishedAt *time.Time

	// Report is what the collection did. Nil until it has run.
	Report *Report

	// Verify and Repair are what a Verify or a Repair found and did. Nil until it has run.
	Verify *registryv1alpha1.VerifyResult
	Repair *registryv1alpha1.RepairResult
}

// collects reports whether the operation is a garbage collection: the one that deletes, and so the one
// held back by a store being written to and run with this replica's writes paused.
func (o *Operation) collects() bool {
	return o.Type == "" || o.Type == registryv1alpha1.OperationGarbageCollection
}

// what names the operation in a message.
func (o *Operation) what() string {
	switch o.Type {
	case registryv1alpha1.OperationVerify:
		return "check"
	case registryv1alpha1.OperationRepair:
		return "repair"
	}
	return "collection"
}

// ErrNotOwed is what Report says about an operation this replica no longer owes: it is gone, it
// finished, or the controller settled this replica's entry. Nothing is written then — a replica that
// reports late does not reopen an entry — and whoever reported must not go on as if it had been.
var ErrNotOwed = errors.New("the operation is gone or no longer owed by this replica")

// Operations are the operations this replica is asked to run.
type Operations interface {
	// Next returns the oldest operation this replica still owes, or nil.
	Next(ctx context.Context) (*Operation, error)

	// Get reads an operation again. Nil when it is gone, or when this replica no longer owes it — the
	// controller settled its entry, or the operation finished.
	Get(ctx context.Context, name string) (*Operation, error)

	// Report writes this replica's progress on an operation. ErrNotOwed when the operation is gone or
	// no longer owed: there is nothing left to report to.
	Report(ctx context.Context, name string, progress Progress) error
}

// Worker collects this replica's store whenever a RegistryOperation asks for it.
//
// It decides nothing about when: the controller creates the operations, on the schedule, after a new
// image set, or because somebody asked. What is decided here is whether this store may be collected
// right now, and that is the same answer whoever asked.
type Worker struct {
	Log *slog.Logger

	// Plan reads this replica's state.
	Plan func() Plan

	// Operations are where the work comes from and where its progress goes.
	Operations Operations

	// Lease serialises collection across replicas, so that at most one collects at any moment.
	Lease Lease

	// Collect does the work of a garbage collection.
	Collect func(ctx context.Context) (Report, error)

	// Verify does the work of a Verify, and Repair of a Repair. Nil refuses the operation on this
	// replica.
	Verify func(ctx context.Context, digests []string) (registryv1alpha1.VerifyResult, error)
	Repair func(ctx context.Context, digests []string) (registryv1alpha1.RepairResult, error)

	// Done is told when an operation that changed or read the store has finished on this replica,
	// whatever its outcome. Optional.
	Done func(operation *Operation)

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
	// A fill and a collection must not overlap: the collection reclaims what no manifest references,
	// and a fill writes blobs before the manifests that reference them. Filling a cache is what the
	// cache is for and reclaiming is housekeeping, so the housekeeping waits.
	Filling func() bool

	// Pause takes the store from this replica's own writers if it can right now, and says whether it
	// could. Nil means there are none.
	//
	// Asked, never waited on: a fill holds the store for its whole pass, and a replica that waited for
	// one while holding the lease would hold every other replica out with it, and could not be canceled
	// while it waited.
	Pause func() (release func(), ok bool)

	// Publish records the outcome of a collection, whatever it was. Called even for a failure: "it
	// has not collected since Tuesday" is the useful fact, and it is invisible if only successes are
	// recorded.
	Publish func(ctx context.Context, finished time.Time, err error)

	// Window bounds one wait for the turn and the collection after it. Zero means DefaultWindow.
	Window time.Duration

	// Now is the clock.
	Now func() time.Time

	// Sleep is the waiting, so a test can see how long was waited instead of waiting it.
	// Returns false when the context ended, exactly as the package function does.
	Sleep func(ctx context.Context, d time.Duration) bool

	// operationsErr is the last failure to read the operations, so the same one is logged once.
	operationsErr string
}

func (w *Worker) now() time.Time {
	if w.Now == nil {
		return time.Now()
	}
	return w.Now()
}

// pushing reports whether a push from outside is in flight, defaulting to "no" when nobody said.
func (w *Worker) pushing() bool {
	return w.Pushing != nil && w.Pushing()
}

// filling reports whether a fill is in flight, defaulting to "no" when nobody said.
func (w *Worker) filling() bool {
	return w.Filling != nil && w.Filling()
}

// quiet takes the store for a collection if nothing writes to it now: this replica's own writers are
// paused, and neither a fill nor a push from outside is under way. The release is nil when it is not.
func (w *Worker) quiet() (func(), bool) {
	release := func() {}
	if w.Pause != nil {
		paused, ok := w.Pause()
		if !ok {
			return nil, false
		}
		release = paused
	}
	// Asked with the pause held, so a fill cannot start in between: one that is waiting for the pause
	// has already said it is filling, and goes first.
	if w.Plan().FillPending || w.filling() || w.pushing() {
		release()
		return nil, false
	}
	return release, true
}

func (w *Worker) sleep(ctx context.Context, d time.Duration) bool {
	if w.Sleep == nil {
		return sleep(ctx, d)
	}
	return w.Sleep(ctx, d)
}

func (w *Worker) window() time.Duration {
	if w.Window <= 0 {
		return DefaultWindow
	}
	return w.Window
}

// Run serves operations until the context ends.
//
// Never returns an error. A replica that cannot collect still serves every image it holds, so there is
// nothing here worth taking the process down for — and taking it down would stop the filling and
// reporting that share it.
func (w *Worker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if operation := w.next(ctx); operation != nil {
			w.serve(ctx, operation)
			continue
		}
		if !w.sleep(ctx, operationPoll) {
			return
		}
	}
}

// next asks for an operation, and treats a failure to ask as "none" until the next look.
func (w *Worker) next(ctx context.Context) *Operation {
	operation, err := w.Operations.Next(ctx)
	if err != nil {
		// Said once per distinct failure rather than at every look: an API server that is away for
		// an hour would otherwise fill the log with the same line four times a minute.
		if message := err.Error(); message != w.operationsErr {
			w.operationsErr = message
			w.Log.Warn("cannot read the registry operations, so nothing is collected until they can be read",
				"error", message)
		}
		return nil
	}
	w.operationsErr = ""
	return operation
}

// serving is one operation as this replica works on it.
type serving struct {
	worker *Worker
	log    *slog.Logger
	name   string

	// written is the progress last written, so the same one is not written again: the controller
	// watches the object and every write wakes it.
	written *Progress
}

// report writes progress that is not final. A failure is logged and left: the next one carries the
// state again.
func (r *serving) report(ctx context.Context, progress Progress) {
	if r.written != nil && sameProgress(*r.written, progress) {
		return
	}
	if err := r.worker.Operations.Report(ctx, r.name, progress); err != nil {
		if !errors.Is(err, ErrNotOwed) {
			r.log.Warn("cannot record the progress of the operation", "error", err.Error())
		}
		return
	}
	r.written = &progress
}

// start writes that this replica runs the operation, and reports whether it still owes it. A write that
// fails for another reason does not stop it: the next write carries the state again, as with report.
func (r *serving) start(ctx context.Context, progress Progress) bool {
	err := r.worker.Operations.Report(ctx, r.name, progress)
	switch {
	case errors.Is(err, ErrNotOwed):
		return false
	case err != nil:
		r.log.Warn("cannot record the progress of the operation", "error", err.Error())
	default:
		r.written = &progress
	}
	return true
}

// settle writes the final progress, and keeps at it until it is written. A final state that is not
// written leaves the operation owed, and an operation still owed is collected again.
func (r *serving) settle(ctx context.Context, progress Progress) {
	for {
		err := r.worker.Operations.Report(ctx, r.name, progress)
		if errors.Is(err, ErrNotOwed) {
			r.log.Info("the operation was settled on this replica by somebody else",
				"phase", progress.Phase, "reason", progress.Reason)
			return
		}
		if err == nil {
			r.log.Info("the operation is settled on this replica",
				"phase", progress.Phase, "reason", progress.Reason)
			return
		}
		r.log.Warn("cannot record the outcome of the operation", "error", err.Error())
		if !r.worker.sleep(ctx, operationPoll) {
			return
		}
	}
}

func sameProgress(a, b Progress) bool {
	return a.Phase == b.Phase && a.Reason == b.Reason && a.Message == b.Message &&
		sameTime(a.StartedAt, b.StartedAt) && sameTime(a.FinishedAt, b.FinishedAt) &&
		a.Report == nil && b.Report == nil && a.Verify == nil && b.Verify == nil && a.Repair == nil && b.Repair == nil
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// serve works on one operation until this replica has settled it.
func (w *Worker) serve(ctx context.Context, operation *Operation) {
	run := &serving{worker: w, log: w.Log.With("operation", operation.Name, "type", operation.Type), name: operation.Name}
	run.log.Info("an operation is asked for")
	passedGate := operation.PassedGate

	for ctx.Err() == nil {
		current, err := w.Operations.Get(ctx, operation.Name)
		if err != nil {
			run.log.Warn("cannot read the operation again", "error", err.Error())
			if !w.sleep(ctx, operationPoll) {
				return
			}
			continue
		}
		if current == nil {
			run.log.Info("the operation is gone or no longer owed by this replica")
			return
		}
		passedGate = passedGate || current.PassedGate

		if current.Canceled {
			now := w.now()
			run.settle(ctx, Progress{
				Phase:      registryv1alpha1.ReplicaOperationCanceled,
				Reason:     registryv1alpha1.ReasonOperationCanceled,
				Message:    "The operation was canceled before this replica started.",
				FinishedAt: &now,
			})
			return
		}

		plan := w.Plan()
		if !plan.Known {
			// Seconds after a start, and nothing to report about: the store has not been looked at.
			if !w.sleep(ctx, unknownPlanRetry) {
				return
			}
			continue
		}

		// The set cannot be compared with a store it is still going into, and a push from outside
		// cannot be swept from under. Only a collection waits for that: a check reads, and a repair
		// replaces a blob with the same content, renamed into place — a fill or a push beside it loses
		// nothing. And a replica with a broken blob is not complete, so its fill is owed for as long as
		// the repair has not run: a repair that waited for the fill would wait for itself. A collection
		// would too — the Repair is not even asked for while it is active — which is why Plan does not
		// call a fill owed that only a Repair can finish.
		var reason, message string
		switch {
		case !current.collects():
		case plan.FillPending || w.filling():
			reason, message = registryv1alpha1.ReasonOperationFilling,
				"The store is being filled; the collection starts when the fill is done."
		case w.pushing():
			reason, message = registryv1alpha1.ReasonOperationPushing,
				"An image set is being pushed into the store; the collection starts when the push is done."
		}
		if reason != "" {
			if !passedGate && !current.Deadline.IsZero() && !w.now().Before(current.Deadline) {
				now := w.now()
				run.settle(ctx, Progress{
					Phase:      registryv1alpha1.ReplicaOperationFailed,
					Reason:     registryv1alpha1.ReasonOperationDeadlineExceeded,
					Message:    fmt.Sprintf("The store was still being written to at the deadline (%s).", reason),
					FinishedAt: &now,
				})
				return
			}
			if passedGate {
				// Still its turn it waits for, and said so: the entry is what tells a restarted syncer
				// that this replica is past the deadline's reach. Waiting/Filling there would not.
				message = fmt.Sprintf("This replica waits for the store to go quiet (%s), then for its turn again.", reason)
				reason = registryv1alpha1.ReasonOperationTurn
			}
			run.report(ctx, Progress{Phase: registryv1alpha1.ReplicaOperationWaiting, Reason: reason, Message: message})
			if !w.sleep(ctx, operationPoll) {
				return
			}
			continue
		}

		passedGate = true
		if w.fire(ctx, run, current) {
			return
		}
	}
}

// fire runs the collection once this replica's turn comes. Returns whether the operation is settled on
// this replica; false means the window closed before the turn came, and it is waited for again.
//
// The wait for the turn and the collection are bounded apart, each by the window. One bound over both
// left the last of three replicas what the first two had not used of it — twenty minutes, on a store
// that takes an hour — and killed its collection as a failure.
func (w *Worker) fire(parent context.Context, run *serving, operation *Operation) bool {
	run.report(parent, Progress{
		Phase:   registryv1alpha1.ReplicaOperationWaiting,
		Reason:  registryv1alpha1.ReasonOperationTurn,
		Message: "Another replica is running an operation; this one goes next.",
	})

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Where this turn is: waiting for the lease, collecting, given up on waiting, or given up because
	// the operation is gone. One value, so the window closing, the operation going and the turn coming
	// cannot both win.
	const (
		turnWaiting int32 = iota
		turnRunning
		turnWaitedOut
		turnGone
		turnBusy
	)
	var turn atomic.Int32
	waitBound := time.AfterFunc(w.window(), func() {
		if turn.CompareAndSwap(turnWaiting, turnWaitedOut) {
			cancel()
		}
	})
	defer waitBound.Stop()

	// A cancellation has to reach a collection that is already running, so the operation is read
	// again while it runs. Timed by the real clock rather than the injected one: a test that makes
	// every wait instant would turn this into a loop reading the operation as fast as it can.
	//
	// An operation gone — deleted, or settled for this replica by the controller — stops a replica
	// still waiting for its turn and not one that has started: that is what deleting it promises.
	var canceled atomic.Bool
	watching := make(chan struct{})
	go func() {
		defer close(watching)
		for sleep(ctx, operationPoll) {
			current, err := w.Operations.Get(ctx, run.name)
			switch {
			case err != nil:
			case current != nil && current.Canceled:
				canceled.Store(true)
				cancel()
				return
			case current == nil && turn.CompareAndSwap(turnWaiting, turnGone):
				// Only a turn not taken yet; a collection that has started goes on, and is still
				// watched for a cancellation.
				cancel()
				return
			}
		}
	}()

	var (
		mu         sync.Mutex
		started    *time.Time
		report     performed
		collectErr error
	)
	err := w.Lease.Hold(ctx, func(leaseCtx context.Context) error {
		if !turn.CompareAndSwap(turnWaiting, turnRunning) {
			return leaseCtx.Err()
		}

		// The store is looked at again with the turn in hand. What was checked before the wait is
		// hours old on the third of three replicas, and a push started meanwhile has its layers on
		// disk and no manifest yet: exactly what a sweep removes. A store being written to gives the
		// turn back, and the replica waits for it to go quiet as it did before. Only for a
		// collection — see serve.
		if operation.collects() {
			release, quiet := w.quiet()
			if !quiet {
				turn.Store(turnBusy)
				return nil
			}
			defer release()
		}

		collectCtx, stop := context.WithTimeout(leaseCtx, w.window())
		defer stop()

		now := w.now()
		// Running is said only by a replica that still owes the operation: one canceled or given up
		// on while it waited for the turn does not start on it.
		if !run.start(parent, Progress{Phase: registryv1alpha1.ReplicaOperationRunning, StartedAt: &now}) {
			turn.Store(turnGone)
			return nil
		}
		mu.Lock()
		started = &now
		mu.Unlock()

		got, gotErr := w.perform(collectCtx, operation)
		mu.Lock()
		report, collectErr = got, gotErr
		mu.Unlock()
		return gotErr
	})
	cancel()
	<-watching

	mu.Lock()
	defer mu.Unlock()

	finished := w.now()
	// What Hold says, when it says more: a lease lost mid-run is the cause, not the cancellation the
	// collection saw because of it.
	cause := collectErr
	if collectErr != nil && err != nil {
		cause = err
	}
	if operation.collects() {
		w.publish(parent, started, canceled.Load() && collectErr != nil, finished, cause)
	}
	if started != nil && w.Done != nil {
		w.Done(operation)
	}

	outcome := Progress{StartedAt: started, FinishedAt: &finished}
	if started != nil {
		outcome.Report, outcome.Verify, outcome.Repair = report.collected, report.verify, report.repair
	}

	switch {
	case started != nil && collectErr == nil:
		// Finished whole before a cancellation reached it: what it did is what happened.
		outcome.Phase, outcome.Reason = registryv1alpha1.ReplicaOperationSucceeded, ""
		outcome.Message = report.message
	case canceled.Load():
		outcome.Phase, outcome.Reason = registryv1alpha1.ReplicaOperationCanceled, registryv1alpha1.ReasonOperationCanceled
		outcome.Message = "The operation was canceled while this replica waited for its turn."
		if started != nil {
			outcome.Message = fmt.Sprintf("The operation was canceled during the %s on this replica; what it had done stays done.",
				operation.what())
		}
	case turn.Load() == turnGone:
		run.log.Info("the operation is gone or no longer owed by this replica, so it does not take its turn")
		return true
	case turn.Load() == turnBusy:
		run.log.Info("the store is being written to, so this replica gives its turn back and waits for it to go quiet")
		return !w.sleep(parent, operationPoll)
	case started != nil && collectErr != nil:
		outcome.Phase, outcome.Reason = registryv1alpha1.ReplicaOperationFailed, registryv1alpha1.ReasonOperationFailed
		outcome.Message = cause.Error()
	case parent.Err() != nil:
		return true
	case turn.Load() == turnWaitedOut:
		run.log.Info("the collection window closed before this replica's turn came, so it waits again")
		return false
	default:
		run.log.Error("cannot take the turn to collect", "error", fmt.Sprint(err))
		return !w.sleep(parent, operationPoll)
	}

	run.settle(parent, outcome)
	return true
}

// publishTimeout bounds recording the outcome of a collection in RegistryStorage.
const publishTimeout = 30 * time.Second

// publish records a collection that ran in RegistryStorage — collectedAt, collectionError, the metrics
// — on a context of its own: the one the collection ran on may have ended with it, at the window or
// with the lease, and a write on it would leave the replica reporting a collection it did not finish.
// Not for one stopped by a cancellation, nor by this process stopping: it did not fail, somebody or
// something stopped it, and recording it as the replica's last collection error would read as a broken
// store — every rollout would.
func (w *Worker) publish(parent context.Context, started *time.Time, canceled bool, finished time.Time, err error) {
	if w.Publish == nil || started == nil || canceled || parent.Err() != nil {
		return
	}
	ctx, stop := context.WithTimeout(context.WithoutCancel(parent), publishTimeout)
	defer stop()
	w.Publish(ctx, finished, err)
}

// removedMessage says what a collection removed, in the terms a person reads a store in: releases,
// images by digest, and the blobs that freed the disk.
func removedMessage(report Report) string {
	message := fmt.Sprintf("Removed %d releases and %d images by digest", len(report.RemovedVersions), report.RemovedManifests)
	if report.RemovedBlobs >= 0 {
		message += fmt.Sprintf("; the sweep removed %d blobs", report.RemovedBlobs)
	}
	return message + "."
}

// performed is what an operation did on this replica, by its type.
type performed struct {
	collected *Report
	verify    *registryv1alpha1.VerifyResult
	repair    *registryv1alpha1.RepairResult
	message   string
}

// perform runs the operation's work.
func (w *Worker) perform(ctx context.Context, operation *Operation) (performed, error) {
	switch operation.Type {
	case registryv1alpha1.OperationVerify:
		if w.Verify == nil {
			return performed{}, errors.New("this replica does not run checks")
		}
		result, err := w.Verify(ctx, operation.Digests)
		return performed{verify: &result, message: verifiedMessage(result)}, err
	case registryv1alpha1.OperationRepair:
		if w.Repair == nil {
			return performed{}, errors.New("this replica does not run repairs")
		}
		result, err := w.Repair(ctx, operation.Digests)
		if err == nil {
			err = repairFailure(result)
		}
		return performed{repair: &result, message: repairedMessage(result)}, err
	}
	report, err := w.collect(ctx)
	return performed{collected: &report, message: removedMessage(report)}, err
}

// verifiedMessage says what a check found.
func verifiedMessage(result registryv1alpha1.VerifyResult) string {
	message := fmt.Sprintf("Checked %d blobs, %d MiB", result.CheckedBlobs, result.CheckedBytes>>20)
	if result.CorruptedCount > 0 {
		message += fmt.Sprintf("; %d do not match their digests and are to be repaired", result.CorruptedCount)
	}
	if len(result.MissingDigests) > 0 {
		message += fmt.Sprintf("; %d of the digests asked about are not held", len(result.MissingDigests))
	}
	return message + "."
}

// repairedMessage says what a repair did, digest by digest in the result and summed here.
func repairedMessage(result registryv1alpha1.RepairResult) string {
	counts := map[registryv1alpha1.DigestRepairOutcome]int{}
	for _, entry := range result.Digests {
		counts[entry.Outcome]++
	}
	return fmt.Sprintf("Repaired %d, intact %d, not held %d, failed %d.",
		counts[registryv1alpha1.DigestRepaired], counts[registryv1alpha1.DigestIntact],
		counts[registryv1alpha1.DigestNotHeld], counts[registryv1alpha1.DigestFailed])
}

// repairFailure fails a repair on this replica when a digest could not be repaired: the operation is
// then Failed, which is what an operator and the controller's retry look for.
func repairFailure(result registryv1alpha1.RepairResult) error {
	failed := 0
	for _, entry := range result.Digests {
		if entry.Outcome == registryv1alpha1.DigestFailed {
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d digests could not be repaired; see repair.digests", failed, len(result.Digests))
}

// collect runs one collection.
//
// It does not make the registry read-only around it, and that is a fix rather than a relaxation.
// Read-only is not a property the deciding needs — it is what blob reclamation needs, so that a push
// cannot slip in between "these blobs are unreferenced" and "these blobs are gone". Inside the storage
// pod the syncer is the writer that matters, so it simply stops writing while it collects — see
// Loop.PauseWrites. Serving is never interrupted.
//
// What read-only used to cost: applying it means restarting the serving process, and the kubelet
// counts that as the container crashing — twice per collection, whether or not anything was
// deletable. The restarts accumulate into kubelet backoff, and a store in that state answers
// `connection refused` for minutes at a time.
func (w *Worker) collect(ctx context.Context) (Report, error) {
	report, collectErr := w.Collect(ctx)
	if collectErr != nil {
		return report, collectErr
	}

	w.Log.Info("the garbage collection finished",
		"considered", report.Considered, "deleted", len(report.Deleted),
		"failed", len(report.Failed), "kept", report.Kept, "swept", report.Swept)
	return report, nil
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
