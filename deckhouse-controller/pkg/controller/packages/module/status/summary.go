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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// State constants for ModuleStatus.Summary.State. Every module maps to
// exactly one of these — the Summary always carries a state — so the UI
// never has to reimplement the lifecycle machine on top of the conditions.
//
//   - Pending:   first install has not started or is blocked by an external
//     factor (Installed=False with a Pending or scheduler-verdict reason).
//   - Failed:    no working version is running — either first install failed
//     (Installed=False, other reason) or an update failed at a late stage.
//   - Updating:  a new version is installing while the old one still runs.
//   - Configuring: Installed=True and accepted settings no run has applied yet; the module
//     keeps running with the previous ones until the run succeeds.
//   - Ready:     Installed=True and every other primary condition is True.
//   - Degraded:  Installed=True with a reconcile problem and no active update.
//   - Suspended: the scheduler withdrew an installed module — it was disabled
//     or lost a requirement — while its release is still installed; once the
//     disable took it down, Installed=False with the scheduler reason and the
//     state is Pending.
//   - Deleting:  the runtime accepted the removal and is tearing the module
//     down; every condition reports Deleting until the resource disappears.
const (
	statePending     = "Pending"
	stateFailed      = "Failed"
	stateUpdating    = "Updating"
	stateConfiguring = "Configuring"
	stateReady       = "Ready"
	stateDegraded    = "Degraded"
	stateSuspended   = "Suspended"
	stateDeleting    = "Deleting"
)

// Scheduler verdict reasons of the intentional-disable family (see the
// dynamic, bundle and script rules in internal/packages/schedule/rule).
// They are listed here because the Summary distinguishes "the module was
// switched off" from "the module lost a requirement" — the user action
// differs, while both arrive on the same internal RequirementsMet condition.
const (
	reasonDisabled         = "Disabled"
	reasonDisabledByBundle = "DisabledByBundle"
	reasonDisabledByScript = "DisabledByScript"
)

// reasonHealthUnknown is a summary-local table key, not a reason the health monitor
// writes: it stands for an intScaled the monitor has not confirmed, which carries no
// reason of its own.
const reasonHealthUnknown = "HealthUnknown"

// advice is the user-facing Summary for one (phase, reason) pair: the
// actionable state, a one-line message and a how-to-solve tip.
type advice struct {
	state   string
	message string
	tip     string
}

// summaryTable is the whole Summary policy in one place: phase → reason →
// what the user sees. Reasons for the RequirementsMet gate are the scheduler
// verdicts as they are; the rest is the same canonical vocabulary the
// application table uses. The phase and the reason both come from the mappers'
// own helpers (phaseOf, the pipeline chains, the signals' canonical reasons),
// so there is a single definition of "what phase are we in" and "what failed".
var summaryTable = map[phase]map[string]advice{
	phaseInstall: {
		// Scheduler verdicts — the module is blocked, not broken. Which gate
		// blocked it is the whole message, so every verdict has its own row.
		reasonDisabled: {
			statePending,
			"Installation is blocked: the module is disabled",
			"Enable the module to start the installation.",
		},
		reasonDisabledByBundle: {
			statePending,
			"Installation is blocked: the module is disabled by the edition bundle",
			"Enable the module explicitly to override the bundle default.",
		},
		reasonDisabledByScript: {
			statePending,
			"Installation is blocked: the module is disabled by its enabled-script",
			"The enabled-script evaluated to false. Check the cluster state the script depends on, or enable the module explicitly.",
		},
		"EnabledScriptError": {
			stateFailed,
			"Installation failed: the module's enabled-script failed",
			"Check the Deckhouse controller logs for the script error. Fix the script or the cluster state it inspects.",
		},
		"DependencyNotEnabled": {
			statePending,
			"Installation is blocked: a required module is not enabled",
			"Enable the required module listed in the condition message. The installation will continue automatically.",
		},
		"DependencyVersionMismatch": {
			statePending,
			"Installation is blocked: a required module version does not match",
			"Update the required module to a version matching the constraint in the condition message.",
		},
		"AnyOfDependenciesUnmet": {
			statePending,
			"Installation is blocked: none of the alternative required modules is enabled",
			"Enable at least one of the modules listed in the condition message.",
		},
		"NoneOfDependenciesViolated": {
			statePending,
			"Installation is blocked: a conflicting module is enabled",
			"Disable the conflicting module listed in the condition message.",
		},
		model.ReasonRequirementsNotMet: {
			statePending,
			"Installation is blocked: the module requirements are not met",
			"Solve the requirements listed in the condition message. The installation will continue automatically.",
		},
		schedule.ReasonDependentRequirementsUnmet: {
			statePending,
			"Installation is blocked: an enabled module that depends on this one does not accept its version",
			"Update or disable the dependent module listed in the condition message, or choose a version it accepts. The installation will continue automatically.",
		},
		schedule.ReasonDependencyCycle: {
			statePending,
			"Installation is blocked: the module's dependencies would form a cycle",
			"Remove the circular dependency between the modules listed in the condition message. The installation will continue automatically.",
		},
		"VersionLookupFailed": {
			stateFailed,
			"Installation failed: a required module version could not be resolved",
			"Check the required module's state and version. The scheduler will retry automatically.",
		},

		model.ReasonAttachFailed: {
			stateFailed,
			"Installation failed: module package could not be downloaded or mounted",
			"Check network connectivity to the registry, verify imagePullSecret and package signature. Fix the issue — the controller will retry on the next reconcile.",
		},
		model.ReasonLoadFromFilesystemFailed: {
			stateFailed,
			"Installation failed: module package on disk could not be loaded",
			"The on-disk artifact is corrupted or has an invalid structure. Delete the cached package from the node disk and re-pull the image. The controller will retry on the next reconcile.",
		},
		model.ReasonSettingsInvalid: {
			stateFailed,
			"Installation failed: module settings did not pass validation",
			"Fix the ModuleConfig fields that fail OpenAPI validation. The controller will retry automatically after the config is changed.",
		},
		model.ReasonHookInitializationFailed: {
			stateFailed,
			"Installation failed: hook synchronization phase failed",
			"Check the hook pod/job logs (kubectl logs). Fix the hook code or its dependencies. Roll back the module version if needed.",
		},
		model.ReasonHookFailed: {
			stateFailed,
			"Installation failed: startup or runtime hooks failed",
			"Check the failed hook logs. Fix the configuration or hook code. The attempt will be retried on the next reconcile.",
		},
		model.ReasonManifestsApplyFailed: {
			stateFailed,
			"Installation failed: Helm could not apply manifests",
			"Check helm history and events in the d8-system namespace. Resolve resource conflicts (namespace, CRD, RBAC). The controller will retry on the next reconcile.",
		},
		model.ReasonScalingForbidden: {
			stateFailed,
			"Installation failed: the module does not allow the requested replicas",
			"The condition message names the workload. Change or remove its replicas in the module's resource requests; until then the replicas stay as they are.",
		},
	},

	// Update entries bake the "stalled vs failed" split into the state field:
	// blockers before the activation (gates, download, load, settings) leave the
	// old version running (Updating), failures of the activated version's first
	// run do not (Failed). A scheduler reason outside this table is a blocked
	// update too (advise).
	phaseUpdate: {
		model.ReasonWaitingWindow: {
			stateUpdating,
			"Update is waiting for the update window; previous version is still running",
			"No action is required: the update starts in the next update window. Change the update windows or apply the release now to start it earlier.",
		},
		model.ReasonRequirementsNotMet: {
			stateUpdating,
			"Update is blocked: the new version's requirements are not met; previous version is still running",
			"Solve the requirements listed in the condition message. The update will continue automatically.",
		},
		schedule.ReasonDependentRequirementsUnmet: {
			stateUpdating,
			"Update is blocked: an enabled module that depends on this one does not accept the new version; previous version is still running",
			"Update or disable the dependent module listed in the condition message, or choose a version it accepts. The update will continue automatically.",
		},
		schedule.ReasonDependencyCycle: {
			stateUpdating,
			"Update is blocked: the new version's dependencies would form a cycle; previous version is still running",
			"Remove the circular dependency between the modules listed in the condition message. The update will continue automatically.",
		},
		model.ReasonAttachFailed: {
			stateUpdating,
			"Update is stalled: new version could not be downloaded; previous version is still running",
			"Check registry connectivity, and image integrity. The previous version continues to work. After fixing, the controller will retry the download.",
		},
		model.ReasonLoadFromFilesystemFailed: {
			stateUpdating,
			"Update is stalled: new version could not be loaded from filesystem; previous version is still running",
			"Delete the corrupted new version package from the node disk. The previous version continues to work. The controller will retry download and loading.",
		},
		model.ReasonSettingsInvalid: {
			stateUpdating,
			"Update is stalled: new settings did not pass validation; previous version is still running",
			"Fix the module settings to match the new version's schema. The previous version continues to work. After fixing, the update will continue.",
		},
		model.ReasonHookInitializationFailed: {
			stateFailed,
			"Update failed during hook synchronization; previous version is no longer running",
			"The module is not working. Check the new version's hook logs. Fix the hook/config or manually roll back the module version.",
		},
		model.ReasonHookFailed: {
			stateFailed,
			"Update failed: startup or runtime hooks of the new version failed; previous version is no longer running",
			"The module is not working. Check the new version's hook logs. Fix the hook/config or roll back the module version manually.",
		},
		model.ReasonManifestsApplyFailed: {
			stateFailed,
			"Update failed: Helm could not apply manifests for the new version; previous version is no longer running",
			"Resources in the cluster are inconsistent. Check helm history and events in the d8-system namespace. Resolve resource conflicts. If needed, roll back manually via helm rollback.",
		},
		model.ReasonScalingForbidden: {
			stateFailed,
			"Update failed: the new version does not allow the requested replicas",
			"The condition message names the workload. Change or remove its replicas in the module's resource requests; until then the replicas stay as they are.",
		},

		// Workload gate. The manifests are applied, so the update is still in
		// flight rather than failed — except a degraded workload, which is the
		// health monitor's terminal verdict on the new version.
		"Reconciling": {
			stateUpdating,
			"Update applied: the new version's workload is rolling out",
			"Wait for the rollout to finish. If it stalls, check pod status and events: a StatefulSet does not replace a pod that is not Ready, delete such a pod.",
		},
		"Degraded": {
			stateDegraded,
			"Update applied but the workload health monitor reports degraded",
			"Check pod status and logs to identify the root cause. Roll back the module version if the new one cannot start.",
		},
		reasonHealthUnknown: {
			stateUpdating,
			"Update applied: waiting for a workload the health monitor can confirm",
			"Either no report has arrived yet, or the chart ships no Deployment or StatefulSet, the only kinds the health monitor watches.",
		},
	},

	// Reconcile degradation reports Degraded — Installed stays True but a
	// problem surfaced with no active update — plus a message and tip.
	phaseReconcile: {
		model.ReasonSettingsInvalid: {
			stateDegraded,
			"Reconcile failed: new settings did not pass validation; previously applied configuration is still in effect",
			"The previous configuration continues to work. Fix the invalid fields in the module settings. After saving, the controller will re-apply the settings.",
		},
		model.ReasonHookInitializationFailed: {
			stateDegraded,
			"Reconcile failed: hook synchronization phase failed; workload remains scaled but is no longer managed",
			"Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		model.ReasonHookFailed: {
			stateDegraded,
			"Reconcile failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
			"Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		model.ReasonManifestsApplyFailed: {
			stateDegraded,
			"Reconcile failed: Helm could not apply manifests",
			"Check events in the d8-system namespace and helm history. Resolve resource conflicts (foreign ownership, finalizers, CRD mismatches). The controller will retry reconcile automatically.",
		},
		model.ReasonScalingForbidden: {
			stateDegraded,
			"Reconcile failed: the module does not allow the requested replicas; previously applied manifests stay in place",
			"The condition message names the workload. Change or remove its replicas in the module's resource requests; until then the replicas stay as they are.",
		},
		"Degraded": {
			stateDegraded,
			"Reconcile failed: workload health monitor reports degraded",
			"Workload health monitor reports degraded. Check pod status and logs to identify the root cause.",
		},
		"Reconciling": {
			stateDegraded,
			"Reconcile failed: workload is still reconciling",
			"Workload is still reconciling. Wait for the rollout to complete. If it stays in this state, check pod status and events: a StatefulSet does not replace a pod that is not Ready, delete such a pod.",
		},
		// No "Scaled" entry: the health monitor only emits reason "Scaled" with
		// status True (healthy), so it can never be a reconcile blocker. A
		// False intScaled always carries Degraded or Reconciling.
	},
}

// summarySuspendedDisabled is the fixed Summary for a running module that was
// switched off — by the user, the bundle or the enabled-script.
var summarySuspendedDisabled = advice{
	state:   stateSuspended,
	message: "Module is suspended: the module was disabled",
	tip:     "Enable the module to resume. The controller will restore all conditions and resume operation automatically.",
}

// summarySuspendedRequirements is the fixed Summary for a running module that
// lost a requirement — a dependency was disabled or stopped matching.
var summarySuspendedRequirements = advice{
	state:   stateSuspended,
	message: "Module is suspended: requirements unmet",
	tip:     "Solve the module requirements. After it, the controller will automatically restore all conditions and resume operation.",
}

// summaryDeleting is the fixed Summary for a module the runtime is tearing down.
var summaryDeleting = advice{
	state:   stateDeleting,
	message: "Module is being deleted",
	tip:     "No action is required. The resource disappears once its release is taken down.",
}

// summaryReady is the fixed Summary for a healthy module: install or update
// completed and every primary condition is True. State alone conveys it, so
// there is no message or tip.
var summaryReady = advice{state: stateReady}

// summaryConfiguring is the fixed Summary for an installed module whose accepted settings no run
// has applied yet: the run applying them is ahead or in flight.
var summaryConfiguring = advice{
	state:   stateConfiguring,
	message: "Configuration in progress: new settings are being applied",
	tip:     "The module keeps working with the previous settings until the new ones are applied. No action is required unless this state persists.",
}

// summaryInstalling is the fixed Summary for a first install whose pipeline is
// running with no failing gate: the module left Pending, and nothing is installed yet.
var summaryInstalling = advice{
	state:   statePending,
	message: "Installation is in progress",
	tip:     "No action is required unless this state persists.",
}

// summaryUpdating is the fixed Summary for an update that is mid-flight with no
// failing gate: the new version's manifests are still being applied
// (ManifestsApplied=False/ApplyingManifests), so the previous version is the
// one running. It is the positive-progress counterpart to summaryReady — see
// the update branch of summarize for why "no blocker" alone cannot mean Ready.
var summaryUpdating = advice{
	state:   stateUpdating,
	message: "Update in progress: the new version is being applied; the previous version is still running",
	tip:     "The previous version continues to run while the new version is applied. No action is required unless this state persists.",
}

// summarize computes the user-facing Summary from the same facts the
// mappers read. It is a pure function — no I/O.
//
// It reads the conditions on the resource from before this projection on purpose: isSuspended
// reads the previous Installed=True, and the install-completion check mirrors mapInstalled's
// success condition so the freshly-installed run reports ready.
func summarize(f *facts) advice {
	// Deleting outranks every other signal: the conditions still describe the last
	// reconcile, so reading them would report a problem the user cannot act on.
	if f.deleting {
		a := summaryDeleting
		if f.removeError != "" {
			a.message += ": " + f.removeError
		}

		// The uninstall's own failure, whatever operation wrote it: a stale run failure shows
		// for the moments until the uninstall reports.
		if s := firstFalse(f.lateStage()); s != nil {
			a.message += ": " + s.reason + ": " + s.message
		}

		return a
	}

	// Suspended — the scheduler withdrew an installed module; its disable may
	// still be running, and a failing one is named.
	if isSuspended(f) {
		a := suspendedFor(f)
		if s := firstFalse(f.lateStage()); s != nil {
			a.message += "; the disable is failing: " + s.reason + ": " + s.message
		}

		return a
	}

	switch phaseOf(f) {
	case phaseInstall:
		blocker := firstFalse(f.installPipeline())
		if blocker == nil {
			if isInstallComplete(f) {
				return summaryReady
			}
			return summaryInstalling
		}
		// A scheduler verdict blocks the module, it does not break it.
		return advise(phaseInstall, blocker.reason, blocker == &f.requirements)

	case phaseUpdate:
		blocker := firstFalse(f.updatePipeline())
		if blocker == nil {
			// No blocker is not "update finished": a candidate is still being
			// prepared, or the activated version has not succeeded yet.
			if f.candidate || !f.manifests.isTrue() {
				return summaryUpdating
			}
			// The manifests landed, but only a positive health report makes the new
			// version ready; anything else is still an update in flight.
			if !f.scaled.isTrue() {
				return advise(phaseUpdate, workloadReason(f), false)
			}
			return summaryReady
		}
		return advise(phaseUpdate, blocker.reason, blocker == &f.updateGate)

	case phaseReconcile:
		blocker := firstFalse(f.reconcileSummaryChain())
		if blocker == nil {
			if f.settingsChanged {
				return summaryConfiguring
			}

			return summaryReady // healthy steady state
		}
		return advise(phaseReconcile, blocker.reason, false)
	}

	return advice{}
}

// suspendedFor picks the Suspended wording by the scheduler verdict family:
// the intentional-disable reasons get the "was disabled" advice, everything
// else reads as unmet requirements.
func suspendedFor(f *facts) advice {
	switch f.requirements.reason {
	case reasonDisabled, reasonDisabledByBundle, reasonDisabledByScript:
		return summarySuspendedDisabled
	}

	return summarySuspendedRequirements
}

// reconcileSummaryChain lists the internal gates that degrade a running
// module, in priority order: hook and manifest failures, then settings, then
// workload health. Only a False Scaled is a degradation.
func (f *facts) reconcileSummaryChain() []*signal {
	return []*signal{&f.hooks, &f.manifests, &f.configured, &f.scaled}
}

// workloadReason returns the health monitor's verdict on the module workload.
// A scaled signal without a reason means the monitor has not reported at all.
func workloadReason(f *facts) string {
	if f.scaled.reason == "" {
		return reasonHealthUnknown
	}

	return f.scaled.reason
}

// advise looks up the Summary of a (phase, reason) pair. A reason outside the table gets a
// generic phrasing: a blocker (a scheduler verdict, an update gate) blocks the module, any other
// failure breaks it.
func advise(ph phase, reason string, blocked bool) advice {
	if a, ok := summaryTable[ph][reason]; ok {
		return a
	}

	switch {
	case ph == phaseInstall && blocked:
		return advice{state: statePending, message: "Installation is blocked: " + reason}
	case ph == phaseInstall:
		return advice{state: stateFailed, message: "Installation failed: " + reason}
	case ph == phaseUpdate && blocked:
		return advice{state: stateUpdating, message: "Update is blocked: " + reason + "; previous version is still running"}
	case ph == phaseUpdate:
		return advice{state: stateFailed, message: "Update failed: " + reason + "; previous version is no longer running"}
	default:
		return advice{state: stateDegraded, message: "Reconcile failed: " + reason}
	}
}
