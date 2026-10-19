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

package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

// Reasons the projection writes or reads besides the model's.
const (
	reasonPending                  = "Pending"
	reasonSettingsChanged          = "SettingsChanged"
	reasonNoResourceReconciliation = "NoResourceReconciliation"
	reasonNoWorkloads              = "NoWorkloads"
	reasonReconciling              = "Reconciling"
	reasonDegraded                 = "Degraded"
	reasonHookInitializationFailed = "HookInitializationFailed"
	reasonSyncHookFailed           = "SyncHookFailed"
)

// signal is one fact the projection decides on: Unknown (status "", not reported since the start,
// so the external conditions it feeds stay as they are), True, or False with its canonical
// external reason and message.
type signal struct {
	status  metav1.ConditionStatus
	reason  string
	message string
}

func (s *signal) isTrue() bool  { return s.status == metav1.ConditionTrue }
func (s *signal) isFalse() bool { return s.status == metav1.ConditionFalse }

// facts is everything the mappers and the summary read, built once per projection by buildFacts.
type facts struct {
	// requirements is the scheduler's decision; before the first activation also a requirements
	// gate.
	requirements      signal
	updateGate        signal // an update gate (requirements, window) blocks the new version
	readyOnFilesystem signal
	loaded            signal
	configured        signal
	hooks             signal // HooksProcessed, with the observed hook failure
	manifests         signal // ManifestsApplied; Unknown while applying
	installed         signal // the release is installed (Report.Current.Installed)
	scaled            signal
	maintenance       signal

	// candidateConfigured is the Candidate's own settings verdict: the update's settings gate while
	// there is a Candidate, so the running version's rejection never blocks the update.
	candidateConfigured signal

	// candidate: a new version is prepared or waits at a gate; the running version's conditions
	// say nothing about it.
	candidate       bool
	pending         bool // the engine works on the package and no install gate failed
	updating        bool // a new version is on its way (isUpdating)
	applying        bool // the release apply is in flight (ManifestsApplied=False/ApplyingManifests)
	settingsChanged bool // accepted settings no run has applied yet
	deleting        bool
	removeError     string

	// external are the statuses of the conditions on the resource, by type: the Installed gate,
	// its stickiness and the update phase read them.
	external map[string]metav1.ConditionStatus
}

// extTrue reports whether the resource carries condType True.
func (f *facts) extTrue(condType string) bool {
	return f.external[condType] == metav1.ConditionTrue
}

// hasExt reports whether the resource carries condType.
func (f *facts) hasExt(condType string) bool {
	_, ok := f.external[condType]
	return ok
}

// input is everything one projection reads: the engine's two halves of the status and the
// resource's own status.
type input struct {
	Report      model.Report
	Known       bool // Known: the engine knows the package; without it nothing internal is reported.
	Observation observed.Observation
	// External are the conditions on the resource; CurrentVersion is its status.currentVersion.
	External       []metav1.Condition
	CurrentVersion string
	Deleting       bool
}

// buildFacts translates in into the facts. Unknown Report conditions stay Unknown, so every
// external condition they feed stays as it is (a restart reports nothing until an operation
// decides).
func buildFacts(in input) facts {
	f := facts{deleting: in.Deleting, external: make(map[string]metav1.ConditionStatus, len(in.External))}

	for _, cond := range in.External {
		f.external[cond.Type] = cond.Status
	}

	if !in.Known {
		return f
	}

	rep := in.Report
	cur := rep.Current
	activated := rep.CurrentVersion != ""
	// The section outlives a Current: until the new revision's operation reports, its operation
	// conditions are the previous revision's.
	stale := activated && cur.Version != "" && cur.Version != rep.CurrentVersion

	if d := cur.Decision; d != nil {
		if d.Enabled {
			f.requirements = signal{status: metav1.ConditionTrue}
		} else {
			f.requirements = falseSignal(reasonRequirementsUnmet, d.Message)
		}
	}

	f.configured = configuredSignal(cur.Configured)

	if stale {
		// The new revision is being applied: its run is ahead and nothing of it reported yet.
		f.applying = true
	} else {
		f.hooks = hooksSignal(cur.HooksProcessed)
		f.manifests, f.applying = manifestsSignal(cur.ManifestsApplied)
		f.installed = reportSignal(cur.Installed, cur.Installed.Reason)
	}

	if h := in.Observation.HookFailure; h != nil && !f.hooks.isFalse() {
		f.hooks = falseSignal(hooksReason(h.Reason), h.Hook+": "+h.Message)
	}

	enabled := cur.Decision != nil && cur.Decision.Enabled
	applied := cur.AppliedChecksum != "" && cur.AcceptedChecksum == cur.AppliedChecksum

	// Accepted but not applied by a run yet. Not while the new revision is pending: its first run
	// applies them anyway.
	if !stale && enabled && cur.AppliedChecksum != "" && !applied && f.configured.isTrue() {
		f.configured = signal{}
		f.settingsChanged = true
	}

	// Maintenance in force only once a run applied it.
	if applied {
		if cur.Maintenance == nelm.NoResourceReconciliation {
			f.maintenance = signal{status: metav1.ConditionTrue, reason: reasonNoResourceReconciliation}
		} else {
			f.maintenance = signal{status: metav1.ConditionFalse}
		}
	}

	if activated {
		f.readyOnFilesystem = signal{status: metav1.ConditionTrue}
		f.loaded = signal{status: metav1.ConditionTrue}
	}

	buildUpdateFacts(rep, activated, &f)

	if (activated || rep.Candidate.Version != "") && firstFalse(f.installPipeline()) == nil {
		f.pending = true
	}

	f.scaled = scaledSignal(in.Observation)

	f.removeError = rep.RemoveError
	f.updating = isUpdating(in, stale)

	return f
}

// buildUpdateFacts translates the Candidate section: before the first activation it is the install
// pipeline, afterwards the candidate of an update.
func buildUpdateFacts(rep model.Report, activated bool, f *facts) {
	u := rep.Candidate
	if u.Version == "" {
		return
	}

	f.candidate = activated

	switch {
	case u.Blocked == model.ReasonRequirementsNotMet:
		if activated {
			f.updateGate = falseSignal(reasonRequirementsUnmet, u.Message)
		} else {
			f.requirements = falseSignal(reasonRequirementsUnmet, u.Message)
		}
	case u.Blocked == model.ReasonWaitingWindow:
		f.updateGate = falseSignal(model.ReasonWaitingWindow, u.Message)
	case u.Blocked == model.ReasonSettingsInvalid:
		f.configured = falseSignal(model.ReasonSettingsInvalid, u.Message)
		f.candidateConfigured = f.configured
		f.settingsChanged = false
	case u.Step == "attach":
		f.readyOnFilesystem = falseSignal(model.ReasonAttachFailed, u.Message)
	case u.Step == "load":
		f.loaded = falseSignal(model.ReasonLoadFromFilesystemFailed, u.Message)
	}
}

// isUpdating reports whether a new version is on its way: a candidate is prepared or blocked, the
// new revision has not reported yet, the resource still shows the previous version, or the update
// has not succeeded since it started (UpdateInstalled False holds the phase until the new
// version's operation succeeds, as the old status.currentVersion did).
func isUpdating(in input, stale bool) bool {
	rep := in.Report

	if rep.Candidate.Version != "" || stale {
		return true
	}

	if in.CurrentVersion != "" && rep.CurrentVersion != "" && in.CurrentVersion != rep.CurrentVersion {
		return true
	}

	for _, cond := range in.External {
		if cond.Type == ConditionUpdateInstalled {
			return cond.Status == metav1.ConditionFalse
		}
	}

	return false
}

// falseSignal is a False signal with its canonical reason.
func falseSignal(reason, message string) signal {
	return signal{status: metav1.ConditionFalse, reason: reason, message: message}
}

// reportSignal translates a Report condition with reason as its canonical reason when False.
func reportSignal(c model.Condition, reason string) signal {
	switch c.Status {
	case model.ConditionTrue:
		return signal{status: metav1.ConditionTrue}
	case model.ConditionFalse:
		return falseSignal(reason, c.Message)
	}

	return signal{}
}

// configuredSignal is Configured: every rejection is SettingsInvalid.
func configuredSignal(c model.Condition) signal {
	return reportSignal(c, model.ReasonSettingsInvalid)
}

// hooksSignal is HooksProcessed: initialization and synchronization failures are
// HookInitializationFailed, every other HookFailed.
func hooksSignal(c model.Condition) signal {
	return reportSignal(c, hooksReason(c.Reason))
}

func hooksReason(reason string) string {
	switch reason {
	case reasonHookInitializationFailed, reasonSyncHookFailed:
		return reasonHookInitializationFailed
	}

	return model.ReasonHookFailed
}

// manifestsSignal is ManifestsApplied: ApplyingManifests is progress, not a failure, so it is
// Unknown with applying set; ReleaseDeleteFailed (a failing disable) passes through; everything
// else is ManifestsApplyFailed.
func manifestsSignal(c model.Condition) (signal, bool) {
	if c.Status == model.ConditionFalse && c.Reason == model.ReasonApplyingManifests {
		return signal{}, true
	}

	reason := model.ReasonManifestsApplyFailed
	if c.Reason == model.ReasonReleaseDeleteFailed {
		reason = c.Reason
	}

	return reportSignal(c, reason), false
}

// scaledSignal translates the workload health: a monitor verdict wins; with none, a render without
// workloads is scaled. Nothing observed is Unknown.
func scaledSignal(o observed.Observation) signal {
	switch o.Health.State {
	case observed.HealthReady:
		return signal{status: metav1.ConditionTrue, reason: ConditionScaled}
	case observed.HealthProgressing:
		return falseSignal(reasonReconciling, o.Health.Message)
	case observed.HealthDegraded:
		return falseSignal(reasonDegraded, o.Health.Message)
	}

	if o.NoWorkloads {
		return signal{status: metav1.ConditionTrue, reason: reasonNoWorkloads}
	}

	return signal{}
}
