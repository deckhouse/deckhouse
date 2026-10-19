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

// The step lists of Current's operations; the order is the contract. Only the uninstall list
// differs by kind (uninstallSteps adds an application's purge), so a kind's policy can diverge
// locally.

package current

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
)

// Step names, in reports and logs.
const (
	stepStartHooks       = "startHooks"
	stepOnStartup        = "onStartup"
	stepBeforeHelm       = "beforeHelm"
	stepApply            = "apply"
	stepAfterHelm        = "afterHelm"
	stepReapply          = "reapply"
	stepRemoveMonitor    = "removeMonitor"
	stepBeforeDeleteHelm = "beforeDeleteHelm"
	stepDeleteRelease    = "deleteRelease"
	stepAfterDeleteHelm  = "afterDeleteHelm"
	stepStopHooks        = "stopHooks"
	stepPurgeOrphans     = "purgeOrphans"
)

// operationState is what one operation's steps share: a snapshot of the loop state plus their
// outputs. The steps touch only this and Current's immutable fields (and the apply step's mark
// through the reporter); the loop reads it back once the operation is done.
type operationState struct {
	// accepted is the configuration the object holds for the whole operation.
	accepted  *configstate.Accepted
	hooksStop func() // hooksStop is in/out: steps start or stop the hooks; nil while none are.
	reapply   bool   // reapply: AfterHelm changed the values.
	// monitorPaused: resume the drift monitor when the run is done.
	monitorPaused bool
	// steps are the operation's steps; its result is reported over them (operationConditions).
	steps []operation.Step
}

func (r *Reconciler) runSteps(st *operationState) []operation.Step {
	return []operation.Step{
		{
			Name: stepStartHooks, Condition: operation.HooksProcessed, Reason: model.ReasonHookInitializationFailed,
			Run: func(ctx context.Context) error { return r.startHooks(ctx, st) },
		},
		{Name: stepOnStartup, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed, Run: r.onStartup},
		{Name: stepBeforeHelm, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed, Run: r.beforeHelm},
		{Name: stepApply, Condition: operation.ManifestsApplied, Reason: model.ReasonManifestsApplyFailed, Run: r.apply},
		{
			Name: stepAfterHelm, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed,
			Run: func(ctx context.Context) error { return r.afterHelm(ctx, st) },
		},
		{
			Name: stepReapply, Condition: operation.ManifestsApplied, Reason: model.ReasonManifestsApplyFailed,
			Run: func(ctx context.Context) error { return r.reapply(ctx, st) },
		},
	}
}

// disableSteps is the disable operation. A disable does not start the hooks: the deletion hooks
// run only when this Current started them; otherwise it only deletes the release, which is quick
// when it is absent. Why: after a restart ~80 disabled modules would otherwise start their hooks on
// every boot. A module disabled while Deckhouse was down therefore loses its deletion hooks;
// accepted.
func (r *Reconciler) disableSteps(st *operationState) []operation.Step {
	return []operation.Step{
		{Name: stepRemoveMonitor, Condition: operation.ManifestsApplied, Reason: model.ReasonReleaseDeleteFailed, Run: r.removeMonitor},
		{
			Name: stepBeforeDeleteHelm, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed,
			Run: func(ctx context.Context) error { return r.beforeDeleteHelm(ctx, st) },
		},
		{Name: stepDeleteRelease, Condition: operation.ManifestsApplied, Reason: model.ReasonReleaseDeleteFailed, Run: r.deleteRelease},
		{
			Name: stepAfterDeleteHelm, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed,
			Run: func(ctx context.Context) error { return r.afterDeleteHelm(ctx, st) },
		},
		{
			Name: stepStopHooks, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed,
			Run: func(context.Context) error { return r.stopHooks(st) },
		},
	}
}

// uninstallSteps is the uninstall operation: the hooks prepared as for a run (startHooks, a no-op
// while they run; onStartup, which the object skips after a success in this enable), the disable
// steps, and for an application the purge of what it left beside its release. Unlike a disable it
// starts the hooks, so the deletion hooks always run: a removal is rare and must leave nothing
// behind. BeforeHelm does not run: it prepares a release about to be deleted. A Current whose
// disable succeeded and that ran nothing since prepares nothing: that disable already ran the
// deletion steps, or skipped them by its own rule.
func (r *Reconciler) uninstallSteps(st *operationState) []operation.Step {
	var steps []operation.Step

	if !r.state.disabled {
		steps = append(steps,
			operation.Step{
				Name: stepStartHooks, Condition: operation.HooksProcessed, Reason: model.ReasonHookInitializationFailed,
				Run: func(ctx context.Context) error { return r.startHooks(ctx, st) },
			},
			operation.Step{Name: stepOnStartup, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed, Run: r.onStartup},
		)
	}

	steps = append(steps, r.disableSteps(st)...)

	// Only a removal purges: a disabled application may use its leftovers again.
	if r.kind == model.KindApplication {
		steps = append(steps,
			operation.Step{Name: stepPurgeOrphans, Condition: operation.ManifestsApplied, Reason: model.ReasonOrphansPurgeFailed, Run: r.purgeOrphans},
		)
	}

	return steps
}

// apply: a package without a chart has no release, which is not a failure. ApplyingManifests is
// marked only while nelm applies a changed release: marking an unchanged one would flap
// ConfigurationApplied on every run. The step's result, reported after the worker returned,
// overwrites the mark.
func (r *Reconciler) apply(ctx context.Context) error {
	if err := r.deps.Nelm.Upgrade(ctx, r.namespace, r.pkg, r.reportApplying, r.drifted); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
		return fmt.Errorf("upgrade release: %w", err)
	}

	return nil
}

func (r *Reconciler) reapply(ctx context.Context, st *operationState) error {
	if !st.reapply {
		return nil
	}

	return r.apply(ctx)
}

// removeMonitor stops the drift monitor: a release being deleted must not report drift.
func (r *Reconciler) removeMonitor(context.Context) error {
	r.deps.Nelm.RemoveMonitor(r.name)

	return nil
}

// deleteRelease: a missing release is nil.
func (r *Reconciler) deleteRelease(ctx context.Context) error {
	if err := r.deps.Nelm.Delete(ctx, r.namespace, r.name); err != nil {
		return fmt.Errorf("delete release: %w", err)
	}

	return nil
}

// purgeOrphans runs last, when nothing of the application is left to recreate what it deletes.
func (r *Reconciler) purgeOrphans(ctx context.Context) error {
	// The purger takes the carrier: it reads a declaration Current itself never calls.
	if err := r.deps.Orphans.Purge(ctx, r.rev.Package); err != nil {
		return fmt.Errorf("delete orphan resources: %w", err)
	}

	return nil
}
