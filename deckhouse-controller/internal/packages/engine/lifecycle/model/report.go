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

package model

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Report is the lifecycle half of a package's status (Engine.Report); the observed half is
// internal/packages/observed (Engine.Observed), and the projection builds the status from both.
// Each section has exactly one writer, and the blocking Stop keeps a stopped writer from writing
// again.
type Report struct {
	// CurrentVersion is the activated version (status.currentVersion), written at activation, the
	// point of no return, and once when a restored package Reconciler starts. A value lagging
	// behind activation would make a restart restore the old version over a partly applied new one.
	CurrentVersion string
	Current        CurrentReport   // Current is written only by the Current goroutine.
	Candidate      CandidateReport // Candidate is written only by the package Reconciler.
	Removed        bool            // Removed: the package Reconciler's remove has finished.
	// RemoveError is the last failure of the by-name release delete of a remove without a Current;
	// cleared when the delete succeeds. While it is set, Removed stays false.
	RemoveError string
}

// Condition is one internal condition of the Current section. The zero value is "not reported":
// the projection keeps the existing external condition. Why: a restart is a reconciliation, not an
// install; a restored Current reports a condition only once an operation decides it, so Installed
// (and every other condition) never reads False for a moment after a restart. Build values with
// TrueCondition and FalseCondition.
type Condition struct {
	Status ConditionStatus
	// Reason is empty unless False. For a failed step it is the reason its error names, else the
	// step's own. Which condition is false is always the step's, so the projection derives the
	// external reason from the condition and reads this one as detail.
	Reason  string
	Message string
}

// ConditionStatus is the status of a Condition; its zero value is ConditionUnknown.
type ConditionStatus string

const (
	ConditionUnknown ConditionStatus = "" // ConditionUnknown: not reported since the start.
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
)

// TrueCondition returns a True condition.
func TrueCondition() Condition {
	return Condition{Status: ConditionTrue}
}

// FalseCondition returns a False condition with its reason and message.
func FalseCondition(reason, message string) Condition {
	return Condition{Status: ConditionFalse, Reason: reason, Message: message}
}

// Reasons of the Current and Candidate sections. Invalid settings have one reason in both: the same
// check (the revision's ValidateSettings) produces it.
const (
	ReasonSettingsInvalid = "SettingsInvalid"
	// ReasonRequirementsNotMet: CheckConstraints, CheckDependents (an enabled dependent rejects the
	// version) or the activation's AddNode (a cycle found after the final gate) rejected the
	// revision.
	ReasonRequirementsNotMet = "RequirementsNotMet"
	ReasonWaitingWindow      = "WaitingWindow"
	// ReasonAttachFailed: the attach step failed (download, verification and mount are one call).
	ReasonAttachFailed             = "AttachFailed"
	ReasonLoadFromFilesystemFailed = "LoadFromFilesystemFailed"
	// ReasonHookInitializationFailed: enabling or synchronizing the hook bindings failed.
	ReasonHookInitializationFailed = "HookInitializationFailed"
	ReasonHookFailed               = "HookFailed"
	ReasonManifestsApplyFailed     = "ManifestsApplyFailed"
	// ReasonApplyingManifests: ManifestsApplied is false while nelm applies a changed release.
	ReasonApplyingManifests   = "ApplyingManifests"
	ReasonReleaseDeleteFailed = "ReleaseDeleteFailed"
	// ReasonOrphansPurgeFailed: deleting what a removed application left beside its release failed.
	ReasonOrphansPurgeFailed = "OrphansPurgeFailed"
	// ReasonDisabled: Installed is false because the package is disabled.
	ReasonDisabled = "Disabled"
	// ReasonCRDsApplyFailed and ReasonWebhooksApplyFailed: a Global ensure step failed for a
	// module.
	ReasonCRDsApplyFailed     = "CRDsApplyFailed"
	ReasonWebhooksApplyFailed = "ConversionWebhooksApplyFailed"
)

// CurrentReport is the Current section. The projection maps it to the external conditions
// (ConfigurationApplied = Configured and HooksProcessed and ManifestsApplied). Current writes it
// only when it changes. A condition no operation has decided since the start is ConditionUnknown
// (see Condition).
type CurrentReport struct {
	// Version is the revision (Image.Version) of the Current whose operation last wrote
	// HooksProcessed, ManifestsApplied and Installed; "" until one did. The section outlives a
	// Current, so after an activation these conditions are the previous revision's until Version
	// equals Report.CurrentVersion: the projection keeps the update phase until then. Accepts and
	// decisions do not move it, they say nothing about the new revision's operation.
	Version string

	Configured       Condition // Configured: the latest pushed configuration was accepted.
	HooksProcessed   Condition // HooksProcessed: the hooks of the last operation succeeded.
	ManifestsApplied Condition // ManifestsApplied: the release of the last operation was applied.
	Installed        Condition // Installed: the release is installed; false only once disabled, never on a failure.

	// AcceptedValues are the effective settings in force, written on every successful accept,
	// before any run. Why beside the checkpoint: other components read the settings of deckhouse
	// and global as soon as they are in force, and a failing first run must not block them.
	AcceptedValues addonutils.Values
	// AcceptedChecksum is the effective values' checksum (with the maintenance mode and, for an
	// application, the resource requests); Global's covers the values alone.
	AcceptedChecksum string
	// Maintenance is the maintenance mode of the accepted configuration; the composition root
	// publishes the maintenance metric from it.
	Maintenance nelm.MaintenanceState

	// Decision is the scheduler's decision about Current's node; nil until one arrived. The
	// projection derives RequirementsMet and status.enabled from it.
	Decision *Decision

	// The settings checkpoint, written after a successful run, and after the disable and every
	// later accept while disabled. Boot restores AppliedValues at the restored object's latest settings version.
	//
	// AppliedValues are the effective settings of the checkpoint (status.lastAppliedConfiguration).
	AppliedValues addonutils.Values
	// AppliedChecksum is the checkpoint's effective values' checksum, as AcceptedChecksum.
	AppliedChecksum string
}

// Decision is the scheduler's decision about one node (Event.Enabled, Reason, Message).
type Decision struct {
	Enabled bool
	Reason  string // Reason is empty when enabled.
	Message string
}

// CandidateReport is the Candidate section: the preparation of the candidate, its blockers and
// failures (the UpdateInstalled condition). Written only by the package Reconciler and cleared when
// no update is needed or at activation. At most one of Blocked and Step is set.
type CandidateReport struct {
	Version string // Version is the desired version being evaluated.
	// Blocked is the reason of the gate that blocks: ReasonSettingsInvalid,
	// ReasonRequirementsNotMet or ReasonWaitingWindow. Nothing is broken; the world must change.
	Blocked string
	// Step is the preparation step that failed (attach, load); the package Reconciler retries it.
	Step string
	// Reason is the failed step's own reason (AttachFailed, LoadFromFilesystemFailed); the
	// projection derives the external reason from Step. For Blocked=RequirementsNotMet it is the scheduler's reason
	// (ConstraintError.Reason, e.g. DependencyNotEnabled), which modules show as is.
	Reason  string
	Message string
}
