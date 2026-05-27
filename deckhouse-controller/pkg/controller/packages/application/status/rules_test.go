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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

type mappingOption func(state *condmap.State)

func withInternalCondition(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return func(state *condmap.State) {
		state.Internal[cond] = metav1.Condition{
			Type:   cond,
			Status: status,
			Reason: reason,
		}
	}
}

func withExternalCondition(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return func(state *condmap.State) {
		state.External[cond] = metav1.Condition{
			Type:   cond,
			Status: status,
			Reason: reason,
		}
	}
}

func withVersionChanged() mappingOption {
	return func(state *condmap.State) {
		state.Updating = true
	}
}

func testMapping(opts ...mappingOption) map[string]metav1.Condition {
	state := &condmap.State{
		Internal: make(map[string]metav1.Condition),
		External: make(map[string]metav1.Condition),
	}

	for _, opt := range opts {
		opt(state)
	}

	result := make(map[string]metav1.Condition)
	for _, cond := range buildMapper().Map(*state) {
		result[cond.Type] = cond
	}

	return result
}

// expectedCondition defines what we expect for a condition in test results
type expectedCondition struct {
	status metav1.ConditionStatus
	reason string
}

// testCase defines a single test case for condition mapping
type testCase struct {
	name     string
	opts     []mappingOption
	expected map[string]*expectedCondition // nil value means condition should be absent
}

func runTestCases(t *testing.T, cases []testCase) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := testMapping(tc.opts...)

			for condType, exp := range tc.expected {
				if exp == nil {
					_, ok := result[condType]
					assert.False(t, ok, "condition '%s' should not be present", condType)
					continue
				}

				cond, ok := result[condType]
				if !ok {
					assert.Failf(t, "condition not found", "condition '%s' not found in result", condType)
					continue
				}

				assert.Equal(t, exp.status, cond.Status, "condition '%s' status", condType)
				assert.Equal(t, exp.reason, cond.Reason, "condition '%s' reason", condType)
			}
		})
	}
}

func TestInstalledRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when Scaled and no version change",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				// True conditions carry no reason — emit() drops it.
				ConditionInstalled: {status: metav1.ConditionTrue, reason: ConditionInstalled},
			},
		},
		{
			name: "true on first install regardless of Updating flag",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				// mapInstalled does not gate on Updating; stickiness happens via external state.
				ConditionInstalled: {status: metav1.ConditionTrue, reason: ConditionInstalled},
			},
		},
		{
			name: "false when ReadyOnFilesystem is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "DownloadFailed"},
			},
		},
		{
			name: "false when Loaded is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionFalse, "RuntimeError"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "LoadFromFilesystemFailed"},
			},
		},
		{
			name: "absent when only Scaled is False (Scaled is not in install pipeline)",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "ClusterNotReady"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
			},
		},
		{
			name: "false when RequirementsMet is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "RequirementsNotMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
			},
		},
		{
			name: "sticky - not in result when already true externally",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "PreviouslyInstalled"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "ClusterNotReady"),
			},
			expected: map[string]*expectedCondition{
				// Sticky rule skips evaluation - condition preserved in external state, not in result
				ConditionInstalled: nil,
			},
		},
	}

	runTestCases(t, cases)
}

func TestUpdateInstalledRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when Scaled and version changed",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: ConditionUpdateInstalled},
			},
		},
		{
			name: "absent when not installed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil,
			},
		},
		{
			name: "true when healthy after rollback (no version change)",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				// In a real rollback scenario, UpdateInstalled was set to False during the failed update
				withExternalCondition(ConditionUpdateInstalled, metav1.ConditionFalse, "UpdateFailed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: ConditionUpdateInstalled},
			},
		},
		{
			name: "absent after fresh install with no updates",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
				// No UpdateInstalled in external state = no update ever happened
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil, // Should not be present for fresh installs
			},
		},
	}

	runTestCases(t, cases)
}

func TestReadyRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when Scaled",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionTrue, reason: ConditionReady},
			},
		},
		{
			name: "false when not installed and Pending",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionPending), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "Pending"},
			},
		},
		{
			name: "requirements passed does not explain readiness",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionTrue, "RequirementsMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: nil,
			},
		},
		{
			name: "true when installed even with Pending",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
				withInternalCondition(string(intstatus.ConditionPending), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionTrue, reason: ConditionReady},
			},
		},
	}

	runTestCases(t, cases)
}

func TestScaledRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when Scaled",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionTrue, reason: ConditionScaled},
			},
		},
		{
			name: "false when Scaled is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Degraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionFalse, reason: "Degraded"},
			},
		},
		{
			name: "false when Scaled is false with Reconciling reason",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Reconciling"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionFalse, reason: "Reconciling"},
			},
		},
		{
			name: "unknown when internal Scaled is absent",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "RequirementsNotMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionUnknown, reason: ""},
			},
		},
	}

	runTestCases(t, cases)
}

func TestManagedRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when Loaded, Scaled, HooksProcessed and ManifestsApplied are true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: ConditionManaged},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "HookFailed"},
			},
		},
		{
			name: "false when ManifestsApplied is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, "boom"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
			},
		},
		{
			name: "false when ReadyOnFilesystem is false during reconcile",
			opts: []mappingOption{
				// reconcile phase: Installed=True externally, not updating.
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "DownloadFailed"},
			},
		},
		{
			name: "true when Pending is true (Pending no longer gates Managed)",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsOK"),
				withInternalCondition(string(intstatus.ConditionPending), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: ConditionManaged},
			},
		},
	}

	runTestCases(t, cases)
}

func TestConfigurationAppliedRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when all config conditions true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: ConditionConfigurationApplied},
			},
		},
		{
			name: "false when Configured is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionFalse, "InvalidSettings"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "SettingsInvalid"},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "HookFailed"},
			},
		},
		{
			name: "false when ManifestsApplied is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, "HelmFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
			},
		},
	}

	runTestCases(t, cases)
}

// TestDependencyDisabled covers the case where an installed and running
// application loses a hard dependency (e.g. a module it depends on was
// disabled). The cause is external, so user-facing signals (Installed, Ready)
// go False, while ConfigurationApplied and Managed go Unknown — managing is
// meaningless until the dependency returns. Scaled is excluded: it is owned
// by the workload health monitor and mirrors the internal condition as-is.
func TestDependencyDisabled(t *testing.T) {
	// Realistic runtime state: app was running with all internal conditions
	// True from the previous successful reconcile, then RequirementsMet flipped
	// to False because a dependency module was disabled.
	runningInternals := []mappingOption{
		withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
		withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
		withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionTrue, "Mounted"),
		withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "Loaded"),
		withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "ConfigOK"),
		withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
		withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsOK"),
		withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
	}

	cases := []testCase{
		{
			name: "all public conditions reflect dependency disabled",
			opts: runningInternals,
			expected: map[string]*expectedCondition{
				// Installed overrides stickiness — the user must see the app stopped being installed.
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				ConditionReady:     {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				// Scaled mirrors its internal condition (True here) — the health monitor is its sole writer.
				ConditionScaled:               {status: metav1.ConditionTrue, reason: ConditionScaled},
				ConditionConfigurationApplied: {status: metav1.ConditionUnknown, reason: "RequirementsUnmet"},
				ConditionManaged:              {status: metav1.ConditionUnknown, reason: "RequirementsUnmet"},
				// UpdateInstalled is silent — the dependency-disabled state is the dominant signal.
				ConditionUpdateInstalled: nil,
			},
		},
		{
			name: "UpdateInstalled silent even while updating",
			opts: append(runningInternals, withVersionChanged()),
			expected: map[string]*expectedCondition{
				ConditionInstalled:            {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				ConditionReady:                {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				ConditionScaled:               {status: metav1.ConditionTrue, reason: ConditionScaled},
				ConditionConfigurationApplied: {status: metav1.ConditionUnknown, reason: "RequirementsUnmet"},
				ConditionManaged:              {status: metav1.ConditionUnknown, reason: "RequirementsUnmet"},
				ConditionUpdateInstalled:      nil,
			},
		},
		{
			name: "first-install dependency unmet still uses install pipeline (no Unknowns)",
			opts: []mappingOption{
				// No external Installed=True — this is a first install, not a running app.
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				ConditionReady:     {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
				// Scaled goes Unknown when its internal condition is absent — health monitor hasn't reported yet.
				ConditionScaled:               {status: metav1.ConditionUnknown, reason: ""},
				ConditionConfigurationApplied: nil,
				ConditionManaged:              nil,
				ConditionUpdateInstalled:      nil,
			},
		},
	}

	runTestCases(t, cases)
}
