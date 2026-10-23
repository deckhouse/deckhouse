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

// The decision: what Current does now (next) and which operation in flight a decision cancels
// (supersedes). Both are pure; every rule of Current lives here.

package current

import (
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

type input struct {
	config *model.Configuration
	// configChecksum is config's configstate.Key: it dedups pushes, keys the accept backoff and
	// decides "in force"; "" while config is nil.
	configChecksum string
	event          *schedule.Event
	// valuesGen counts changed() and driftGen drifted(); both are part of the own inputs, so the
	// node runs again.
	valuesGen uint64
	driftGen  uint64
	uninstall bool // uninstall is never cleared once set.
}

// state is the loop's own state that next reads. A value, so next is a pure function of it.
type state struct {
	// version is the running revision (Image.Version); an event of another is not ours.
	version string
	// disabled: the disable succeeded and no operation but an uninstall started since.
	disabled bool
	// ran: the package has run, so it may write a settings checkpoint (hasRun): Current was
	// restored from one or one of its runs succeeded.
	ran bool
	// busy: an operation runs in the worker (set by start, cleared when its result is collected).
	busy bool
	// hooksStarted: the hooks run (set when an operation's result is collected), so the next
	// render is a run and not a sync.
	hooksStarted bool
	// applied is Facts.Applied; empty after start, so every node runs once after a boot or a
	// activation (nelm skips an unchanged release).
	applied  string
	settings configstate.State
	// operations delays the retry of a failed operation, keyed by operationKey; accept delays the
	// retry of a configuration whose accept failed, keyed by its configChecksum. Keyed, so new
	// Inputs or a new configuration never wait for the backoff of an older attempt.
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
	actionDisable
	actionUninstall
)

type action struct {
	kind actionKind
	// inputs are the event's Inputs an operation starts with; a run's or a disable's become applied
	// on success, a sync's never do.
	inputs string
}

// next decides what Current does now from its state and the latest inputs; ownInputs is their hash
// (Facts.OwnInputs) and now the time the backoffs are read at. Pure. The rules, in priority order:
func next(s state, in input, ownInputs string, now time.Time) action {
	// 1. Uninstall wins, also for a disabled Current: deleting an absent release is quick, and an
	// application still has its leftovers to purge.
	if in.uninstall {
		if s.operations.Holds(operationKey(operation.Uninstall, ""), now) {
			return action{kind: actionWait}
		}

		return action{kind: actionUninstall}
	}

	// 2. Accept a configuration the object does not hold yet, before anything renders from the
	// object. A new one at once: it cancels a run, because the object cannot be configured under
	// it. The retry of a failed one once its backoff expired, and only while the worker is idle: it
	// never cancels a run, which renders the last accepted settings that stay in force whatever the
	// retry finds, and it must not hide the rules below from supersedes.
	if in.config != nil && !s.settings.InForce(in.configChecksum) && !s.accept.Holds(in.configChecksum, now) {
		if retried := s.settings.Pushed == in.configChecksum; !retried || !s.busy {
			return action{kind: actionAccept}
		}
	}

	// 3. Not this Current's event (ours): none yet, one of another revision (the previous
	// Current's, right after activation), or a stale one.
	ev := in.event
	if !ours(s, ev, ownInputs) {
		return action{kind: actionWait}
	}

	// 4. Disabled by the decision but not here. A disable only deletes, so it needs no accepted
	// configuration.
	if !ev.Enabled && !s.disabled {
		if s.operations.Holds(operationKey(operation.Disable, ""), now) {
			return action{kind: actionWait}
		}

		return action{kind: actionDisable, inputs: ev.Inputs}
	}

	// 5. Without an accepted configuration there is nothing to run with: absent settings mean "not
	// ready", never empty values. An accepted one was pushed, so rule 3 checked the echo.
	if s.settings.Accepted == nil {
		return action{kind: actionWait}
	}

	// 6. The gate is open and the node is not converged on these Inputs. Hooks not started yet sync
	// first: their synchronizations change the values, and the run that follows renders them once
	// instead of a run that starts the hooks and a second one for what they changed. A failed sync
	// holds both until its backoff expires; a run only its own Inputs.
	if ev.Enabled && ev.Gate && ev.Inputs != s.applied {
		switch {
		case s.operations.Holds(operationKey(operation.Sync, ""), now):
			return action{kind: actionWait}
		case !s.hooksStarted:
			return action{kind: actionSync, inputs: ev.Inputs}
		case !s.operations.Holds(operationKey(operation.Run, ev.Inputs), now):
			return action{kind: actionRun, inputs: ev.Inputs}
		}
	}

	// 7. Converged, gated, held by the backoff, or disabled and done.
	return action{kind: actionWait}
}

// ours reports whether ev is of this revision and reflects the own inputs Current holds now.
// The echo is required once a configuration was pushed, for a disable as for a run: the enabled
// script reads its settings from this object, and right after activation or a restore the scheduler
// may have decided before the accept; acting on that stale decision would delete a release the
// accepted settings keep enabled. Before any push the own inputs carry no settings to miss.
func ours(s state, ev *schedule.Event, ownInputs string) bool {
	return ev != nil && ev.Version == s.version && (s.settings.Pushed == "" || ev.OwnInputs == ownInputs)
}

// supersedes reports whether want cancels the operation in flight; the loop acts on want once the
// worker is idle.
//   - An uninstall is never cancelled: the package Reconciler waits for it.
//   - A disable is cancelled only by an uninstall: it must not leave the hooks half stopped because
//     the decision flipped back.
//   - A run is cancelled only by a new configuration to accept (the object cannot be configured
//     under it), a disable and an uninstall. Neither a wait nor a run with other Inputs cancels
//     it: it finishes, its success sets applied to the Inputs it started with, and the next run
//     follows for the new ones. Why: a runtime hook that changes the values during runs would
//     otherwise cancel them again and again, also in the middle of a release apply.
//   - A sync is cancelled as a run is: the values its synchronizations change are rendered by the
//     run that follows.
func supersedes(want action, op *operation.Operation) bool {
	switch op.Kind {
	case operation.Uninstall:
		return false
	case operation.Disable:
		return want.kind == actionUninstall
	case operation.Run, operation.Sync:
		switch want.kind {
		case actionWait, actionSync, actionRun:
			return false
		default:
			return true
		}
	default:
		return false
	}
}

// operationKey is the backoff key of an operation: a run's kind and the Inputs it started with. A
// sync, a disable and an uninstall do the same whatever the Inputs are, so their kind alone: a
// runtime hook that changes the own inputs must not retry them outside their backoff.
func operationKey(kind operation.Kind, inputs string) string {
	if kind != operation.Run {
		return string(kind)
	}

	return string(kind) + ":" + inputs
}
