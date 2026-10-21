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

// The loop, as Current's (current/loop.go).

package global

import (
	"context"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// loop is the loop goroutine. On exit it cancels and waits for the operation in progress and stops
// the global hook lanes, writing nothing.
func (g *Global) loop(ctx context.Context) {
	defer close(g.done)
	defer g.exit()

	// wake is the one timer: it fires at the earliest retry deadline.
	wake := g.clock.NewTimer(time.Hour)
	wake.Stop()

	defer wake.Stop()

	for {
		// The one exit on cancellation, also for the cancel the select below saw: nothing is
		// accepted, published or started on a dead context.
		if ctx.Err() != nil {
			return
		}

		in := g.in.Load()
		now := g.clock.Now()
		ownInputs := g.ownInputs(in)

		want := next(g.state, in, ownInputs, now)

		// Accept first, synchronously, then decide again: accepted settings change the own inputs,
		// so the scheduler never computes over the own inputs they replace.
		if want.kind == actionAccept && g.operation == nil {
			g.accept(ctx, in.settings, in.settingsChecksum)
			continue
		}

		g.publishFacts(ownInputs)

		if g.operation != nil && supersedes(want, g.operation) {
			g.operation.Cancel()
		}

		if g.operation == nil {
			g.act(ctx, want, in.event)
		}

		retry.Arm(wake, g.clock, retry.Earliest(now, g.state.operations, g.state.accept))

		select {
		case <-ctx.Done():
			// leaves at the top of the loop
		case <-g.in.Ready():
		case res := <-g.operation.Done():
			g.finishOperation(res)
		case <-wake.C():
		}
	}
}

// act starts the operation want decides while the worker is idle; the loop accepts by itself. ev
// is the event next decided a run on.
func (g *Global) act(ctx context.Context, want action, ev *schedule.Event) {
	switch want.kind {
	case actionSync:
		g.startOperation(ctx, operation.Sync, "", &operationState{}, g.syncSteps)
	case actionRun:
		st := &operationState{enabled: ev.EnabledModules, accepted: g.state.settings.Accepted}
		g.startOperation(ctx, operation.Run, want.inputs, st, g.runSteps)
	case actionWait, actionAccept:
	}
}

func (g *Global) startOperation(ctx context.Context, kind operation.Kind, key string, st *operationState, steps func(*operationState) []operation.Step) {
	g.state.busy = true
	g.operationState = st
	g.operation = operation.Start(ctx, schedule.GlobalNode, kind, key, steps(st))
}

func (g *Global) collectOperation() *operationState {
	st := g.operationState
	g.operation, g.operationState = nil, nil
	g.state.busy = false

	return st
}

// finishOperation takes the worker's result back into the loop state and reports it. A cancelled
// operation is not a failure: next decides again. A failure stays reported until the next success.
func (g *Global) finishOperation(res operation.Result) {
	st := g.collectOperation()

	// What a publish step wrote stays published, whatever the run does afterwards: Output survives
	// a failed run (see the package doc).
	if st.output != "" {
		g.output = st.output
	}

	if res.Failed != nil {
		// A failure clears applied: disabling the module whose CRD broke the run may bring the
		// Inputs back to those of the last success, and Global must still run again.
		if res.Kind == operation.Run {
			g.state.applied = ""
		}

		if !res.Canceled {
			g.reportFailure(res)

			now := g.clock.Now()
			until := g.state.operations.Failed(operationKey(res.Kind, res.Key), now)

			g.logger.Warn("global operation failed", slog.String("operation", string(res.Kind)),
				slog.String("step", res.Failed.Name), slog.Duration("retry_in", until.Sub(now)), log.Err(res.Err))
		}

		return
	}

	switch res.Kind {
	case operation.Sync:
		// Only a recovered failure is reported: a clean start leaves the existing conditions
		// alone. No run precedes the first successful sync, so a deadline in the backoff means a
		// sync failed.
		recovered := !g.state.operations.Until().IsZero()

		g.state.operations.Succeeded()
		g.state.synced = true
		g.stopHooks = st.stopHooks

		if recovered {
			g.reporter.UpdateCurrent(func(r *model.CurrentReport) { r.HooksProcessed = model.TrueCondition() })
		}

		close(g.synced)
	case operation.Run:
		g.logger.Debug("global run succeeded", slog.Int("modules", len(st.modules)))

		g.state.operations.Succeeded()
		g.state.applied = res.Key
		g.reportSuccess(st.accepted)
	}
}

// exit cancels and waits for the worker, then stops the hook lanes (hooklane Lanes.Stop); it
// writes and publishes nothing. A sync that failed or was cancelled midway has stopped its lanes
// itself (hooklane Start, onStartup).
func (g *Global) exit() {
	if g.operation != nil {
		g.operation.Cancel()
		<-g.operation.Done()

		// A sync that succeeded as Stop came in has started the hooks.
		if st := g.collectOperation(); st.stopHooks != nil {
			g.stopHooks = st.stopHooks
		}
	}

	if g.stopHooks != nil {
		g.stopHooks()
	}
}
