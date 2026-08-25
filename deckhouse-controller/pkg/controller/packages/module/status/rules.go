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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// External condition types — what the user sees on the Module resource.
const (
	// ConditionEnabled reflects the scheduler's enablement verdict for the
	// module — the folded decision over the explicit intent (spec.enabled or
	// ModuleConfig), the edition bundle, enabled-scripts and module
	// dependencies. True when the module is scheduled to run. False when the
	// scheduler forbids it, with the scheduler's own decision reason
	// (Disabled, DisabledByBundle, DisabledByScript, DependencyNotEnabled,
	// DependencyVersionMismatch, ...). Absent until the first scheduling
	// decision.
	ConditionEnabled = "Enabled"

	// ConditionInstalled reflects the state of the first install of the module.
	// True when the install pipeline completed; False while it is blocked or has failed
	// at one of: waiting for dependent modules to converge (Pending), a scheduler
	// forbid (the scheduler's decision reason, e.g. Disabled, DependencyNotEnabled),
	// download, load from filesystem, settings validation, hooks, or Helm manifest apply.
	// Sticky: once True it is never retracted — subsequent failures surface on
	// UpdateInstalled instead — except when the scheduler switches the module off,
	// which tears the release down (see isDisabled).
	// Possible reasons: Pending, the scheduler's decision reason, DownloadFailed,
	// LoadFromFilesystemFailed, SettingsInvalid, HookInitializationFailed,
	// HookFailed, ManifestsApplyFailed.
	ConditionInstalled = "Installed"

	// ConditionUpdateInstalled reflects the state of installing a new version over
	// a running module. While the update is in progress, the previously
	// installed version may keep working, so Ready/Scaled/ConfigurationApplied
	// /Managed can stay True while UpdateInstalled reports a problem with the new
	// version. False means the update is blocked or has failed.
	// Possible reasons: Pending, DownloadFailed, LoadFromFilesystemFailed,
	// SettingsInvalid, HookInitializationFailed, HookFailed, ManifestsApplyFailed,
	// ApplyingManifests (the new version's manifests are still being applied).
	ConditionUpdateInstalled = "UpdateInstalled"

	// ConditionReady reflects user-facing readiness of the module.
	// On first install it tracks Installed and goes False alongside it on failure.
	// During an update it can stay True while the previous version keeps working.
	// On reconcile it goes False when the running version can no longer be trusted
	// (download, hook, or manifest-apply failures); a settings-only failure does
	// not affect Ready because the running version's settings are unchanged.
	// Possible reasons: Pending, the scheduler's decision reason, DownloadFailed,
	// LoadFromFilesystemFailed, SettingsInvalid, HookInitializationFailed,
	// HookFailed, ManifestsApplyFailed, ApplyingManifests (mid-apply over a
	// non-working previous version), Ready (when True).
	ConditionReady = "Ready"

	// ConditionScaled reflects the runtime scaling state of the module.
	// Owned exclusively by the workload health monitor — no other condition
	// influences this value. True at steady state, False when at least one
	// workload is rolling out (Reconciling) or failed (Degraded), Unknown
	// when there are no workloads to observe yet.
	// Possible reasons: Reconciling (False), Degraded (False), Scaled (True).
	ConditionScaled = "Scaled"

	// ConditionManaged reflects whether the controller is actively managing the
	// module. False means the controller cannot bring the module to
	// (or keep it in) a managed state: typically hook, Helm, or — during reconcile —
	// download failures, where continuing to manage the current state is unsafe.
	// Settings-only failures do not break Managed. Unknown when the scheduler
	// switched the module off under the running release — managing is meaningless
	// until it is enabled again, but the cause is external rather than a
	// controller failure.
	// Possible reasons: the scheduler's decision reason, DownloadFailed,
	// HookInitializationFailed, HookFailed, ManifestsApplyFailed,
	// ApplyingManifests (mid-apply over a non-managed previous version),
	// Managed (when True).
	ConditionManaged = "Managed"

	// ConditionConfigurationApplied reflects whether the desired configuration —
	// settings, render, hooks, manifests — was successfully applied. False on
	// invalid settings, hook errors, or Helm errors. On reconcile a download
	// failure makes the configuration state Unknown (we cannot tell whether the
	// desired config is on disk). A scheduler switch-off under the running
	// release also forces Unknown — the desired configuration is no longer
	// being maintained.
	// Possible reasons: the scheduler's decision reason, DownloadFailed,
	// SettingsInvalid, HookInitializationFailed, HookFailed, ManifestsApplyFailed,
	// ApplyingManifests (the new version's manifests are still being applied),
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
//   - RequirementsMet: the scheduler is the only non-True writer and its
//     decision reasons are already canonical K8s CamelCase (Disabled,
//     DisabledByBundle, DependencyNotEnabled, ...). Pass through — collapsing
//     them would erase why the module is off.
//   - HooksProcessed: the internal reason distinguishes HookInitializationFailed
//     (sync/init phase) from HookFailed (runtime hooks).
//   - ManifestsApplied: ApplyingManifests is a non-failure mid-step indicator
//     and passes through; every other internal reason becomes ManifestsApplyFailed.
func canonicalReason(internalCond, internalReason string) string {
	switch internalCond {
	case intPending:
		return "Pending"
	case intRequirementsMet:
		return internalReason
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

// installPipeline lists every gate from the scheduler verdict to manifests in
// priority order. The other chains are slices into it (so they cannot drift
// apart); reconcileChain combines the filesystem gates with late-stage gates
// because settings failures don't break a running module on reconcile.
var installPipeline = []string{
	intRequirementsMet,   // [0] install only
	intReadyOnFilesystem, // [1] update onwards
	intLoaded,            // [2]
	intConfigured,        // [3] config phase onwards
	intHooksProcessed,    // [4] late stage onwards
	intManifestsApplied,  // [5]
}

var (
	updatePipeline = installPipeline[1:] // scheduler verdict not re-checked on version change
	configPipeline = installPipeline[3:] // settings + hooks + manifests
	lateStage      = installPipeline[4:] // hooks + manifests

	// reconcileChain: gates that break a running module on reconcile — the
	// filesystem gates (download/mount and load) plus the late-stage gates.
	// Settings (Configured) is excluded: an invalid new config does not break
	// the already-running version.
	reconcileChain = []string{intReadyOnFilesystem, intLoaded, intHooksProcessed, intManifestsApplied}
)

// firstFalse returns the first internal condition in chain whose status is False.
// ManifestsApplied=False/ApplyingManifests is progress, not a terminal failure.
func firstFalse(state condmap.State, chain []string) (string, bool) {
	for _, cond := range chain {
		if state.IntEqual(cond, metav1.ConditionFalse) && !isApplyingManifests(state, cond) {
			return cond, true
		}
	}

	return "", false
}

// isApplyingManifests recognises the "manifests are being applied right now"
// state, which the deployer surfaces as ManifestsApplied=False with reason
// ApplyingManifests.
//
// why: ApplyingManifests is a transient progress marker, not a failure. If
// firstFalse treated it like any other False, every reconcile would briefly
// flip mapped conditions (Ready, Managed, ConfigurationApplied, Scaled) to
// False/Unknown during the apply window. That produced visible status flaps
// in -owide and in the UI for a healthy module, so we skip it here.
func isApplyingManifests(state condmap.State, cond string) bool {
	if cond != intManifestsApplied {
		return false
	}

	reason, _ := state.GetIntReason(cond)
	return reason == string(intstatus.ConditionReasonApplyingManifests)
}

// applyingProgress refreshes a mapped condition during an update's manifest-apply
// window. While manifests apply, the True gate is unmet and the update mappers
// return empty, leaving the condition as-is — right when it is already True (the
// previous version still works, don't flap it), wrong when it carries a stale
// failure from an earlier attempt. In that case emit False/ApplyingManifests so it
// tracks the apply. Returns false outside the window or when the condition is
// already True.
func applyingProgress(state condmap.State, ext string) (metav1.Condition, bool) {
	if !isApplyingManifests(state, intManifestsApplied) {
		return metav1.Condition{}, false
	}
	if state.ExtEqual(ext, metav1.ConditionTrue) {
		return metav1.Condition{}, false
	}

	return emit(state, ext, metav1.ConditionFalse, intManifestsApplied), true
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
			mapEnabled,
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

// isDisabled reports whether a previously-installed module has been switched
// off by the scheduler — an explicit user disable, a bundle or enabled-script
// decision, or a lost dependency. The cause is external to the module's own
// pipeline — public conditions reflect that distinction by going to False
// for user-facing signals (Enabled, Installed, Ready) and Unknown for runtime
// and configuration signals (Scaled, ConfigurationApplied, Managed). It
// overrides the Installed stickiness because the disable tears the release
// down — users must see that the module stopped being installed, not a
// silently kept Installed=True.
func isDisabled(state condmap.State) bool {
	return state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) &&
		state.IntEqual(intRequirementsMet, metav1.ConditionFalse)
}

// mapEnabled mirrors the scheduler's enablement verdict carried on the
// internal RequirementsMet condition. Unlike the other mappers it has no
// failure semantics of its own: False is a normal state (the module is
// switched off), so the verdict is reported symmetrically with the
// scheduler's reason passed through. The condition stays absent until the
// scheduler has actually decided (internal RequirementsMet is set Unknown at
// status reset).
func mapEnabled(state condmap.State) metav1.Condition {
	status, ok := state.GetIntStatus(intRequirementsMet)
	if !ok || status == metav1.ConditionUnknown {
		return metav1.Condition{}
	}

	return emit(state, ConditionEnabled, status, intRequirementsMet)
}

// mapInstalled is sticky: once Installed=True it is never retracted, except
// when the scheduler switches the module off under the running release — see
// isDisabled.
func mapInstalled(state condmap.State) metav1.Condition {
	if isDisabled(state) {
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
// an already-installed module. Fires only after Installed=True and either
// an update is in progress or a previous update condition exists. Falls silent
// when the module is switched off — the disabled state is the dominant signal
// and is reported on the other conditions.
func mapUpdateInstalled(state condmap.State) metav1.Condition {
	if isDisabled(state) {
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
	if state.IntEqual(intManifestsApplied, metav1.ConditionTrue) {
		return emit(state, ConditionUpdateInstalled, metav1.ConditionTrue, intManifestsApplied)
	}
	if updating {
		if cond, ok := applyingProgress(state, ConditionUpdateInstalled); ok {
			return cond
		}
	}

	return metav1.Condition{}
}

// mapReady tracks user-facing readiness. Failure chain depends on phase:
//   - install:   any pipeline failure breaks readiness.
//   - update:    only hook/manifest failures (old version still works).
//   - reconcile: filesystem and hook/manifest failures (settings alone do not).
//
// A scheduler switch-off under a running module forces Ready=False regardless
// of phase — the module is no longer working.
func mapReady(state condmap.State) metav1.Condition {
	if isDisabled(state) {
		return emit(state, ConditionReady, metav1.ConditionFalse, intRequirementsMet)
	}

	ph := phaseOf(state)

	var blocker string
	var ok bool

	switch ph {
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
	if ph == phaseUpdate {
		if cond, ok := applyingProgress(state, ConditionReady); ok {
			return cond
		}
	}

	return metav1.Condition{}
}

// mapScaled normally mirrors the workload health monitor, but lifecycle
// failures override it where the public status model needs failure context.
// During first install, Scaled stays absent until the module is actually scaled.
//
// why per phase:
//   - install: Scaled was previously emitted as Unknown when intScaled was
//     missing. For a freshly-created module that briefly produced a
//     Scaled=Unknown row with empty reason in -owide before any other
//     condition appeared, and confused users into thinking the controller
//     had given up. We now suppress the condition entirely until intScaled
//     actually goes True.
//   - update: a hook or manifests failure during update is a workload-level
//     failure as well. We surface that on Scaled (Unknown for hook failures
//     because the workload state is no longer observable, False for
//     ManifestsApplyFailed because the workload itself rejected the new
//     manifests).
//   - reconcile: a filesystem failure makes the runtime state untrustworthy,
//     so Scaled becomes Unknown rather than reporting whatever the health
//     monitor saw last.
func mapScaled(state condmap.State) metav1.Condition {
	if isDisabled(state) {
		return emit(state, ConditionScaled, metav1.ConditionUnknown, intRequirementsMet)
	}

	switch phaseOf(state) {
	case phaseInstall:
		if _, ok := pipelineBlocker(state, installPipeline); ok {
			return metav1.Condition{}
		}
		status, ok := state.GetIntStatus(intScaled)
		if !ok || status != metav1.ConditionTrue {
			return metav1.Condition{}
		}
		return emit(state, ConditionScaled, status, intScaled)
	case phaseUpdate:
		if cond, ok := firstFalse(state, lateStage); ok {
			if cond == intManifestsApplied {
				return emit(state, ConditionScaled, metav1.ConditionFalse, cond)
			}
			return emit(state, ConditionScaled, metav1.ConditionUnknown, cond)
		}
	case phaseReconcile:
		if state.IntEqual(intReadyOnFilesystem, metav1.ConditionFalse) {
			return emit(state, ConditionScaled, metav1.ConditionUnknown, intReadyOnFilesystem)
		}
	}

	status, ok := state.GetIntStatus(intScaled)
	if !ok {
		return metav1.Condition{}
	}

	return emit(state, ConditionScaled, status, intScaled)
}

// mapManaged reports whether the controller can actively manage the module.
// Settings failures never break management; filesystem failures break it only
// during reconcile (the running state is no longer trustworthy). A scheduler
// switch-off forces Managed=Unknown — managing is meaningless until the module
// is enabled again, but the cause is external rather than a controller failure.
func mapManaged(state condmap.State) metav1.Condition {
	if isDisabled(state) {
		return emit(state, ConditionManaged, metav1.ConditionUnknown, intRequirementsMet)
	}

	ph := phaseOf(state)

	chain := lateStage
	switch ph {
	case phaseInstall:
		// why: during the first install a HookInitializationFailed means we
		// never started managing the workload — there is nothing to "stop
		// managing". Emitting Managed=False there would be misleading and
		// would also light up degraded sub-states.
		// Runtime HookFailed during install still flows through (we did start
		// managing), only the init flavour is suppressed.
		if state.IntEqual(intHooksProcessed, metav1.ConditionFalse) {
			reason, _ := state.GetIntReason(intHooksProcessed)
			if canonicalReason(intHooksProcessed, reason) == "HookInitializationFailed" {
				return metav1.Condition{}
			}
		}
	case phaseReconcile:
		chain = reconcileChain
	}

	if cond, ok := firstFalse(state, chain); ok {
		return emit(state, ConditionManaged, metav1.ConditionFalse, cond)
	}
	if state.AllIntEqual(metav1.ConditionTrue, intLoaded, intScaled, intHooksProcessed, intManifestsApplied) {
		return emit(state, ConditionManaged, metav1.ConditionTrue, intLoaded)
	}
	if ph == phaseUpdate {
		if cond, ok := applyingProgress(state, ConditionManaged); ok {
			return cond
		}
	}

	return metav1.Condition{}
}

// mapConfigurationApplied tracks whether settings, hooks, and manifests for the
// desired configuration have been applied. During reconcile, a filesystem failure
// leaves the configuration state Unknown — we cannot tell whether the desired
// config is on disk. During an update, early failures don't change what's
// already applied (the old config is still in place). A scheduler switch-off
// forces Unknown — the desired configuration is no longer being maintained.
func mapConfigurationApplied(state condmap.State) metav1.Condition {
	if isDisabled(state) {
		return emit(state, ConditionConfigurationApplied, metav1.ConditionUnknown, intRequirementsMet)
	}

	ph := phaseOf(state)

	switch ph {
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
	if ph == phaseUpdate {
		if cond, ok := applyingProgress(state, ConditionConfigurationApplied); ok {
			return cond
		}
	}

	return metav1.Condition{}
}
