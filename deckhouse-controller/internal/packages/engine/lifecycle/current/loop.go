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

package current

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// loop is the loop goroutine; on exit it cancels the operation, stops the hooks and writes nothing.
func (r *Reconciler) loop(ctx context.Context) {
	defer close(r.done)
	defer r.exit()

	// wake is the one timer: it fires at the earliest retry deadline.
	wake := r.clock.NewTimer(time.Hour)
	wake.Stop()

	defer wake.Stop()

	for {
		// The one exit on cancellation, also for the cancel the select below saw: nothing is
		// accepted, published or started on a dead context.
		if ctx.Err() != nil {
			// An uninstall may have arrived after the last load.
			if r.in.Load().uninstall {
				r.uninstallErr = ctx.Err() // removal interrupted, not done
			}

			return
		}

		in := r.in.Load()
		now := r.clock.Now()
		ownInputs := r.ownInputs(in)

		want := next(r.state, in, ownInputs, now)

		// Accept first, synchronously, then decide again: an accepted configuration changes the own
		// inputs, so the scheduler never computes over the own inputs it replaces.
		if want.kind == actionAccept && r.operation == nil {
			r.accept(ctx, in.config, in.configChecksum)
			continue
		}

		r.publishFacts(ownInputs)
		r.reportDecision(in, ownInputs)

		if r.operation != nil && supersedes(want, r.operation) {
			r.operation.Cancel()
		}

		if r.operation == nil {
			r.act(ctx, want)
		}

		retry.Arm(wake, r.clock, retry.Earliest(now, r.state.operations, r.state.accept))

		select {
		case <-ctx.Done():
			// leaves at the top of the loop
		case <-r.in.Ready():
		case res := <-r.operation.Done():
			if r.finishOperation(res) {
				return // uninstalled
			}
		case <-wake.C():
		}
	}
}

// act starts the operation want decides while the worker is idle; the loop accepts by itself.
func (r *Reconciler) act(ctx context.Context, want action) {
	switch want.kind {
	case actionRun:
		r.startOperation(ctx, operation.Run, want.inputs, r.runSteps)
	case actionDisable:
		r.startOperation(ctx, operation.Disable, want.inputs, r.disableSteps)
	case actionUninstall:
		r.startOperation(ctx, operation.Uninstall, "", r.uninstallSteps)
	case actionWait, actionAccept:
	}
}

// startOperation ends the disabled state, after the steps are built: uninstallSteps reads it. An
// uninstall keeps it, so its retries prepare the hooks as its first attempt did. A run pauses the
// drift monitor: its hooks may delete release resources, which is not drift.
func (r *Reconciler) startOperation(ctx context.Context, kind operation.Kind, inputs string, steps func(*operationState) []operation.Step) {
	st := &operationState{accepted: r.state.settings.Accepted, hooksStop: r.hooksStop}

	if kind == operation.Run && r.deps.Nelm.HasMonitor(r.name) {
		r.deps.Nelm.PauseMonitor(r.name)
		st.monitorPaused = true
	}

	st.steps = steps(st)

	if kind != operation.Uninstall {
		r.state.disabled = false
	}

	r.state.busy = true
	r.operationState = st
	r.operation = operation.Start(ctx, r.name, kind, inputs, st.steps)
}

func (r *Reconciler) collectOperation() *operationState {
	st := r.operationState
	r.operation, r.operationState = nil, nil
	r.state.busy = false
	r.hooksStop = st.hooksStop

	if st.monitorPaused {
		r.deps.Nelm.ResumeMonitor(r.name)
	}

	return st
}

// finishOperation reports the worker's result; true once the uninstall succeeded. A cancelled
// operation is not a failure: next decides again.
func (r *Reconciler) finishOperation(res operation.Result) bool {
	st := r.collectOperation()

	if res.Failed != nil {
		// A failure clears applied, so the node runs again even when the Inputs come back to
		// those of the last success. Uninstall keeps applied: its node has already left the
		// graph, and it is retried until it succeeds.
		if res.Kind != operation.Uninstall {
			r.state.applied = ""
		}

		if !res.Canceled {
			r.reportFailure(res, st.steps)

			now := r.clock.Now()
			until := r.state.operations.Failed(operationKey(res.Kind, res.Key), now)

			r.logger.Warn("operation failed", slog.String("operation", string(res.Kind)),
				slog.String("step", res.Failed.Name), slog.Duration("retry_in", until.Sub(now)), log.Err(res.Err))
		}

		return false
	}

	r.logger.Debug("operation succeeded", slog.String("operation", string(res.Kind)))

	r.state.operations.Succeeded()

	switch res.Kind {
	case operation.Run:
		r.state.applied = res.Key
		r.state.ran = true
		r.reportSuccess(operation.Run, st.steps, st.accepted)
	case operation.Disable:
		r.state.disabled = true
		r.state.applied = res.Key
		r.reportSuccess(operation.Disable, st.steps, r.state.settings.Accepted)
	case operation.Uninstall:
		return true
	}

	return false
}

func (r *Reconciler) exit() {
	if r.operation != nil {
		r.operation.Cancel()
		<-r.operation.Done()
		r.collectOperation()
	}

	if r.hooksStop != nil {
		r.hooksStop()
		r.hooksStop = nil
	}
}

// ownInputs is Facts.OwnInputs: the hash of the revision's identity, accepted configuration, values
// and drift generations.
func (r *Reconciler) ownInputs(in input) string {
	sum := ""
	if a := r.state.settings.Accepted; a != nil {
		sum = a.Checksum
	}

	h := sha256.Sum256(fmt.Appendf(nil, "version=%s\nconfiguration=%s\nvalues=%d\ndrift=%d\n",
		r.state.version, sum, in.valuesGen, in.driftGen))

	return hex.EncodeToString(h[:])
}

// publishFacts runs on every iteration: the scheduler drops equal facts, so Current keeps no
// record of what it published.
func (r *Reconciler) publishFacts(ownInputs string) {
	r.deps.Scheduler.Publish(r.name, schedule.Facts{
		Version:   r.state.version,
		OwnInputs: ownInputs,
		Applied:   r.state.applied,
		Output:    r.output,
	})
}

// OutputOf is the checksum of obj's image digests, which Global publishes for every render; empty
// for an object without digests. It changes only with a revision that ships other images.
func OutputOf(obj model.Package) string {
	d, ok := obj.(digested)
	if !ok || len(d.GetDigests()) == 0 {
		return ""
	}

	// json.Marshal sorts map keys, so equal digests hash equally.
	data, _ := json.Marshal(d.GetDigests())
	h := sha256.Sum256(data)

	return hex.EncodeToString(h[:])
}

// reportDecision reports the decision next would act on (ours): one computed before the accept
// would flip status.enabled.
func (r *Reconciler) reportDecision(in input, ownInputs string) {
	ev := in.event
	if !ours(r.state, ev, ownInputs) {
		return
	}

	d := model.Decision{Enabled: ev.Enabled, Reason: ev.Reason, Message: ev.Message}

	r.reporter.UpdateCurrent(func(r *model.CurrentReport) { r.Decision = &d })
}
