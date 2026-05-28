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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// summaryFor builds the pre-mapping state from the given options and runs
// summarize on it — the same state the mapper consumes in service.go.
func summaryFor(opts ...mappingOption) (string, string, string) {
	state := condmap.State{
		Internal: make(map[string]metav1.Condition),
		External: make(map[string]metav1.Condition),
	}
	for _, opt := range opts {
		opt(&state)
	}

	return summarize(state)
}

// installed marks the app as previously installed (sticky external condition),
// which puts summarize into the update or reconcile phase.
func installed() mappingOption {
	return withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed")
}

func intCond(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return withInternalCondition(cond, status, reason)
}

func TestSummarize_All22Scenarios(t *testing.T) {
	tests := []struct {
		name    string
		opts    []mappingOption
		state   string
		message string
		tip     string
	}{
		// ── Install (not yet installed) ────────────────────────────────

		{
			name:    "1. Install: waiting for dependent modules to converge",
			opts:    []mappingOption{intCond(intPending, metav1.ConditionTrue, "Waiting")},
			state:   statePending,
			message: "Installation is waiting for dependent modules to converge",
			tip:     "Wait for dependent modules to converge automatically. No action required.",
		},
		{
			name:    "2. Install: requirements unmet",
			opts:    []mappingOption{intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled")},
			state:   statePending,
			message: "Installation is blocked: module requirements are not satisfied",
			tip:     "Check the module's spec.requirements: required Deckhouse version or dependent modules do not match the cluster. Update Deckhouse, enable required modules, or adjust requirements.",
		},
		{
			name:    "3. Install: download/mount failed",
			opts:    []mappingOption{intCond(intReadyOnFilesystem, metav1.ConditionFalse, "MountFailed")},
			state:   stateFailed,
			message: "Installation failed: module package could not be downloaded or mounted",
			tip:     "Check network connectivity to the registry, verify imagePullSecret and package signature. Fix the issue — the controller will retry on the next reconcile.",
		},
		{
			name:    "4. Install: load from filesystem failed",
			opts:    []mappingOption{intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")},
			state:   stateFailed,
			message: "Installation failed: module package on disk could not be loaded",
			tip:     "The on-disk artifact is corrupted or has an invalid structure. Delete the cached package from the node disk and re-pull the image. The controller will retry on the next reconcile.",
		},
		{
			name:    "5. Install: invalid settings",
			opts:    []mappingOption{intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")},
			state:   stateFailed,
			message: "Installation failed: module settings did not pass validation",
			tip:     "Fix the ModuleConfig fields that fail OpenAPI validation. The controller will retry automatically after the config is changed.",
		},
		{
			name:    "6. Install: hook sync phase failed",
			opts:    []mappingOption{intCond(intHooksProcessed, metav1.ConditionFalse, "HookInitializationFailed")},
			state:   stateFailed,
			message: "Installation failed: hook synchronization phase failed",
			tip:     "Check the hook pod/job logs (kubectl logs). Fix the hook code or its dependencies. Roll back the module version if needed.",
		},
		{
			name:    "7. Install: startup/runtime hooks failed",
			opts:    []mappingOption{intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")},
			state:   stateFailed,
			message: "Installation failed: startup or runtime hooks failed",
			tip:     "Check the failed hook logs. Fix the configuration or hook code. The attempt will be retried on the next reconcile.",
		},
		{
			name:    "8. Install: Helm apply failed",
			opts:    []mappingOption{intCond(intManifestsApplied, metav1.ConditionFalse, "boom")},
			state:   stateFailed,
			message: "Installation failed: Helm could not apply manifests",
			tip:     "Check helm history and events in the module namespace. Resolve resource conflicts (namespace, CRD, RBAC). The controller will retry on the next reconcile.",
		},

		// ── Update (installed, version change in progress) ────────────

		{
			name:    "9. Update: waiting for dependent modules to converge",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intPending, metav1.ConditionTrue, "Waiting")},
			state:   stateUpdating,
			message: "Update is waiting for dependent modules to converge; previous version is still serving",
			tip:     "Wait — the previous version is still working. The update will continue automatically once dependent modules converge.",
		},
		{
			name:    "10. Update: download/mount failed, old version serving",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intReadyOnFilesystem, metav1.ConditionFalse, "MountFailed")},
			state:   stateUpdating,
			message: "Update is stalled: new version could not be downloaded; previous version is still serving",
			tip:     "Check registry connectivity, imagePullSecret, and network. The previous version continues to work. After fixing, the controller will retry the download.",
		},
		{
			name:    "11. Update: load from filesystem failed, old version serving",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")},
			state:   stateUpdating,
			message: "Update is stalled: new version could not be loaded from filesystem; previous version is still serving",
			tip:     "Delete the corrupted new version package from the node disk. The previous version continues to work. The controller will retry download and loading.",
		},
		{
			name:    "12. Update: invalid settings, old version serving",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")},
			state:   stateUpdating,
			message: "Update is stalled: new settings did not pass validation; previous version is still serving",
			tip:     "Fix the ModuleConfig to match the new version's schema. The previous version continues to work. After fixing, the update will continue.",
		},
		{
			name:    "13. Update: hook sync failed, old version down",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intHooksProcessed, metav1.ConditionFalse, "HookInitializationFailed")},
			state:   stateFailed,
			message: "Update failed during hook synchronization; previous version is no longer serving",
			tip:     "The application is not serving requests. Check the new version's hook logs. Fix the hook/config or manually roll back the module version.",
		},
		{
			name:    "14. Update: startup/runtime hooks failed, old version down",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")},
			state:   stateFailed,
			message: "Update failed: startup or runtime hooks of the new version failed; previous version is no longer serving",
			tip:     "The application is not serving requests. Check the new version's hook logs. Fix the hook/config or roll back the module version manually.",
		},
		{
			name:    "15. Update: Helm apply failed",
			opts:    []mappingOption{installed(), withVersionChanged(), intCond(intManifestsApplied, metav1.ConditionFalse, "boom")},
			state:   stateFailed,
			message: "Update failed: Helm could not apply manifests for the new version; previous version is no longer serving",
			tip:     "Resources in the cluster are inconsistent. Check helm history and events in the namespace. Resolve resource conflicts. If needed, roll back manually via helm rollback.",
		},

		// ── Reconcile (installed, no active update) ───────────────────

		{
			// Healthy steady state — Ready with no message or tip.
			name:    "16. Reconcile: all working",
			opts:    append([]mappingOption{installed()}, withSuccessfulApply()...),
			state:   stateReady,
			message: "",
			tip:     "",
		},
		{
			name:    "17. Reconcile: artifact tampered (download verification failed)",
			opts:    []mappingOption{installed(), intCond(intReadyOnFilesystem, metav1.ConditionFalse, "VerificationFailed")},
			state:   stateDegraded,
			message: "Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted",
			tip:     "The on-disk artifact has been tampered with or corrupted. Verify integrity, delete and re-fetch the package from the registry. The controller will retry reconcile.",
		},
		{
			name:    "18. Reconcile: load from filesystem failed",
			opts:    []mappingOption{installed(), intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")},
			state:   stateDegraded,
			message: "Reconcile failed: module could not be loaded from filesystem; runtime state can no longer be trusted",
			tip:     "Delete the corrupted package cache on the node. The controller will retry loading; conditions will be restored based on reconcile progress.",
		},
		{
			name:    "19. Reconcile: invalid settings",
			opts:    []mappingOption{installed(), intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")},
			state:   stateDegraded,
			message: "Reconcile failed: new settings did not pass validation; previously applied configuration is still in effect",
			tip:     "The previous configuration continues to work. Fix the invalid fields in ModuleConfig. After saving, the controller will re-apply the settings.",
		},
		{
			name:    "20. Reconcile: startup/runtime hooks failed",
			opts:    []mappingOption{installed(), intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")},
			state:   stateDegraded,
			message: "Reconcile failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
			tip:     "Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		{
			name:    "21. Reconcile: Helm apply failed",
			opts:    []mappingOption{installed(), intCond(intManifestsApplied, metav1.ConditionFalse, "boom")},
			state:   stateDegraded,
			message: "Reconcile failed: Helm could not apply manifests",
			tip:     "Check events in the namespace and helm history. Resolve resource conflicts (foreign ownership, finalizers, CRD mismatches). The controller will retry reconcile automatically.",
		},

		// ── Suspended (dependency disabled under a running app) ───────

		{
			name: "22. Dependency disabled",
			opts: []mappingOption{
				installed(),
				intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"),
				intCond(intReadyOnFilesystem, metav1.ConditionTrue, "Mounted"),
				intCond(intLoaded, metav1.ConditionTrue, "Loaded"),
				intCond(intConfigured, metav1.ConditionTrue, "ConfigOK"),
				intCond(intHooksProcessed, metav1.ConditionTrue, "HooksOK"),
				intCond(intManifestsApplied, metav1.ConditionTrue, "ManifestsOK"),
				intCond(intScaled, metav1.ConditionTrue, "Ready"),
			},
			state:   stateSuspended,
			message: "Module is suspended: a required dependency has been disabled",
			tip:     "Enable the disabled dependent module back. After it converges, the controller will automatically restore all conditions and resume operation.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, message, tip := summaryFor(tt.opts...)
			if state != tt.state {
				t.Errorf("state = %q, want %q", state, tt.state)
			}
			if message != tt.message {
				t.Errorf("message = %q, want %q", message, tt.message)
			}
			if tip != tt.tip {
				t.Errorf("tip = %q, want %q", tip, tt.tip)
			}
		})
	}
}

func TestSummarize_EdgeCases(t *testing.T) {
	t.Run("no conditions yet — pending install", func(t *testing.T) {
		state, message, tip := summaryFor()
		if state != statePending {
			t.Errorf("state = %q, want %q", state, statePending)
		}
		if message != "Installation is waiting for dependent modules to converge" {
			t.Errorf("message = %q", message)
		}
		if tip != "Wait for dependent modules to converge automatically. No action required." {
			t.Errorf("tip = %q", tip)
		}
	})

	t.Run("pipeline clear and scaled — install just completed, ready", func(t *testing.T) {
		// Not yet installed externally, but every internal gate is True and the
		// workload is scaled: this is the run install completes on. Mirrors
		// mapInstalled's success check — the app is Ready.
		state, message, tip := summaryFor(withSuccessfulApply()...)
		if state != stateReady {
			t.Errorf("state = %q, want %q", state, stateReady)
		}
		if message != "" || tip != "" {
			t.Errorf("got message=%q tip=%q, want both empty", message, tip)
		}
	})

	t.Run("workload degraded during reconcile", func(t *testing.T) {
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionFalse, "Degraded"))
		if state != stateDegraded {
			t.Errorf("state = %q, want %q", state, stateDegraded)
		}
		if message != "Reconcile failed: workload health monitor reports degraded" {
			t.Errorf("message = %q", message)
		}
		if tip != "Workload health monitor reports degraded. Check pod status and logs to identify the root cause." {
			t.Errorf("tip = %q", tip)
		}
	})

	t.Run("workload reconciling during reconcile", func(t *testing.T) {
		// The health monitor reports a rollout as Scaled=False/Reconciling.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionFalse, "Reconciling"))
		if state != stateDegraded {
			t.Errorf("state = %q, want %q", state, stateDegraded)
		}
		if message != "Reconcile failed: workload is still reconciling" {
			t.Errorf("message = %q", message)
		}
		if tip != "Workload is still reconciling. Wait for the rollout to complete. If it stays in this state, check pod status and events." {
			t.Errorf("tip = %q", tip)
		}
	})

	t.Run("scaled unknown (no workloads to observe) is ready, not degraded", func(t *testing.T) {
		// Unknown is the health monitor's "nothing to observe" signal with an
		// empty reason — it is not a degradation.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionUnknown, ""))
		if state != stateReady {
			t.Errorf("state = %q, want %q", state, stateReady)
		}
		if message != "" || tip != "" {
			t.Errorf("got message=%q tip=%q, want both empty", message, tip)
		}
	})

	t.Run("unknown reconcile reason falls back to generic phrasing", func(t *testing.T) {
		// The health monitor is the only writer that can produce an arbitrary
		// reason (it passes through), so it exercises the defensive fallback.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionFalse, "WeirdError"))
		if state != stateDegraded {
			t.Errorf("state = %q, want %q", state, stateDegraded)
		}
		if message != "Reconcile failed: WeirdError" {
			t.Errorf("message = %q", message)
		}
		if tip != "" {
			t.Errorf("tip = %q, want empty", tip)
		}
	})

	t.Run("manifests applying on a healthy app is ready, not degraded", func(t *testing.T) {
		// ManifestsApplied=False/ApplyingManifests is a transient progress
		// marker, not a failure: firstFalse skips it so a healthy app does not
		// flap to Degraded during every apply window.
		opts := append([]mappingOption{installed()}, withSuccessfulApply()...)
		opts = append(opts, intCond(intManifestsApplied, metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)))

		state, message, tip := summaryFor(opts...)
		if state != stateReady {
			t.Errorf("state = %q, want %q", state, stateReady)
		}
		if message != "" || tip != "" {
			t.Errorf("got message=%q tip=%q, want both empty", message, tip)
		}
	})

	t.Run("real failure outranks degraded workload", func(t *testing.T) {
		// A filesystem failure and a degraded workload at once: intScaled is the
		// lowest-priority gate, so the artifact failure wins.
		state, message, _ := summaryFor(installed(),
			intCond(intReadyOnFilesystem, metav1.ConditionFalse, "VerificationFailed"),
			intCond(intScaled, metav1.ConditionFalse, "Degraded"))
		if state != stateDegraded {
			t.Errorf("state = %q, want %q", state, stateDegraded)
		}
		if message != "Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted" {
			t.Errorf("message = %q", message)
		}
	})
}

func TestSummarize_SuspendedVsPending(t *testing.T) {
	t.Run("suspended when previously installed and requirements drop", func(t *testing.T) {
		state, message, _ := summaryFor(installed(), intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"))
		if state != stateSuspended {
			t.Errorf("state = %q, want %q", state, stateSuspended)
		}
		if message != "Module is suspended: a required dependency has been disabled" {
			t.Errorf("message = %q", message)
		}
	})

	t.Run("pending when requirements unmet on first install", func(t *testing.T) {
		// No external Installed=True: this is a first install blocked on
		// requirements, not a running app that lost a dependency.
		state, message, _ := summaryFor(intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"))
		if state != statePending {
			t.Errorf("state = %q, want %q (Pending)", state, statePending)
		}
		if message == "Module is suspended: a required dependency has been disabled" {
			t.Errorf("unexpected suspended message on first install")
		}
	})
}
