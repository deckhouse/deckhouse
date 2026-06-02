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
)

// State constants for ApplicationStatus.Summary.State. Every application maps
// to exactly one of these — the Summary always carries a state — so the UI
// never has to reimplement the lifecycle machine on top of the conditions.
//
//   - Pending:   first install has not started or is blocked by an external
//     factor (Installed=False, reason Pending or RequirementsUnmet).
//   - Failed:    no working version is serving — either first install failed
//     (Installed=False, other reason) or an update failed at a late stage
//     (UpdateInstalled=False together with Ready=False).
//   - Updating:  a new version is installing while the old one still serves
//     (UpdateInstalled=False, Ready/Scaled/ConfigurationApplied/Managed True).
//   - Ready:     Installed=True and every other primary condition is True.
//   - Degraded:  Installed=True with a reconcile problem and no active update.
//   - Suspended: a hard dependency was disabled under a running app
//     (Installed=False/RequirementsUnmet with the runtime conditions Unknown,
//     which is what distinguishes it from a first-install Pending).
const (
	statePending   = "Pending"
	stateFailed    = "Failed"
	stateUpdating  = "Updating"
	stateReady     = "Ready"
	stateDegraded  = "Degraded"
	stateSuspended = "Suspended"
)

// advice is the user-facing Summary for one (phase, canonical reason) pair:
// the actionable state, a one-line message and a how-to-solve tip.
type advice struct {
	state   string
	message string
	tip     string
}

// summaryTable is the whole Summary policy in one place: phase → canonical
// reason → what the user sees. It mirrors the scenarios enumerated in
// summary_test.go one-to-one. The phase and the canonical reason both come
// from the mapper's own helpers (phaseOf, the pipeline chains, canonicalReason),
// so there is a single definition of "what phase are we in" and "what failed".
var summaryTable = map[phase]map[string]advice{
	phaseInstall: {
		"Pending": {
			statePending,
			"Installation is waiting for dependent modules to converge",
			"Wait for dependent modules to converge automatically. No action required.",
		},
		"RequirementsUnmet": {
			statePending,
			"Installation is blocked: application requirements are not satisfied",
			"Check the application's spec.requirements: required Deckhouse version or dependent modules do not match the cluster. Update Deckhouse, enable required modules, or adjust requirements.",
		},
		"DownloadFailed": {
			stateFailed,
			"Installation failed: application package could not be downloaded or mounted",
			"Check network connectivity to the registry, verify imagePullSecret and package signature. Fix the issue — the controller will retry on the next reconcile.",
		},
		"LoadFromFilesystemFailed": {
			stateFailed,
			"Installation failed: application package on disk could not be loaded",
			"The on-disk artifact is corrupted or has an invalid structure. Delete the cached package from the node disk and re-pull the image. The controller will retry on the next reconcile.",
		},
		"SettingsInvalid": {
			stateFailed,
			"Installation failed: application settings did not pass validation",
			"Fix the ModuleConfig fields that fail OpenAPI validation. The controller will retry automatically after the config is changed.",
		},
		"HookInitializationFailed": {
			stateFailed,
			"Installation failed: hook synchronization phase failed",
			"Check the hook pod/job logs (kubectl logs). Fix the hook code or its dependencies. Roll back the application version if needed.",
		},
		"HookFailed": {
			stateFailed,
			"Installation failed: startup or runtime hooks failed",
			"Check the failed hook logs. Fix the configuration or hook code. The attempt will be retried on the next reconcile.",
		},
		"ManifestsApplyFailed": {
			stateFailed,
			"Installation failed: Helm could not apply manifests",
			"Check helm history and events in the application namespace. Resolve resource conflicts (namespace, CRD, RBAC). The controller will retry on the next reconcile.",
		},
	},

	// Update entries bake the "stalled vs failed" split into the state field:
	// early-stage blockers (download, load, settings, pending) leave the old
	// version serving (Updating), late-stage ones (hooks, manifests) do not
	// (Failed). This is the same split the old code recovered by reading
	// Ready=False — here it falls out of the reason alone.
	phaseUpdate: {
		"Pending": {
			stateUpdating,
			"Update is waiting for dependent modules to converge; previous version is still serving",
			"Wait — the previous version is still working. The update will continue automatically once dependent modules converge.",
		},
		"DownloadFailed": {
			stateUpdating,
			"Update is stalled: new version could not be downloaded; previous version is still serving",
			"Check registry connectivity, and image integrity. The previous version continues to work. After fixing, the controller will retry the download.",
		},
		"LoadFromFilesystemFailed": {
			stateUpdating,
			"Update is stalled: new version could not be loaded from filesystem; previous version is still serving",
			"Delete the corrupted new version package from the node disk. The previous version continues to work. The controller will retry download and loading.",
		},
		"SettingsInvalid": {
			stateUpdating,
			"Update is stalled: new settings did not pass validation; previous version is still serving",
			"Fix the application settings to match the new version's schema. The previous version continues to work. After fixing, the update will continue.",
		},
		"HookInitializationFailed": {
			stateFailed,
			"Update failed during hook synchronization; previous version is no longer serving",
			"The application is not serving requests. Check the new version's hook logs. Fix the hook/config or manually roll back the application version.",
		},
		"HookFailed": {
			stateFailed,
			"Update failed: startup or runtime hooks of the new version failed; previous version is no longer serving",
			"The application is not serving requests. Check the new version's hook logs. Fix the hook/config or roll back the application version manually.",
		},
		"ManifestsApplyFailed": {
			stateFailed,
			"Update failed: Helm could not apply manifests for the new version; previous version is no longer serving",
			"Resources in the cluster are inconsistent. Check helm history and events in the namespace. Resolve resource conflicts. If needed, roll back manually via helm rollback.",
		},
	},

	// Reconcile degradation reports Degraded — Installed stays True but a
	// problem surfaced with no active update — plus a message and tip.
	phaseReconcile: {
		"DownloadFailed": {
			stateDegraded,
			"Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted",
			"The on-disk artifact has been tampered with or corrupted. Verify integrity, delete and re-fetch the package from the registry. The controller will retry reconcile.",
		},
		"LoadFromFilesystemFailed": {
			stateDegraded,
			"Reconcile failed: application could not be loaded from filesystem; runtime state can no longer be trusted",
			"Delete the corrupted package cache on the node. The controller will retry loading; conditions will be restored based on reconcile progress.",
		},
		"SettingsInvalid": {
			stateDegraded,
			"Reconcile failed: new settings did not pass validation; previously applied configuration is still in effect",
			"The previous configuration continues to work. Fix the invalid fields in the application settings. After saving, the controller will re-apply the settings.",
		},
		"HookInitializationFailed": {
			stateDegraded,
			"Reconcile failed: hook synchronization phase failed; workload remains scaled but is no longer managed",
			"Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		"HookFailed": {
			stateDegraded,
			"Reconcile failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
			"Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		"ManifestsApplyFailed": {
			stateDegraded,
			"Reconcile failed: Helm could not apply manifests",
			"Check events in the namespace and helm history. Resolve resource conflicts (foreign ownership, finalizers, CRD mismatches). The controller will retry reconcile automatically.",
		},
		"Degraded": {
			stateDegraded,
			"Reconcile failed: workload health monitor reports degraded",
			"Workload health monitor reports degraded. Check pod status and logs to identify the root cause.",
		},
		"Reconciling": {
			stateDegraded,
			"Reconcile failed: workload is still reconciling",
			"Workload is still reconciling. Wait for the rollout to complete. If it stays in this state, check pod status and events.",
		},
		// No "Scaled" entry: the health monitor only emits reason "Scaled" with
		// status True (healthy), so it can never be a reconcile blocker. A
		// False intScaled always carries Degraded or Reconciling.
	},
}

// summarySuspended is the fixed Summary for a running app whose hard dependency
// was disabled.
var summarySuspended = advice{
	state:   stateSuspended,
	message: "Application is suspended: requirements unmet",
	tip:     "Solve the application requirements. After it, the controller will automatically restore all conditions and resume operation.",
}

// summaryReady is the fixed Summary for a healthy application: install or update
// completed and every primary condition is True. State alone conveys it, so
// there is no message or tip.
var summaryReady = advice{state: stateReady}

// summaryUpdating is the fixed Summary for an update that is mid-flight with no
// failing gate: the new version's manifests are still being applied
// (ManifestsApplied=False/ApplyingManifests), so the previous version is the
// one serving. It is the positive-progress counterpart to summaryReady — see
// the update branch of summarize for why "no blocker" alone cannot mean Ready.
var summaryUpdating = advice{
	state:   stateUpdating,
	message: "Update in progress: the new version is being applied; the previous version is still serving",
	tip:     "The previous version continues to serve while the new version is applied. No action is required unless this state persists.",
}

// summarize computes the user-facing Summary (state, message, tip) from the
// same condmap.State the mappers consume. It is a pure function — no I/O.
//
// It is fed the pre-mapping state on purpose: isDependencyDisabled reads the
// previous external Installed=True before this run flips it to False, and the
// install-completion check below mirrors mapInstalled's success condition so
// the freshly-installed run reports ready rather than pending.
func summarize(state condmap.State) (string, string, string) {
	// Suspended — dependency disabled under a running app. Shares the mapper's
	// definition exactly, so the two can never drift apart.
	if isDependencyDisabled(state) {
		return summarySuspended.state, summarySuspended.message, summarySuspended.tip
	}

	switch phaseOf(state) {
	case phaseInstall:
		blocker, ok := pipelineBlocker(state, installPipeline)
		if !ok {
			// Pipeline clear: install just completed (mirrors mapInstalled's
			// success check) → ready; otherwise still waiting for dependent
			// modules to converge.
			if state.IntEqual(intScaled, metav1.ConditionTrue) {
				return summaryReady.state, summaryReady.message, summaryReady.tip
			}
			return adviseFor(phaseInstall, "Pending")
		}
		return adviseFor(phaseInstall, reasonOf(state, blocker))

	case phaseUpdate:
		blocker, ok := pipelineBlocker(state, updatePipeline)
		if !ok {
			// "No blocker" is not the same as "update finished". firstFalse skips a
			// transient ManifestsApplied=False/ApplyingManifests exactly as the
			// mapper does, so the update pipeline reads clear during every re-apply
			// window. But mapUpdateInstalled only flips UpdateInstalled (and mapReady
			// only flips Ready) to True once ManifestsApplied is actually True; until
			// then it returns empty and leaves the previous — possibly failed —
			// conditions sticky. Mirror that success gate here, otherwise a mid-apply
			// retry over a failed update would report Ready while the conditions a
			// client also reads still say ManifestsApplyFailed.
			if state.IntEqual(intManifestsApplied, metav1.ConditionTrue) {
				return summaryReady.state, summaryReady.message, summaryReady.tip // update done
			}
			return summaryUpdating.state, summaryUpdating.message, summaryUpdating.tip
		}
		return adviseFor(phaseUpdate, reasonOf(state, blocker))

	case phaseReconcile:
		blocker, ok := firstFalse(state, reconcileSummaryChain)
		if !ok {
			return summaryReady.state, summaryReady.message, summaryReady.tip // healthy steady state
		}
		return adviseFor(phaseReconcile, reasonOf(state, blocker))
	}

	return "", "", ""
}

// reconcileSummaryChain lists the internal gates that degrade a running app,
// in priority order. It mirrors what the mapper actually breaks on reconcile
// so the summary cannot disagree with the conditions: the reconcileChain gates
// (which break Ready/Managed) first, then Configured (which breaks only
// ConfigurationApplied), then Scaled (workload health).
//
// intScaled is last: a real artifact, hook or manifest failure outranks
// workload health. Only a False is a degradation — the health monitor reports
// both Reconciling and Degraded as Scaled=False, while Unknown means "no
// workloads to observe", which is not a problem.
var reconcileSummaryChain = []string{
	intReadyOnFilesystem,
	intLoaded,
	intHooksProcessed,
	intManifestsApplied,
	intConfigured,
	intScaled,
}

// reasonOf returns the canonical external reason for a failing internal
// condition — the same translation the mappers apply via emit/canonicalReason.
func reasonOf(state condmap.State, internalCond string) string {
	intReason, _ := state.GetIntReason(internalCond)
	return canonicalReason(internalCond, intReason)
}

// adviseFor looks up the Summary for a (phase, reason) pair, falling back to a
// generic phrasing when the reason is outside the documented vocabulary (e.g.
// a newly added internal condition that has no tip yet).
func adviseFor(ph phase, reason string) (string, string, string) {
	if a, ok := summaryTable[ph][reason]; ok {
		return a.state, a.message, a.tip
	}

	switch ph {
	case phaseInstall:
		return stateFailed, "Installation failed: " + reason, ""
	case phaseUpdate:
		return stateFailed, "Update failed: " + reason + "; previous version is no longer serving", ""
	case phaseReconcile:
		return stateDegraded, "Reconcile failed: " + reason, ""
	}

	return "", "", ""
}
