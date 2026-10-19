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

// The decision: what Global does now (next) and which operation in flight a decision cancels
// (supersedes). Both are pure; every rule of Global lives here.

package global

import (
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

type input struct {
	settings *model.Settings
	// settingsChecksum is settings' configstate.Key; "" while settings is nil.
	settingsChecksum string
	event            *schedule.Event
	// valuesGen counts changed(): a global runtime hook changed the values. Part of the own inputs,
	// so Global runs again and publishes them.
	valuesGen uint64
}

// state is the loop's own state that next reads. A value, so next is a pure function of it.
type state struct {
	// synced: the sync succeeded; the hooks run for as long as Global does.
	synced bool
	busy   bool
	// applied becomes Facts.Applied; a failed run clears it, and it is empty after start, so
	// Global runs once after every boot.
	applied  string
	settings configstate.State
	// operations and accept are keyed as Current's: the run without a module whose CRD is broken
	// starts at once.
	operations retry.Backoff
	accept     retry.Backoff
}

// actionKind is what next decides the loop does now.
type actionKind int

const (
	actionWait actionKind = iota
	actionAccept
	actionSync
	actionRun
)

type action struct {
	kind actionKind
	// inputs are the event's Inputs a run starts with; on success they become applied.
	inputs string
}

// next is Current's next (current/next.go) for Global. The rules, in priority order:
func next(s state, in input, ownInputs string, now time.Time) action {
	// 1. Accept settings the global module does not hold yet, as Current does (its rule 2). First
	// also before the sync: synchronization runs hooks that read the settings, so the checkpoint
	// goes into the global values before it. New settings during the sync wait for it (see
	// supersedes).
	if in.settings != nil && !s.settings.InForce(in.settingsChecksum) && !s.accept.Holds(in.settingsChecksum, now) {
		if retried := s.settings.Pushed == in.settingsChecksum; !retried || !s.busy {
			return action{kind: actionAccept}
		}
	}

	// 2. Start and synchronize the hooks once, before any run: the global values a run renders and
	// the gate rules read are filled by the synchronization. A failed sync is retried after its
	// backoff; it needs no settings and no event (the scheduler is still paused).
	if !s.synced {
		if s.operations.Holds(operationKey(operation.Sync, ""), now) {
			return action{kind: actionWait}
		}

		return action{kind: actionSync}
	}

	// 3. No event yet.
	ev := in.event
	if ev == nil {
		return action{kind: actionWait}
	}

	// 4. The own-inputs echo: run only on an event computed over the own inputs Global holds now;
	// absent settings mean "not ready".
	if s.settings.Accepted == nil || ev.OwnInputs != ownInputs {
		return action{kind: actionWait}
	}

	// 5. The gate is open, the node is not converged on these Inputs, which carry the identity of
	// every enabled module, and these Inputs did not just fail.
	if ev.Gate && ev.Inputs != s.applied && !s.operations.Holds(operationKey(operation.Run, ev.Inputs), now) {
		return action{kind: actionRun, inputs: ev.Inputs}
	}

	// 6. Converged, gated or held by the backoff.
	return action{kind: actionWait}
}

// supersedes is Current's table (current/next.go) without disable and uninstall, plus the sync:
//   - The sync is never cancelled by a decision, only by Stop: the boot gate waits for it, and
//     cancelling it would start the hooks again from scratch. Settings pushed meanwhile are
//     accepted right after it, before the first run.
//   - A run is cancelled only by new settings to accept, as in Current; a run with other Inputs
//     waits for it to finish. A module's broken CRD fails the run at once (the API rejects it), and
//     the run without that module follows with its new Inputs, held by no backoff.
func supersedes(want action, op *operation.Operation) bool {
	if op.Kind != operation.Run {
		return false
	}

	switch want.kind {
	case actionWait, actionRun:
		return false
	default:
		return true
	}
}

// operationKey is the backoff key of an operation: its kind and the Inputs it started with. The
// sync has no Inputs: it is retried until it succeeds.
func operationKey(kind operation.Kind, inputs string) string {
	if kind == operation.Sync {
		return string(operation.Sync)
	}

	return string(kind) + ":" + inputs
}
