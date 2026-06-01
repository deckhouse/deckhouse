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

	"github.com/stretchr/testify/assert"
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
// which puts the mapper and summarize into the update or reconcile phase.
func installed() mappingOption {
	return withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed")
}

func intCond(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return withInternalCondition(cond, status, reason)
}

// running is a previously-installed app with every internal gate True; the
// overrides (applied last) introduce the fault under test.
func running(overrides ...mappingOption) []mappingOption {
	opts := append([]mappingOption{installed()}, withSuccessfulApply()...)
	return append(opts, overrides...)
}

// updatingApp is a running app with a version change in progress.
func updatingApp(overrides ...mappingOption) []mappingOption {
	return running(append([]mappingOption{withVersionChanged()}, overrides...)...)
}

// TestLifecycleScenarios drives one internal state through BOTH the mapper and
// summarize, asserting the external conditions and the summary together. This
// is what guarantees the summary can never disagree with the conditions a
// client also sees — they are derived from the same state by shared helpers.
func TestLifecycleScenarios(t *testing.T) {
	cases := []struct {
		name      string
		opts      []mappingOption
		wantConds map[string]*expectedCondition // nil value asserts the condition is absent
		state     string
		message   string
		tip       string
	}{
		// ── Install (not yet installed) ────────────────────────────────

		{
			name: "install: waiting for dependent modules",
			opts: []mappingOption{intCond(intPending, metav1.ConditionTrue, "Waiting")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "Pending"},
				ConditionReady:     {metav1.ConditionFalse, "Pending"},
				ConditionScaled:    nil,
			},
			state:   statePending,
			message: "Installation is waiting for dependent modules to converge",
			tip:     "Wait for dependent modules to converge automatically. No action required.",
		},
		{
			name: "install: requirements unmet",
			opts: []mappingOption{intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "RequirementsUnmet"},
				ConditionReady:     {metav1.ConditionFalse, "RequirementsUnmet"},
			},
			state:   statePending,
			message: "Installation is blocked: application requirements are not satisfied",
			tip:     "Check the application's spec.requirements: required Deckhouse version or dependent modules do not match the cluster. Update Deckhouse, enable required modules, or adjust requirements.",
		},
		{
			name: "install: download/mount failed",
			opts: []mappingOption{intCond(intReadyOnFilesystem, metav1.ConditionFalse, "MountFailed")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "DownloadFailed"},
				ConditionReady:     {metav1.ConditionFalse, "DownloadFailed"},
			},
			state:   stateFailed,
			message: "Installation failed: application package could not be downloaded or mounted",
			tip:     "Check network connectivity to the registry, verify imagePullSecret and package signature. Fix the issue — the controller will retry on the next reconcile.",
		},
		{
			name: "install: load from filesystem failed",
			opts: []mappingOption{intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "LoadFromFilesystemFailed"},
				ConditionReady:     {metav1.ConditionFalse, "LoadFromFilesystemFailed"},
			},
			state:   stateFailed,
			message: "Installation failed: application package on disk could not be loaded",
			tip:     "The on-disk artifact is corrupted or has an invalid structure. Delete the cached package from the node disk and re-pull the image. The controller will retry on the next reconcile.",
		},
		{
			name: "install: invalid settings",
			opts: []mappingOption{intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled:            {metav1.ConditionFalse, "SettingsInvalid"},
				ConditionReady:                {metav1.ConditionFalse, "SettingsInvalid"},
				ConditionConfigurationApplied: {metav1.ConditionFalse, "SettingsInvalid"},
			},
			state:   stateFailed,
			message: "Installation failed: application settings did not pass validation",
			tip:     "Fix the ModuleConfig fields that fail OpenAPI validation. The controller will retry automatically after the config is changed.",
		},
		{
			name: "install: hook sync phase failed",
			opts: []mappingOption{intCond(intHooksProcessed, metav1.ConditionFalse, "HookInitializationFailed")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "HookInitializationFailed"},
				ConditionReady:     {metav1.ConditionFalse, "HookInitializationFailed"},
				// Managed is suppressed on first-install hook-init failure:
				// nothing was ever managed.
				ConditionManaged: nil,
			},
			state:   stateFailed,
			message: "Installation failed: hook synchronization phase failed",
			tip:     "Check the hook pod/job logs (kubectl logs). Fix the hook code or its dependencies. Roll back the application version if needed.",
		},
		{
			name: "install: startup/runtime hooks failed",
			opts: []mappingOption{intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "HookFailed"},
				ConditionReady:     {metav1.ConditionFalse, "HookFailed"},
				ConditionManaged:   {metav1.ConditionFalse, "HookFailed"},
			},
			state:   stateFailed,
			message: "Installation failed: startup or runtime hooks failed",
			tip:     "Check the failed hook logs. Fix the configuration or hook code. The attempt will be retried on the next reconcile.",
		},
		{
			name: "install: Helm apply failed",
			opts: []mappingOption{intCond(intManifestsApplied, metav1.ConditionFalse, "boom")},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "ManifestsApplyFailed"},
				ConditionReady:     {metav1.ConditionFalse, "ManifestsApplyFailed"},
				ConditionManaged:   {metav1.ConditionFalse, "ManifestsApplyFailed"},
			},
			state:   stateFailed,
			message: "Installation failed: Helm could not apply manifests",
			tip:     "Check helm history and events in the application namespace. Resolve resource conflicts (namespace, CRD, RBAC). The controller will retry on the next reconcile.",
		},

		// ── Update (installed, version change in progress) ────────────
		// Early-stage faults leave the old version serving (Ready/Scaled True);
		// late-stage faults take it down.

		{
			name: "update: waiting for dependent modules, old version serving",
			opts: updatingApp(intCond(intPending, metav1.ConditionTrue, "Waiting")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "Pending"},
				ConditionReady:           {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:          {metav1.ConditionTrue, ConditionScaled},
			},
			state:   stateUpdating,
			message: "Update is waiting for dependent modules to converge; previous version is still serving",
			tip:     "Wait — the previous version is still working. The update will continue automatically once dependent modules converge.",
		},
		{
			name: "update: download failed, old version serving",
			opts: updatingApp(intCond(intReadyOnFilesystem, metav1.ConditionFalse, "MountFailed")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "DownloadFailed"},
				ConditionReady:           {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:          {metav1.ConditionTrue, ConditionScaled},
			},
			state:   stateUpdating,
			message: "Update is stalled: new version could not be downloaded; previous version is still serving",
			tip:     "Check registry connectivity, and image integrity. The previous version continues to work. After fixing, the controller will retry the download.",
		},
		{
			name: "update: load from filesystem failed, old version serving",
			opts: updatingApp(intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "LoadFromFilesystemFailed"},
				ConditionReady:           {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:          {metav1.ConditionTrue, ConditionScaled},
			},
			state:   stateUpdating,
			message: "Update is stalled: new version could not be loaded from filesystem; previous version is still serving",
			tip:     "Delete the corrupted new version package from the node disk. The previous version continues to work. The controller will retry download and loading.",
		},
		{
			name: "update: invalid settings, old version serving",
			opts: updatingApp(intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "SettingsInvalid"},
				ConditionReady:           {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:          {metav1.ConditionTrue, ConditionScaled},
			},
			state:   stateUpdating,
			message: "Update is stalled: new settings did not pass validation; previous version is still serving",
			tip:     "Fix the application settings to match the new version's schema. The previous version continues to work. After fixing, the update will continue.",
		},
		{
			name: "update: hook sync failed, old version down",
			opts: updatingApp(intCond(intHooksProcessed, metav1.ConditionFalse, "HookInitializationFailed")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "HookInitializationFailed"},
				ConditionReady:           {metav1.ConditionFalse, "HookInitializationFailed"},
				ConditionScaled:          {metav1.ConditionUnknown, "HookInitializationFailed"},
			},
			state:   stateFailed,
			message: "Update failed during hook synchronization; previous version is no longer serving",
			tip:     "The application is not serving requests. Check the new version's hook logs. Fix the hook/config or manually roll back the application version.",
		},
		{
			name: "update: startup/runtime hooks failed, old version down",
			opts: updatingApp(intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "HookFailed"},
				ConditionReady:           {metav1.ConditionFalse, "HookFailed"},
				ConditionScaled:          {metav1.ConditionUnknown, "HookFailed"},
			},
			state:   stateFailed,
			message: "Update failed: startup or runtime hooks of the new version failed; previous version is no longer serving",
			tip:     "The application is not serving requests. Check the new version's hook logs. Fix the hook/config or roll back the application version manually.",
		},
		{
			name: "update: Helm apply failed",
			opts: updatingApp(intCond(intManifestsApplied, metav1.ConditionFalse, "boom")),
			wantConds: map[string]*expectedCondition{
				ConditionUpdateInstalled: {metav1.ConditionFalse, "ManifestsApplyFailed"},
				ConditionReady:           {metav1.ConditionFalse, "ManifestsApplyFailed"},
				ConditionScaled:          {metav1.ConditionFalse, "ManifestsApplyFailed"},
			},
			state:   stateFailed,
			message: "Update failed: Helm could not apply manifests for the new version; previous version is no longer serving",
			tip:     "Resources in the cluster are inconsistent. Check helm history and events in the namespace. Resolve resource conflicts. If needed, roll back manually via helm rollback.",
		},

		// ── Reconcile (installed, no active update) ───────────────────

		{
			name: "reconcile: all working",
			opts: running(),
			wantConds: map[string]*expectedCondition{
				ConditionReady:                {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:               {metav1.ConditionTrue, ConditionScaled},
				ConditionManaged:              {metav1.ConditionTrue, ConditionManaged},
				ConditionConfigurationApplied: {metav1.ConditionTrue, ConditionConfigurationApplied},
			},
			state:   stateReady,
			message: "",
			tip:     "",
		},
		{
			name: "reconcile: artifact verification failed",
			opts: []mappingOption{installed(), intCond(intReadyOnFilesystem, metav1.ConditionFalse, "VerificationFailed")},
			wantConds: map[string]*expectedCondition{
				ConditionReady:                {metav1.ConditionFalse, "DownloadFailed"},
				ConditionScaled:               {metav1.ConditionUnknown, "DownloadFailed"},
				ConditionConfigurationApplied: {metav1.ConditionUnknown, "DownloadFailed"},
				ConditionManaged:              {metav1.ConditionFalse, "DownloadFailed"},
			},
			state:   stateDegraded,
			message: "Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted",
			tip:     "The on-disk artifact has been tampered with or corrupted. Verify integrity, delete and re-fetch the package from the registry. The controller will retry reconcile.",
		},
		{
			name: "reconcile: load from filesystem failed",
			opts: running(intCond(intLoaded, metav1.ConditionFalse, "RuntimeError")),
			wantConds: map[string]*expectedCondition{
				ConditionReady:   {metav1.ConditionFalse, "LoadFromFilesystemFailed"},
				ConditionManaged: {metav1.ConditionFalse, "LoadFromFilesystemFailed"},
				// Scaled (workload health) and ConfigurationApplied are unaffected
				// by a load failure on reconcile.
				ConditionScaled:               {metav1.ConditionTrue, ConditionScaled},
				ConditionConfigurationApplied: {metav1.ConditionTrue, ConditionConfigurationApplied},
			},
			state:   stateDegraded,
			message: "Reconcile failed: application could not be loaded from filesystem; runtime state can no longer be trusted",
			tip:     "Delete the corrupted package cache on the node. The controller will retry loading; conditions will be restored based on reconcile progress.",
		},
		{
			name: "reconcile: invalid settings (old config still serving)",
			opts: running(intCond(intConfigured, metav1.ConditionFalse, "InvalidSettings")),
			wantConds: map[string]*expectedCondition{
				ConditionConfigurationApplied: {metav1.ConditionFalse, "SettingsInvalid"},
				ConditionReady:                {metav1.ConditionTrue, ConditionReady},
				ConditionScaled:               {metav1.ConditionTrue, ConditionScaled},
				ConditionManaged:              {metav1.ConditionTrue, ConditionManaged},
			},
			state:   stateDegraded,
			message: "Reconcile failed: new settings did not pass validation; previously applied configuration is still in effect",
			tip:     "The previous configuration continues to work. Fix the invalid fields in the application settings. After saving, the controller will re-apply the settings.",
		},
		{
			name: "reconcile: startup/runtime hooks failed",
			opts: running(intCond(intHooksProcessed, metav1.ConditionFalse, "HookExecutionFailed")),
			wantConds: map[string]*expectedCondition{
				ConditionReady:                {metav1.ConditionFalse, "HookFailed"},
				ConditionManaged:              {metav1.ConditionFalse, "HookFailed"},
				ConditionConfigurationApplied: {metav1.ConditionFalse, "HookFailed"},
				ConditionScaled:               {metav1.ConditionTrue, ConditionScaled},
			},
			state:   stateDegraded,
			message: "Reconcile failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
			tip:     "Pods are alive but the controller is not managing them. Check the failed hook logs. Fix the hook/config — the controller will retry reconcile.",
		},
		{
			name: "reconcile: Helm apply failed",
			opts: running(intCond(intManifestsApplied, metav1.ConditionFalse, "boom")),
			wantConds: map[string]*expectedCondition{
				ConditionReady:   {metav1.ConditionFalse, "ManifestsApplyFailed"},
				ConditionManaged: {metav1.ConditionFalse, "ManifestsApplyFailed"},
			},
			state:   stateDegraded,
			message: "Reconcile failed: Helm could not apply manifests",
			tip:     "Check events in the namespace and helm history. Resolve resource conflicts (foreign ownership, finalizers, CRD mismatches). The controller will retry reconcile automatically.",
		},
		{
			name: "reconcile: workload degraded (health monitor)",
			opts: running(intCond(intScaled, metav1.ConditionFalse, "Degraded")),
			wantConds: map[string]*expectedCondition{
				ConditionScaled:               {metav1.ConditionFalse, "Degraded"},
				ConditionConfigurationApplied: {metav1.ConditionTrue, ConditionConfigurationApplied},
			},
			state:   stateDegraded,
			message: "Reconcile failed: workload health monitor reports degraded",
			tip:     "Workload health monitor reports degraded. Check pod status and logs to identify the root cause.",
		},

		// ── Suspended (dependency disabled under a running app) ───────

		{
			name: "suspended: dependency disabled",
			opts: running(intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled")),
			wantConds: map[string]*expectedCondition{
				ConditionInstalled:            {metav1.ConditionFalse, "RequirementsUnmet"},
				ConditionReady:                {metav1.ConditionFalse, "RequirementsUnmet"},
				ConditionScaled:               {metav1.ConditionUnknown, "RequirementsUnmet"},
				ConditionConfigurationApplied: {metav1.ConditionUnknown, "RequirementsUnmet"},
				ConditionManaged:              {metav1.ConditionUnknown, "RequirementsUnmet"},
				ConditionUpdateInstalled:      nil,
			},
			state:   stateSuspended,
			message: "Application is suspended: requirements unmet",
			tip:     "Solve the application requirements. After it, the controller will automatically restore all conditions and resume operation.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conds := testMapping(tc.opts...)
			for typ, exp := range tc.wantConds {
				cond, ok := conds[typ]
				if exp == nil {
					assert.Falsef(t, ok, "condition %q should be absent, got %+v", typ, cond)
					continue
				}
				if !assert.Truef(t, ok, "condition %q should be present", typ) {
					continue
				}
				assert.Equalf(t, exp.status, cond.Status, "condition %q status", typ)
				assert.Equalf(t, exp.reason, cond.Reason, "condition %q reason", typ)
			}

			state, message, tip := summaryFor(tc.opts...)
			assert.Equal(t, tc.state, state, "summary state")
			assert.Equal(t, tc.message, message, "summary message")
			assert.Equal(t, tc.tip, tip, "summary tip")
		})
	}
}

// TestSummarize_EdgeCases covers summarize mechanics that are not tied to a
// single lifecycle scenario.
func TestSummarize_EdgeCases(t *testing.T) {
	t.Run("no conditions yet — pending install", func(t *testing.T) {
		state, message, tip := summaryFor()
		assert.Equal(t, statePending, state)
		assert.Equal(t, "Installation is waiting for dependent modules to converge", message)
		assert.Equal(t, "Wait for dependent modules to converge automatically. No action required.", tip)
	})

	t.Run("pipeline clear and scaled — install just completed, ready", func(t *testing.T) {
		// Not yet installed externally, but every internal gate is True and the
		// workload is scaled: this is the run install completes on. Mirrors
		// mapInstalled's success check — the app is Ready.
		state, message, tip := summaryFor(withSuccessfulApply()...)
		assert.Equal(t, stateReady, state)
		assert.Empty(t, message)
		assert.Empty(t, tip)
	})

	t.Run("workload reconciling during reconcile", func(t *testing.T) {
		// The health monitor reports a rollout as Scaled=False/Reconciling.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionFalse, "Reconciling"))
		assert.Equal(t, stateDegraded, state)
		assert.Equal(t, "Reconcile failed: workload is still reconciling", message)
		assert.Equal(t, "Workload is still reconciling. Wait for the rollout to complete. If it stays in this state, check pod status and events.", tip)
	})

	t.Run("scaled unknown (no workloads to observe) is ready, not degraded", func(t *testing.T) {
		// Unknown is the health monitor's "nothing to observe" signal with an
		// empty reason — it is not a degradation.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionUnknown, ""))
		assert.Equal(t, stateReady, state)
		assert.Empty(t, message)
		assert.Empty(t, tip)
	})

	t.Run("manifests applying on a healthy app is ready, not degraded", func(t *testing.T) {
		// ManifestsApplied=False/ApplyingManifests is a transient progress
		// marker, not a failure: firstFalse skips it so a healthy app does not
		// flap to Degraded during every apply window.
		opts := running(intCond(intManifestsApplied, metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)))
		state, message, tip := summaryFor(opts...)
		assert.Equal(t, stateReady, state)
		assert.Empty(t, message)
		assert.Empty(t, tip)
	})

	t.Run("update mid-apply is updating, not ready, even over a sticky failure", func(t *testing.T) {
		// Regression: a working version is installed, then switched to a broken
		// one. The new version's manifests fail, so the mapper records
		// UpdateInstalled/Ready=False/ManifestsApplyFailed (sticky). On the next
		// reconcile attempt nelm re-enters the apply window and emits
		// ManifestsApplied=False/ApplyingManifests, which firstFalse skips as
		// transient progress — so the update pipeline reads clear and the mapper
		// leaves the sticky failure untouched (Scaled=False keeps mapReady from
		// re-asserting True). summarize sees the same state and must NOT jump to
		// Ready: an update with manifests not yet applied is still updating, never
		// ready. Before the fix this returned stateReady, contradicting the
		// conditions a client also reads.
		opts := updatingApp(
			intCond(intManifestsApplied, metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
			intCond(intScaled, metav1.ConditionFalse, "Reconciling"),
		)
		state, _, _ := summaryFor(opts...)
		assert.Equal(t, stateUpdating, state)
	})

	t.Run("unknown reconcile reason falls back to generic phrasing", func(t *testing.T) {
		// The health monitor is the only writer that can produce an arbitrary
		// reason (it passes through), so it exercises the defensive fallback.
		state, message, tip := summaryFor(installed(), intCond(intScaled, metav1.ConditionFalse, "WeirdError"))
		assert.Equal(t, stateDegraded, state)
		assert.Equal(t, "Reconcile failed: WeirdError", message)
		assert.Empty(t, tip)
	})

	t.Run("real failure outranks degraded workload", func(t *testing.T) {
		// A filesystem failure and a degraded workload at once: intScaled is the
		// lowest-priority gate, so the artifact failure wins.
		state, message, _ := summaryFor(installed(),
			intCond(intReadyOnFilesystem, metav1.ConditionFalse, "VerificationFailed"),
			intCond(intScaled, metav1.ConditionFalse, "Degraded"))
		assert.Equal(t, stateDegraded, state)
		assert.Equal(t, "Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted", message)
	})
}

func TestSummarize_SuspendedVsPending(t *testing.T) {
	t.Run("suspended when previously installed and requirements drop", func(t *testing.T) {
		state, message, _ := summaryFor(installed(), intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"))
		assert.Equal(t, stateSuspended, state)
		assert.Equal(t, "Application is suspended: requirements unmet", message)
	})

	t.Run("pending when requirements unmet on first install", func(t *testing.T) {
		// No external Installed=True: this is a first install blocked on
		// requirements, not a running app that lost a dependency.
		state, message, _ := summaryFor(intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"))
		assert.Equal(t, statePending, state)
		assert.NotEqual(t, "Application is suspended: requirements unmet", message)
	})
}
