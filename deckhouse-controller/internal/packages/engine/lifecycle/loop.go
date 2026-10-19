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

// The package Reconciler loop, the update decision and the final gate's settings memo.

package lifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/gate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
)

// loop wakes on an input, the end of a preparation attempt or its timer, loads the inbox once and
// pushes the latest configuration and event to Current (which ignores repeats, so the package
// Reconciler keeps no record of them), then reconciles. It blocks only in the gates' checks, the
// activation, remove and shutdown.
func (r *Reconciler) loop(ctx context.Context) {
	defer close(r.done)
	defer r.wake.Stop()

	for {
		select {
		case <-ctx.Done():
			r.shutdown(ctx)
			return
		case <-r.in.Ready():
		case res := <-r.preparationDone():
			r.preparationFinished(res)
		case <-r.wake.C():
		}

		in := r.in.Load()

		if ctx.Err() != nil {
			r.shutdown(ctx)
			return
		}

		if in.removing {
			// An event of Current's revision only signals that the boot gate is open (Global
			// synchronized, or addon-operator converged): the uninstall may start the hooks, which
			// read the global values. Wait for it before RemoveNode, after which none arrives.
			// Nothing is pushed meanwhile, so Current cannot run.
			if r.current != nil && (in.event == nil || in.event.Version != r.current.Revision().Image.Version) {
				continue
			}

			r.remove(ctx)

			return
		}

		// The same Desired is loaded on every wake (inbox fields are levels); only a new one is hashed.
		if in.desired != nil && r.config != &in.desired.Configuration {
			r.config = &in.desired.Configuration
			r.configChecksum = configstate.Key(r.config)
		}

		if r.current != nil {
			r.current.Push(r.config, r.configChecksum, in.event)
		}

		if in.desired != nil {
			r.reconcile(ctx, in)
		}
	}
}

// reconcile decides over one input (gate.Decide) and acts. A blocked decision keeps the
// preparation of the desired revision: it stays valid, and a failed attempt is still retried. A
// package that turns from embedded to external, or back, is not handled: Boot looks for the
// recorded version where the new kind is served from, and fails.
func (r *Reconciler) reconcile(ctx context.Context, in reconcilerInput) {
	desired := *in.desired
	gi := r.gateInput(in)

	// The gate checks settings only for a prepared candidate, so only with one; the memo goes with it.
	deps := gate.Deps{Scheduler: r.deps.Scheduler}
	if r.candidate != nil {
		deps.Settings = &r.candidate.settings
	}

	d := gate.Decide(ctx, gi, deps)

	var windowAt time.Time

	switch d.Kind {
	case gate.UpToDate:
		r.stopCandidate(ctx)
		r.reporter.SetCandidate(model.CandidateReport{})
	case gate.Prepare:
		r.prepare(ctx, desired)
		r.reporter.SetCandidate(model.CandidateReport{
			Version: desired.Version,
			Step:    d.Step,
			Reason:  d.Reason,
			Message: d.Message,
		})
	case gate.Blocked:
		if r.preparing(desired) {
			r.prepare(ctx, desired)
		} else {
			r.stopCandidate(ctx)
		}

		windowAt = d.WakeAt

		r.reporter.SetCandidate(model.CandidateReport{Version: desired.Version, Blocked: d.Reason, Reason: d.Cause, Message: d.Message})
	case gate.Activate:
		r.activate(ctx, r.config, r.configChecksum)
	}

	r.resetWake(windowAt, gi.Now)
}

// resetWake sets the one timer to the earliest of the window gate's wake, the retry of a failed
// preparation and the retry of a settings check that failed to decide, or stops it. now is the
// time the gates decided at.
func (r *Reconciler) resetWake(windowAt, now time.Time) {
	at := windowAt

	if c := r.candidate; c != nil {
		if retryAt := c.backoff.Until(); c.op == nil && c.outcome().Step != "" && (at.IsZero() || retryAt.Before(at)) {
			at = retryAt
		}

		// Only ahead: a deadline that passed while the final gate was not reached waits for it.
		if retryAt := retry.Earliest(now, c.settings.backoff); !retryAt.IsZero() && (at.IsZero() || retryAt.Before(at)) {
			at = retryAt
		}
	}

	retry.Arm(r.wake, r.clock, at)
}

// gateInput copies what the decision reads; the preparation counts only when it is of the desired
// revision.
func (r *Reconciler) gateInput(in reconcilerInput) gate.Input {
	gi := gate.Input{
		Name:     r.name,
		Desired:  *in.desired,
		Settings: r.config.Settings,
		Now:      r.clock.Now(),
	}

	if r.current != nil {
		img := r.current.Revision().Image
		gi.Current = &img
	}

	if r.preparing(gi.Desired) {
		outcome := r.candidate.outcome()
		gi.Prepared = &outcome
	}

	return gi
}

// settingsCheck memoizes the final gate's last verdict: a blocked Candidate is checked on every
// wake, and ValidateSettings may run a hook. A failure to decide is not memoized: it is retried
// with backoff, like a failed preparation. The checksum is the whole key: the memo lives in the
// candidate and goes with it.
type settingsCheck struct {
	clock    retry.Clock
	checksum string // checksum is empty when no verdict is memoized.
	rejected error  // rejected is the verdict's rejection; nil when the settings are valid.
	// failure is the last failure to decide; backoff holds its retry, keyed by the checksum.
	failure error
	backoff retry.Backoff
}

// CheckSettings is the final gate's settings check over the memo: why object rejects settings, or
// why it could not decide; nil when it accepts them.
func (v *settingsCheck) CheckSettings(ctx context.Context, object model.Package, settings model.Settings, now time.Time) error {
	sum := configstate.Key(settings)
	if v.checksum == sum {
		return v.rejected
	}

	if v.backoff.Holds(sum, now) {
		return v.failure
	}

	var obj packageObject = object

	res, err := obj.ValidateSettings(ctx, settings.SchemaVersion, settings.Values)
	if err != nil {
		// From when the check returned: a check slower than the delay must not retry back to back.
		v.failure = err
		v.backoff.Failed(sum, v.clock.Now())

		return v.failure
	}

	v.failure = nil
	v.backoff.Succeeded()

	var rejected error
	if !res.Valid {
		rejected = errors.New(res.Message)
	}

	v.checksum, v.rejected = sum, rejected

	return rejected
}
