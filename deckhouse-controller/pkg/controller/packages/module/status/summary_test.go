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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/werf/nelm/pkg/legacy/progrep"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
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

// installed marks the module as previously installed (sticky external
// condition), which puts the mapper and summarize into the update or
// reconcile phase.
func installed() mappingOption {
	return withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed")
}

// running is a previously-installed module with every internal gate True; the
// overrides (applied last) introduce the fault under test.
func running(overrides ...mappingOption) []mappingOption {
	opts := append([]mappingOption{installed()}, withSuccessfulApply()...)
	return append(opts, overrides...)
}

// TestModuleSummaryScenarios drives the module-specific states — scheduler
// verdicts at install time and the disabled-module transition — through BOTH
// the mapper and summarize, asserting the external conditions and the summary
// together. Scenarios shared with the application service (plain install
// failures, update, reconcile) are covered by the application package tests;
// only the module-specific vocabulary is verified here.
func TestModuleSummaryScenarios(t *testing.T) {
	cases := []struct {
		name      string
		opts      []mappingOption
		wantConds map[string]*expectedCondition // nil value asserts the condition is absent
		state     string
		message   string
		tip       string
	}{
		// ── Install blocked by a scheduler verdict ─────────────────────

		{
			name: "install: module disabled",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "Disabled"),
			},
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "Disabled"},
			}),
			state:   statePending,
			message: "Installation is blocked: the module is disabled",
			tip:     "Enable the module to start the installation.",
		},
		{
			name: "install: dependency not enabled",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			},
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "DependencyNotEnabled"},
			}),
			state:   statePending,
			message: "Installation is blocked: a required module is not enabled",
			tip:     "Enable the required module listed in the condition message. The installation will continue automatically.",
		},
		{
			name: "install: enabled-script failed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "EnabledScriptError"),
			},
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "EnabledScriptError"},
			}),
			state:   stateFailed,
			message: "Installation failed: the module's enabled-script failed",
			tip:     "Check the Deckhouse controller logs for the script error. Fix the script or the cluster state it inspects.",
		},
		{
			// A verdict outside the documented vocabulary stays a blocked
			// state, not a failure — see adviseSchedulerBlocked.
			name: "install: unknown scheduler verdict",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "CustomRuleForbid"),
			},
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "CustomRuleForbid"},
			}),
			state:   statePending,
			message: "Installation is blocked: CustomRuleForbid",
			tip:     "",
		},

		// ── Running module withdrawn by the scheduler ──────────────────

		{
			name: "running: module disabled",
			opts: running(
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "Disabled"),
			),
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "Disabled"},
			}),
			state:   stateSuspended,
			message: "Module is suspended: the module was disabled",
			tip:     "Enable the module to resume. The controller will restore all conditions and resume operation automatically.",
		},
		{
			name: "running: dependency lost",
			opts: running(
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			),
			wantConds: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, "DependencyNotEnabled"},
			}),
			state:   stateSuspended,
			message: "Module is suspended: requirements unmet",
			tip:     "Solve the module requirements. After it, the controller will automatically restore all conditions and resume operation.",
		},

		// ── Healthy steady state ───────────────────────────────────────

		{
			name: "running: healthy",
			opts: running(),
			wantConds: map[string]*expectedCondition{
				ConditionReady: {metav1.ConditionTrue, "Ready"},
			},
			state:   stateReady,
			message: "",
			tip:     "",
		},

		// ── Workload gate ──────────────────────────────────────────────
		//
		// Modules install with nelm's final tracking disabled, so
		// ManifestsApplied=True says the manifests reached the API server, not
		// that the workload is up. The summary must not read it as Ready.

		{
			name: "update: manifests applied while the workload rolls out",
			opts: running(
				withVersionChanged(),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Reconciling"),
			),
			wantConds: map[string]*expectedCondition{
				ConditionScaled: {metav1.ConditionFalse, "Reconciling"},
			},
			state:   stateUpdating,
			message: "Update applied: the new version's workload is rolling out",
			tip:     "Wait for the rollout to finish. If it stalls, check pod status and events.",
		},
		{
			name: "update: manifests applied with no health report",
			opts: running(
				withVersionChanged(),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionUnknown, ""),
			),
			state:   stateUpdating,
			message: "Update applied: waiting for a workload the health monitor can confirm",
			tip:     "Either no report has arrived yet, or the chart ships no Deployment or StatefulSet, the only kinds the health monitor watches.",
		},

		// ── Deleting (teardown accepted by the runtime) ────────────────

		{
			// A disabled module being deleted reports the teardown, not the
			// scheduler verdict that would otherwise win.
			name: "deleting: disabled module is torn down",
			opts: []mappingOption{
				installed(),
				withInternalCondition(intRequirementsMet, metav1.ConditionFalse, reasonDisabled),
				withDeleting(),
			},
			wantConds: map[string]*expectedCondition{
				ConditionInstalled: {metav1.ConditionFalse, condmap.ReasonDeleting},
			},
			state:   stateDeleting,
			message: "Module is being deleted",
			tip:     "No action is required. The resource disappears once its release is taken down.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conds := testMapping(tc.opts...)
			for condType, want := range tc.wantConds {
				got, ok := conds[condType]
				if want == nil {
					assert.False(t, ok, "condition %s should be absent", condType)
					continue
				}
				if !assert.True(t, ok, "condition %s should be present", condType) {
					continue
				}
				assert.Equal(t, want.status, got.Status, "condition %s status", condType)
				assert.Equal(t, want.reason, got.Reason, "condition %s reason", condType)
			}

			state, message, tip := summaryFor(tc.opts...)
			assert.Equal(t, tc.state, state, "summary state")
			assert.Equal(t, tc.message, message, "summary message")
			assert.Equal(t, tc.tip, tip, "summary tip")
		})
	}
}

// TestComputeAndApplyConditionsInstalledGate covers the removal half of the
// Installed gate, which the mapping cases above cannot see: switching a running
// module off removes every gated condition from the resource, enabling it again
// keeps them away while it installs, and the run that installs it brings them
// back. status.enabled follows the scheduler throughout.
func TestComputeAndApplyConditionsInstalledGate(t *testing.T) {
	module := &v1beta1.Module{ObjectMeta: metav1.ObjectMeta{Name: "mod"}}

	internal := func(requirements, hooks intstatus.Condition) []intstatus.Condition {
		return []intstatus.Condition{
			requirements,
			{Type: intstatus.ConditionReadyOnFilesystem, Status: metav1.ConditionTrue},
			{Type: intstatus.ConditionLoaded, Status: metav1.ConditionTrue},
			{Type: intstatus.ConditionConfigured, Status: metav1.ConditionTrue},
			hooks,
			{Type: intstatus.ConditionManifestsApplied, Status: metav1.ConditionTrue},
			{Type: intstatus.ConditionScaled, Status: metav1.ConditionTrue},
		}
	}
	enabled := intstatus.Condition{Type: intstatus.ConditionRequirementsMet, Status: metav1.ConditionTrue}
	disabled := intstatus.Condition{Type: intstatus.ConditionRequirementsMet, Status: metav1.ConditionFalse, Reason: reasonDisabled}
	hooksOK := intstatus.Condition{Type: intstatus.ConditionHooksProcessed, Status: metav1.ConditionTrue}
	hooksFailed := intstatus.Condition{Type: intstatus.ConditionHooksProcessed, Status: metav1.ConditionFalse, Reason: "HookExecutionFailed"}

	current := intstatus.Status{Version: "1.0.0", Conditions: internal(enabled, hooksOK)}
	svc := &Service{
		mapper: buildMapper(),
		getter: func(string) intstatus.Status { return current },
	}

	svc.computeAndApplyConditions("mod", module)
	require.Len(t, module.Status.Conditions, 5)
	require.NotNil(t, module.Status.Enabled)
	assert.True(t, *module.Status.Enabled)

	assertOnly := func(want map[string]string) {
		t.Helper()

		require.Len(t, module.Status.Conditions, len(want))
		for condType, reason := range want {
			cond := meta.FindStatusCondition(module.Status.Conditions, condType)
			if assert.NotNil(t, cond, "condition %s", condType) {
				assert.Equal(t, reason, cond.Reason, "condition %s reason", condType)
			}
		}
	}

	current = intstatus.Status{Version: "1.0.0", Conditions: internal(disabled, hooksOK)}
	svc.computeAndApplyConditions("mod", module)
	assertOnly(map[string]string{ConditionInstalled: reasonDisabled})
	assert.False(t, *module.Status.Enabled)

	// Enabled again, the reinstall fails at hooks: Installed alone reports it.
	current = intstatus.Status{Version: "1.0.0", Conditions: internal(enabled, hooksFailed)}
	svc.computeAndApplyConditions("mod", module)
	assertOnly(map[string]string{ConditionInstalled: "HookFailed"})
	assert.True(t, *module.Status.Enabled)

	current = intstatus.Status{Version: "1.0.0", Conditions: internal(enabled, hooksOK)}
	svc.computeAndApplyConditions("mod", module)
	require.Len(t, module.Status.Conditions, 5)
	assert.True(t, meta.IsStatusConditionTrue(module.Status.Conditions, ConditionReady))
}

// TestComputeAndApplyEnabled covers status.enabled: the scheduler's verdict,
// which spec.enabled only feeds — an explicit intent can still be overruled —
// and which stands while the scheduler has not decided anew.
func TestComputeAndApplyEnabled(t *testing.T) {
	requirements := func(status metav1.ConditionStatus, reason string) intstatus.Status {
		return intstatus.Status{Conditions: []intstatus.Condition{
			{Type: intstatus.ConditionRequirementsMet, Status: status, Reason: intstatus.ConditionReason(reason)},
		}}
	}

	cases := []struct {
		name    string
		spec    *bool
		current *bool
		runtime intstatus.Status
		want    *bool
	}{
		{
			name:    "enabled by the config, disabled by the scheduler",
			spec:    ptr.To(true),
			runtime: requirements(metav1.ConditionFalse, "DependencyNotEnabled"),
			want:    ptr.To(false),
		},
		{
			name:    "disabled by the config and the scheduler",
			spec:    ptr.To(false),
			runtime: requirements(metav1.ConditionFalse, reasonDisabled),
			want:    ptr.To(false),
		},
		{
			name:    "no config, enabled by the scheduler",
			runtime: requirements(metav1.ConditionTrue, ""),
			want:    ptr.To(true),
		},
		{
			name:    "absent before the first decision",
			spec:    ptr.To(true),
			runtime: requirements(metav1.ConditionUnknown, ""),
		},
		{
			// A version change resets the runtime status to Unknown until the
			// scheduler runs again.
			name:    "last verdict stands while the scheduler has not decided",
			current: ptr.To(true),
			runtime: requirements(metav1.ConditionUnknown, ""),
			want:    ptr.To(true),
		},
		{
			// After a restart the runtime has no status for the module yet.
			name:    "last verdict stands while the runtime tracks nothing",
			current: ptr.To(false),
			want:    ptr.To(false),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module := &v1beta1.Module{
				ObjectMeta: metav1.ObjectMeta{Name: "mod"},
				Spec:       v1beta1.ModuleSpec{Enabled: tc.spec},
				Status:     v1beta1.ModuleStatus{Enabled: tc.current},
			}
			svc := &Service{
				mapper: buildMapper(),
				getter: func(string) intstatus.Status { return tc.runtime },
			}

			svc.computeAndApplyConditions("mod", module)
			assert.Equal(t, tc.want, module.Status.Enabled)
		})
	}
}

// TestComputeAndApplyConditionsOnDeletion covers this package's own deletionTimestamp
// wiring: the cases above set condmap.State.Deleting directly and never reach it.
func TestComputeAndApplyConditionsOnDeletion(t *testing.T) {
	deleted := metav1.NewTime(time.Unix(0, 0))
	module := &v1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "mod", DeletionTimestamp: &deleted},
		Status:     v1beta1.ModuleStatus{Enabled: ptr.To(true)},
	}

	// A Run task that finished after the teardown started still reports
	// ManifestsApplied, which is what would otherwise commit the version. The
	// teardown itself writes False/Deleting over RequirementsMet.
	svc := &Service{
		mapper: buildMapper(),
		getter: func(string) intstatus.Status {
			return intstatus.Status{
				Version: "1.2.3",
				Conditions: []intstatus.Condition{
					{Type: intstatus.ConditionRequirementsMet, Status: metav1.ConditionFalse, Reason: intstatus.ConditionReasonDeleting},
					{Type: intstatus.ConditionManifestsApplied, Status: metav1.ConditionTrue},
					{Type: intstatus.ConditionScaled, Status: metav1.ConditionTrue},
				},
			}
		},
	}

	svc.computeAndApplyConditions("mod", module)

	assert.Len(t, module.Status.Conditions, 6)
	for _, cond := range module.Status.Conditions {
		assert.Equal(t, metav1.ConditionFalse, cond.Status, "condition %s status", cond.Type)
		assert.Equal(t, condmap.ReasonDeleting, cond.Reason, "condition %s reason", cond.Type)
	}
	assert.Equal(t, stateDeleting, module.Status.Summary.State)
	assert.Empty(t, module.Status.CurrentVersion.Version)
	assert.Equal(t, ptr.To(true), module.Status.Enabled, "the teardown is no scheduler verdict")
}

// TestComputeAndApplyTrackingOnDeletion covers the uninstall progress of a CR on its way out:
// unlike the version, the report is committed despite the deletionTimestamp, and it outlives
// the runtime status, which is dropped when the teardown drains, before the finalizer is removed.
func TestComputeAndApplyTrackingOnDeletion(t *testing.T) {
	deleted := metav1.NewTime(time.Unix(0, 0))
	module := &v1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "mod", DeletionTimestamp: &deleted},
	}

	tracking := intstatus.Tracking{Completed: 1, Remaining: 1, Report: progrep.ProgressReport{Operations: []progrep.Operation{
		{OperationRef: progrep.OperationRef{Type: progrep.OperationTypeDelete}, Category: progrep.OperationCategoryResource, Status: progrep.OperationStatusCompleted},
		{OperationRef: progrep.OperationRef{Type: progrep.OperationTypeTrackAbsence}, Category: progrep.OperationCategoryTrack, Status: progrep.OperationStatusProgressing},
	}}}
	want, err := json.Marshal(tracking)
	require.NoError(t, err)

	current := intstatus.Status{Tracking: tracking}
	svc := &Service{
		mapper: buildMapper(),
		getter: func(string) intstatus.Status { return current },
	}

	svc.computeAndApplyConditions("mod", module)
	assert.JSONEq(t, string(want), string(module.Status.Tracking.Raw))

	current = intstatus.Status{}
	svc.computeAndApplyConditions("mod", module)
	assert.JSONEq(t, string(want), string(module.Status.Tracking.Raw))
}
