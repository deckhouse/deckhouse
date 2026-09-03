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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
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
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:   {metav1.ConditionFalse, "Disabled"},
				ConditionInstalled: {metav1.ConditionFalse, "Disabled"},
				ConditionReady:     {metav1.ConditionFalse, "Disabled"},
			},
			state:   statePending,
			message: "Installation is blocked: the module is disabled",
			tip:     "Enable the module to start the installation.",
		},
		{
			name: "install: dependency not enabled",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			},
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:   {metav1.ConditionFalse, "DependencyNotEnabled"},
				ConditionInstalled: {metav1.ConditionFalse, "DependencyNotEnabled"},
			},
			state:   statePending,
			message: "Installation is blocked: a required module is not enabled",
			tip:     "Enable the required module listed in the condition message. The installation will continue automatically.",
		},
		{
			name: "install: enabled-script failed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "EnabledScriptError"),
			},
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:   {metav1.ConditionFalse, "EnabledScriptError"},
				ConditionInstalled: {metav1.ConditionFalse, "EnabledScriptError"},
			},
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
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:   {metav1.ConditionFalse, "CustomRuleForbid"},
				ConditionInstalled: {metav1.ConditionFalse, "CustomRuleForbid"},
			},
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
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:              {metav1.ConditionFalse, "Disabled"},
				ConditionInstalled:            {metav1.ConditionFalse, "Disabled"},
				ConditionReady:                {metav1.ConditionFalse, "Disabled"},
				ConditionScaled:               {metav1.ConditionUnknown, "Disabled"},
				ConditionManaged:              {metav1.ConditionUnknown, "Disabled"},
				ConditionConfigurationApplied: {metav1.ConditionUnknown, "Disabled"},
			},
			state:   stateSuspended,
			message: "Module is suspended: the module was disabled",
			tip:     "Enable the module to resume. The controller will restore all conditions and resume operation automatically.",
		},
		{
			name: "running: dependency lost",
			opts: running(
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			),
			wantConds: map[string]*expectedCondition{
				ConditionEnabled:   {metav1.ConditionFalse, "DependencyNotEnabled"},
				ConditionInstalled: {metav1.ConditionFalse, "DependencyNotEnabled"},
				ConditionScaled:    {metav1.ConditionUnknown, "DependencyNotEnabled"},
			},
			state:   stateSuspended,
			message: "Module is suspended: requirements unmet",
			tip:     "Solve the module requirements. After it, the controller will automatically restore all conditions and resume operation.",
		},

		// ── Healthy steady state ───────────────────────────────────────

		{
			name: "running: healthy",
			opts: running(),
			wantConds: map[string]*expectedCondition{
				ConditionEnabled: {metav1.ConditionTrue, "Enabled"},
				ConditionReady:   {metav1.ConditionTrue, "Ready"},
			},
			state:   stateReady,
			message: "",
			tip:     "",
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
				ConditionEnabled:   {metav1.ConditionFalse, condmap.ReasonDeleting},
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

// TestComputeAndApplyConditionsOnDeletion covers this package's own deletionTimestamp
// wiring: the cases above set condmap.State.Deleting directly and never reach it.
func TestComputeAndApplyConditionsOnDeletion(t *testing.T) {
	deleted := metav1.NewTime(time.Unix(0, 0))
	module := &v1alpha2.Module{
		ObjectMeta: metav1.ObjectMeta{Name: "mod", DeletionTimestamp: &deleted},
	}

	// A Run task that finished after the teardown started still reports
	// ManifestsApplied, which is what would otherwise commit the version.
	svc := &Service{
		mapper: buildMapper(),
		getter: func(string) intstatus.Status {
			return intstatus.Status{
				Version: "1.2.3",
				Conditions: []intstatus.Condition{
					{Type: intstatus.ConditionManifestsApplied, Status: metav1.ConditionTrue},
					{Type: intstatus.ConditionScaled, Status: metav1.ConditionTrue},
				},
			}
		},
	}

	svc.computeAndApplyConditions("mod", module)

	assert.Len(t, module.Status.Conditions, 7)
	for _, cond := range module.Status.Conditions {
		assert.Equal(t, metav1.ConditionFalse, cond.Status, "condition %s status", cond.Type)
		assert.Equal(t, condmap.ReasonDeleting, cond.Reason, "condition %s reason", cond.Type)
	}
	assert.Equal(t, stateDeleting, module.Status.Summary.State)
	assert.Empty(t, module.Status.CurrentVersion.Version)
}
