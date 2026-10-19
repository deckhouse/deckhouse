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

// Package gate is the update decision of one package: Decide answers whether the desired revision
// is Current, must be prepared, is blocked, or may be activated. It reads one Input and calls only
// read-only checks, so it has no effects; the package Reconciler acts on the Decision.
package gate

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// scheduler is the admission check of the scheduler (the package Reconciler's scheduler satisfies
// it).
type scheduler interface {
	// CheckConstraints reports whether a node with constraints would be admitted now. Read-only.
	CheckConstraints(name string, constraints schedule.Constraints) error
	// CheckDependents reports whether an enabled dependent rejects version. Read-only.
	CheckDependents(name string, version *semver.Version) error
}

// settings checks settings against the prepared object (its own ValidateSettings); nil means
// accepted, an error a rejection or a failure to decide, both Blocked here. The package
// Reconciler's implementation memoizes a verdict and retries a failure with backoff. Blocking: it
// may run a hook.
type settings interface {
	// now is Input.Now: the retry is held by the time the decision reads.
	CheckSettings(ctx context.Context, object model.Package, settings model.Settings, now time.Time) error
}

// Deps are the read-only checks Decide calls.
type Deps struct {
	Scheduler scheduler
	Settings  settings
}

// Input is everything the decision reads, taken once per wake.
type Input struct {
	Name    string
	Desired model.Desired
	// Settings are those the package Reconciler pushes to Current: the final gate checks exactly
	// what the activation pushes.
	Settings model.Settings
	// Current is Current's image; nil for a first install.
	Current *model.Image
	// Prepared is the latest result of the preparation of the desired revision; nil when there is
	// none.
	Prepared *Prepared
	Now      time.Time
}

// Prepared is the preparation of the desired revision as the decision reads it.
type Prepared struct {
	// Ready: the revision is attached and loaded; Object is the loaded object, idle until the
	// activation, so the final gate's read-only checks may use it.
	Ready  bool
	Object model.Package
	// Step is the preparation step that failed last (the package Reconciler retries it); empty when
	// none did. Reason is the step's own; Message says why.
	Step    string
	Reason  string
	Message string
}

// Kind says who must act on a decision.
type Kind int

const (
	UpToDate Kind = iota // UpToDate: the desired revision is Current; no Candidate is needed.
	// Prepare: the revision is not prepared yet; the package Reconciler retries a failed
	// preparation.
	Prepare
	// Blocked: nothing is broken and the world must change (settings, requirements, the clock).
	// Re-evaluated on every wake, never retried with backoff: retrying cannot change the answer.
	Blocked
	Activate // Activate: every gate passed; the prepared revision may become Current.
)

// Decision is Decide's answer.
type Decision struct {
	Kind   Kind
	Reason string
	// Cause is the scheduler's own reason of a requirements blocker (ConstraintError.Reason, e.g.
	// DependencyNotEnabled); empty for every other decision.
	Cause   string
	Message string
	Step    string    // Step is the failed preparation step (Prepare).
	WakeAt  time.Time // WakeAt is when the window gate wants to be evaluated again (Blocked).
}

// windows is the update calendar as the window gate reads it (model.Windows).
type windows interface {
	IsAllowed(t time.Time) bool
	NextAllowedTime(t time.Time) time.Time
}

type packageObject interface {
	GetConstraints() schedule.Constraints
	GetVersion() *semver.Version
}

// Decide runs the gates in their contract order, current → requirements → prepared → final
// (settings of an update, constraints, dependents) → window, and returns the first that stops;
// Activate when none does. Every wake decides from the start. The gates before prepared check requirements only,
// before anything is downloaded: settings can be checked only once the revision is loaded. The
// window comes last, so the revision is downloaded, loaded and checked ahead and the activation
// happens at the window start; meanwhile the package Reconciler keeps the prepared candidate, and
// every wake (a settings change among them) runs the final gate again.
func Decide(ctx context.Context, in Input, deps Deps) Decision {
	// Current: the same version. A version names one image (versions are immutable), so no
	// preparation ever attaches Current's own instance.
	if c := in.Current; c != nil && in.Desired.Version == c.Version {
		return Decision{Kind: UpToDate}
	}

	// Requirements: the constraints declared on the package version resource.
	if err := deps.Scheduler.CheckConstraints(in.Name, in.Desired.Requirements); err != nil {
		return requirementsBlocked(err)
	}

	// Prepared: wait for the preparation and report its failure.
	r := in.Prepared
	if r == nil || !r.Ready {
		d := Decision{Kind: Prepare}
		if r != nil {
			d.Step, d.Reason, d.Message = r.Step, r.Reason, r.Message
		}

		return d
	}

	// Final: the loaded revision against the settings the package Reconciler pushes (the check the
	// new Current accepts with) and its own constraints, which depend on the graph and are checked
	// on every wake. Not atomic with the activation: AddNode is the authoritative admission check.
	// The settings only for an update: they protect a working Current from a revision that rejects
	// them. A first install has none to protect, and its decision exists only on the node the
	// activation adds, so a module nobody enabled must not stop at settings it never needs; its
	// Current rejects them on accept and does not run without accepted ones.
	if in.Current != nil {
		if err := deps.Settings.CheckSettings(ctx, r.Object, in.Settings, in.Now); err != nil {
			return Decision{Kind: Blocked, Reason: model.ReasonSettingsInvalid, Message: err.Error()}
		}
	}

	var object packageObject = r.Object
	if err := deps.Scheduler.CheckConstraints(in.Name, object.GetConstraints()); err != nil {
		return requirementsBlocked(err)
	}

	// Dependents: an enabled package that depends on this one must accept the new version, or the
	// activation would disable it and delete its release. A first install is checked as well: an
	// optional dependency that is present must match too. Like the other final checks it is not
	// atomic with the activation, and AddNode does not repeat it: Restore adds nodes through
	// AddNode too, and a boot must not fail over a mismatch that is already in place.
	if err := deps.Scheduler.CheckDependents(in.Name, object.GetVersion()); err != nil {
		return requirementsBlocked(err)
	}

	// Window: passes for a first install (no Current to protect), in manual mode, with Force,
	// without windows or inside one. The notification before a window is the controller's.
	policy := in.Desired.Policy
	if in.Current == nil || policy.Mode == model.UpdateModeManual || policy.Force || policy.Windows == nil {
		return Decision{Kind: Activate}
	}

	var w windows = policy.Windows
	if w.IsAllowed(in.Now) {
		return Decision{Kind: Activate}
	}

	next := w.NextAllowedTime(in.Now)

	return Decision{
		Kind:    Blocked,
		Reason:  model.ReasonWaitingWindow,
		Message: "waiting for the update window at " + next.UTC().Format(time.RFC3339),
		WakeAt:  next,
	}
}

// requirementsBlocked is the decision of a requirements gate that err rejected.
func requirementsBlocked(err error) Decision {
	return Decision{
		Kind:    Blocked,
		Reason:  model.ReasonRequirementsNotMet,
		Cause:   ConstraintReason(err),
		Message: ConstraintMessage(err),
	}
}

// ConstraintReason returns the reason of a CheckConstraints or AddNode error; empty when err is no
// *schedule.ConstraintError.
func ConstraintReason(err error) string {
	var ce *schedule.ConstraintError
	if errors.As(err, &ce) {
		return ce.Reason
	}

	return ""
}

// ConstraintMessage returns the message of a CheckConstraints or AddNode error without the reason
// prefix ConstraintError.Error() adds: the decision carries its own reason.
func ConstraintMessage(err error) string {
	var ce *schedule.ConstraintError
	if errors.As(err, &ce) {
		return ce.Message
	}

	return err.Error()
}
