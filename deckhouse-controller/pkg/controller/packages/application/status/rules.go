// Copyright 2025 Flant JSC
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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// External condition types — what the user sees on the Application resource.
const (
	// ConditionInstalled reflects the state of the first install of the application.
	// True when the install pipeline completed; False while it is blocked or has failed
	// at one of: waiting for module converge (Pending), unmet requirements, download,
	// load from filesystem, settings validation, hooks, or Helm manifest apply.
	// Sticky: once True it is never retracted — subsequent failures surface on
	// UpdateInstalled instead.
	// Possible reasons: Pending, RequirementsUnmet, DownloadFailed,
	// LoadFromFilesystemFailed, SettingsInvalid, HookInitializationFailed,
	// HookFailed, ManifestsApplyFailed.
	ConditionInstalled = "Installed"

	// ConditionUpdateInstalled reflects the state of installing a new version over
	// a running application. While the update is in progress, the previously
	// installed version may keep serving traffic, so Ready/Scaled/ConfigurationApplied
	// /Managed can stay True while UpdateInstalled reports a problem with the new
	// version. False means the update is blocked or has failed.
	// Possible reasons: Pending, DownloadFailed, LoadFromFilesystemFailed,
	// SettingsInvalid, HookInitializationFailed, HookFailed, ManifestsApplyFailed.
	ConditionUpdateInstalled = "UpdateInstalled"

	// ConditionReady reflects user-facing readiness of the application.
	// On first install it tracks Installed and goes False alongside it on failure.
	// During an update it can stay True while the previous version keeps serving.
	// On reconcile it goes False when the running version can no longer be trusted
	// (download, hook, or manifest-apply failures); a settings-only failure does
	// not affect Ready because the running version's settings are unchanged.
	// Possible reasons: Pending, RequirementsUnmet, DownloadFailed,
	// LoadFromFilesystemFailed, SettingsInvalid, HookInitializationFailed,
	// HookFailed, ManifestsApplyFailed, Ready (when True).
	ConditionReady = "Ready"

	// ConditionScaled reflects the runtime scaling state of the application.
	// Owned exclusively by the workload health monitor — no other condition
	// influences this value. True at steady state, False when at least one
	// workload is rolling out (Reconciling) or failed (Degraded), Unknown
	// when there are no workloads to observe yet.
	// Possible reasons: Reconciling (False), Degraded (False), Scaled (True).
	ConditionScaled = "Scaled"

	// ConditionManaged reflects whether the controller is actively managing the
	// application. False means the controller cannot bring the application to
	// (or keep it in) a managed state: typically hook, Helm, or — during reconcile —
	// download failures, where continuing to manage the current state is unsafe.
	// Settings-only failures do not break Managed. Unknown when a hard dependency
	// is disabled under the running app — managing is meaningless until the
	// dependency returns, but the cause is external rather than a controller failure.
	// Possible reasons: RequirementsUnmet, DownloadFailed, HookInitializationFailed,
	// HookFailed, ManifestsApplyFailed, Managed (when True).
	ConditionManaged = "Managed"

	// ConditionConfigurationApplied reflects whether the desired configuration —
	// settings, render, hooks, manifests — was successfully applied. False on
	// invalid settings, hook errors, or Helm errors. On reconcile a download
	// failure makes the configuration state Unknown (we cannot tell whether the
	// desired config is on disk). A disabled dependency under the running app
	// also forces Unknown — the desired configuration is no longer being maintained.
	// Possible reasons: RequirementsUnmet, DownloadFailed, SettingsInvalid,
	// HookInitializationFailed, HookFailed, ManifestsApplyFailed,
	// ConfigurationApplied (when True).
	ConditionConfigurationApplied = "ConfigurationApplied"
)

// Internal condition names as plain strings — every condmap.State method takes
// a string, so converting once at the package level avoids repeating the cast.
const (
	intPending           = string(intstatus.ConditionPending)
	intRequirementsMet   = string(intstatus.ConditionRequirementsMet)
	intReadyOnFilesystem = string(intstatus.ConditionReadyOnFilesystem)
	intLoaded            = string(intstatus.ConditionLoaded)
	intConfigured        = string(intstatus.ConditionConfigured)
	intHooksProcessed    = string(intstatus.ConditionHooksProcessed)
	intManifestsApplied  = string(intstatus.ConditionManifestsApplied)
	intScaled            = string(intstatus.ConditionScaled)
)

// canonicalReason returns the user-facing reason for an external condition
// derived from the failing internal condition. The mapper is the authoritative
// source of external reasons — internal reasons are debug-only and never
// exported as-is, except as a discriminator when one internal condition maps
// to multiple external reasons.
//
// Special cases:
//   - HooksProcessed: the internal reason distinguishes HookInitializationFailed
//     (sync/init phase) from HookFailed (runtime hooks).
//   - ManifestsApplied: ApplyingManifests is a non-failure mid-step indicator
//     and passes through; every other internal reason becomes ManifestsApplyFailed.
func canonicalReason(internalCond, internalReason string) string {
	switch internalCond {
	case intPending:
		return "Pending"
	case intRequirementsMet:
		return "RequirementsUnmet"
	case intReadyOnFilesystem:
		return "DownloadFailed"
	case intLoaded:
		return "LoadFromFilesystemFailed"
	case intConfigured:
		return "SettingsInvalid"
	case intHooksProcessed:
		switch internalReason {
		case "HookInitializationFailed", "SyncHookFailed":
			return "HookInitializationFailed"
		}
		return "HookFailed"
	case intManifestsApplied:
		if internalReason == string(intstatus.ConditionReasonApplyingManifests) {
			return internalReason
		}
		return "ManifestsApplyFailed"
	case intScaled:
		// The health monitor is the only non-True writer of intScaled, and it
		// produces canonical external reasons directly ("Reconciling",
		// "Degraded"). No translation needed — pass through.
		return internalReason
	}

	return ""
}

// emit builds an external condition from an internal one. The Reason for
// failure status is the canonical reason for the external vocabulary; the
// internal reason is debug detail and is read only to disambiguate where one
// internal condition maps to multiple external reasons. The Message is taken
// verbatim from the internal condition. True conditions carry no message and
// use the external condition type as their reason (per Kubernetes convention
// and the reason vocabulary documented on each external condition).
func emit(state condmap.State, ext string, status metav1.ConditionStatus, internalCond string) metav1.Condition {
	if status == metav1.ConditionTrue {
		return metav1.Condition{Type: ext, Status: status, Reason: ext}
	}

	intReason, message := state.GetIntReason(internalCond)

	return metav1.Condition{
		Type:    ext,
		Status:  status,
		Reason:  canonicalReason(internalCond, intReason),
		Message: message,
	}
}

// phase classifies a mapping run by the externally observed install state.
type phase int

const (
	phaseInstall   phase = iota // not yet installed
	phaseUpdate                 // installed and a version change is in progress
	phaseReconcile              // installed and not updating
)

// phaseOf classifies the current run.
func phaseOf(state condmap.State) phase {
	if !state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		return phaseInstall
	}
	if state.IsUpdating() {
		return phaseUpdate
	}

	return phaseReconcile
}

// installPipeline lists every gate from requirements to manifests in priority
// order. The other chains are slices into it (so they cannot drift apart);
// reconcileChain combines the filesystem gate with late-stage gates because
// settings failures don't break a running app on reconcile.
var installPipeline = []string{
	intRequirementsMet,   // [0] install only
	intReadyOnFilesystem, // [1] update onwards
	intLoaded,            // [2]
	intConfigured,        // [3] config phase onwards
	intHooksProcessed,    // [4] late stage onwards
	intManifestsApplied,  // [5]
}

var (
	updatePipeline = installPipeline[1:] // RequirementsMet not re-checked on version change
	configPipeline = installPipeline[3:] // settings + hooks + manifests
	lateStage      = installPipeline[4:] // hooks + manifests

	// reconcileChain: gates that break a running app on reconcile.
	reconcileChain = []string{intReadyOnFilesystem, intHooksProcessed, intManifestsApplied}
)

// firstFalse returns the first internal condition in chain whose status is False.
func firstFalse(state condmap.State, chain []string) (string, bool) {
	for _, cond := range chain {
		if state.IntEqual(cond, metav1.ConditionFalse) {
			return cond, true
		}
	}

	return "", false
}

// pipelineBlocker returns the highest-priority blocker for an install or
// update flow: Pending=True wins over any False condition in chain.
func pipelineBlocker(state condmap.State, chain []string) (string, bool) {
	if state.IntEqual(intPending, metav1.ConditionTrue) {
		return intPending, true
	}

	return firstFalse(state, chain)
}

// buildMapper returns the standard set of mappers in evaluation order.
func buildMapper() condmap.Mapper {
	return condmap.Mapper{
		Maps: []condmap.Map{
			mapInstalled,
			mapUpdateInstalled,
			mapReady,
			mapScaled,
			mapManaged,
			mapConfigurationApplied,
		},
	}
}

// Convention for all mappers below: failure checks come BEFORE success checks.
// A previously-True runtime condition (e.g. Scaled=True from the old version)
// must not mask a fresh failure (e.g. HooksProcessed=False from a new attempt).

// isDependencyDisabled reports whether a previously-installed app has lost a
// hard requirement (typically a dependency module being disabled). The cause
// is external — public conditions reflect that distinction by going to False
// for user-facing signals (Installed, Ready) and Unknown for runtime and
// configuration signals (Scaled, ConfigurationApplied, Managed). It overrides
// the Installed stickiness because we want users to see that the app stopped
// being installed for an external reason, not silently keep Installed=True.
func isDependencyDisabled(state condmap.State) bool {
	return state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) &&
		state.IntEqual(intRequirementsMet, metav1.ConditionFalse)
}

// mapInstalled is sticky: once Installed=True it is never retracted, except
// when a hard dependency is removed under the running app — see
// isDependencyDisabled.
func mapInstalled(state condmap.State) metav1.Condition {
	if isDependencyDisabled(state) {
		return emit(state, ConditionInstalled, metav1.ConditionFalse, intRequirementsMet)
	}
	if state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		return metav1.Condition{}
	}
	if cond, ok := pipelineBlocker(state, installPipeline); ok {
		return emit(state, ConditionInstalled, metav1.ConditionFalse, cond)
	}
	if state.IntEqual(intScaled, metav1.ConditionTrue) {
		return emit(state, ConditionInstalled, metav1.ConditionTrue, intScaled)
	}

	return metav1.Condition{}
}

// mapUpdateInstalled reports the progress of installing a new version on top of
// an already-installed application. Fires only after Installed=True and either
// an update is in progress or a previous update condition exists. Falls silent
// when a dependency is disabled — the dependency-disabled state is the
// dominant signal and is reported on the other conditions.
func mapUpdateInstalled(state condmap.State) metav1.Condition {
	if isDependencyDisabled(state) {
		return metav1.Condition{}
	}
	if !state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		return metav1.Condition{}
	}

	updating := state.IsUpdating()
	if !updating && !state.HasExt(ConditionUpdateInstalled) {
		return metav1.Condition{}
	}

	if updating {
		if cond, ok := pipelineBlocker(state, updatePipeline); ok {
			return emit(state, ConditionUpdateInstalled, metav1.ConditionFalse, cond)
		}
	}
	if state.IntEqual(intScaled, metav1.ConditionTrue) {
		return emit(state, ConditionUpdateInstalled, metav1.ConditionTrue, intScaled)
	}

	return metav1.Condition{}
}

// mapReady tracks user-facing readiness. Failure chain depends on phase:
//   - install:   any pipeline failure breaks readiness.
//   - update:    only hook/manifest failures (old version still serves).
//   - reconcile: filesystem and hook/manifest failures (settings alone do not).
//
// A disabled dependency on a running app forces Ready=False regardless of
// phase — the app is no longer serving.
func mapReady(state condmap.State) metav1.Condition {
	if isDependencyDisabled(state) {
		return emit(state, ConditionReady, metav1.ConditionFalse, intRequirementsMet)
	}

	var blocker string
	var ok bool

	switch phaseOf(state) {
	case phaseInstall:
		blocker, ok = pipelineBlocker(state, installPipeline)
	case phaseUpdate:
		blocker, ok = firstFalse(state, lateStage)
	case phaseReconcile:
		blocker, ok = firstFalse(state, reconcileChain)
	}

	if ok {
		return emit(state, ConditionReady, metav1.ConditionFalse, blocker)
	}
	if state.IntEqual(intScaled, metav1.ConditionTrue) {
		return emit(state, ConditionReady, metav1.ConditionTrue, intScaled)
	}

	return metav1.Condition{}
}

// mapScaled mirrors the internal Scaled condition verbatim. Scaled is owned
// by a separate controller (the workload health monitor) and is not derived
// from any other condition — install/update/dependency signals never override
// it. When the internal condition is absent, external Scaled is Unknown.
func mapScaled(state condmap.State) metav1.Condition {
	status, ok := state.GetIntStatus(intScaled)
	if !ok {
		return metav1.Condition{Type: ConditionScaled, Status: metav1.ConditionUnknown}
	}

	return emit(state, ConditionScaled, status, intScaled)
}

// mapManaged reports whether the controller can actively manage the application.
// Settings failures never break management; filesystem failures break it only
// during reconcile (the running state is no longer trustworthy). A disabled
// dependency forces Managed=Unknown — managing is meaningless until the
// dependency returns, but the cause is external rather than a controller failure.
func mapManaged(state condmap.State) metav1.Condition {
	if isDependencyDisabled(state) {
		return emit(state, ConditionManaged, metav1.ConditionUnknown, intRequirementsMet)
	}

	chain := lateStage
	if phaseOf(state) == phaseReconcile {
		chain = reconcileChain
	}
	if cond, ok := firstFalse(state, chain); ok {
		return emit(state, ConditionManaged, metav1.ConditionFalse, cond)
	}
	if state.AllIntEqual(metav1.ConditionTrue, intLoaded, intScaled, intHooksProcessed, intManifestsApplied) {
		return emit(state, ConditionManaged, metav1.ConditionTrue, intLoaded)
	}

	return metav1.Condition{}
}

// mapConfigurationApplied tracks whether settings, hooks, and manifests for the
// desired configuration have been applied. During reconcile, a filesystem failure
// leaves the configuration state Unknown — we cannot tell whether the desired
// config is on disk. During an update, early failures don't change what's
// already applied (the old config is still in place). A disabled dependency
// forces Unknown — the desired configuration is no longer being maintained.
func mapConfigurationApplied(state condmap.State) metav1.Condition {
	if isDependencyDisabled(state) {
		return emit(state, ConditionConfigurationApplied, metav1.ConditionUnknown, intRequirementsMet)
	}

	switch phaseOf(state) {
	case phaseInstall:
		if cond, ok := firstFalse(state, configPipeline); ok {
			return emit(state, ConditionConfigurationApplied, metav1.ConditionFalse, cond)
		}
	case phaseUpdate:
		if cond, ok := firstFalse(state, lateStage); ok {
			return emit(state, ConditionConfigurationApplied, metav1.ConditionFalse, cond)
		}
	case phaseReconcile:
		if state.IntEqual(intReadyOnFilesystem, metav1.ConditionFalse) {
			return emit(state, ConditionConfigurationApplied, metav1.ConditionUnknown, intReadyOnFilesystem)
		}
		if cond, ok := firstFalse(state, configPipeline); ok {
			return emit(state, ConditionConfigurationApplied, metav1.ConditionFalse, cond)
		}
	}

	if state.AllIntEqual(metav1.ConditionTrue, intConfigured, intHooksProcessed, intManifestsApplied) {
		return emit(state, ConditionConfigurationApplied, metav1.ConditionTrue, intConfigured)
	}

	return metav1.Condition{}
}
