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

func withDeleting() mappingOption {
	return func(state *condmap.State) {
		state.Deleting = true
	}
}

func withSuccessfulApply() []mappingOption {
	return []mappingOption{
		withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionTrue, "RequirementsMet"),
		withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionTrue, "Mounted"),
		withInternalCondition(string(intstatus.ConditionLoaded), metav1.ConditionTrue, "Loaded"),
		withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "Configured"),
		withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksProcessed"),
		withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "ManifestsApplied"),
		withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Scaled"),
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
	for _, cond := range buildMapper().Map(*state).Set {
		result[cond.Type] = cond
	}

	return result
}

// expectedCondition defines what we expect for a condition in test results
type expectedCondition struct {
	status metav1.ConditionStatus
	reason string
}

// gatedConditions are the conditions behind the Installed gate.
var gatedConditions = []string{
	ConditionUpdateInstalled,
	ConditionReady,
	ConditionScaled,
	ConditionManaged,
	ConditionConfigurationApplied,
}

// gatedAbsent adds to want the promise of the Installed gate: every condition
// behind it is absent from the mapping result.
func gatedAbsent(want map[string]*expectedCondition) map[string]*expectedCondition {
	for _, condType := range gatedConditions {
		want[condType] = nil
	}

	return want
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
			name: "true when apply is complete",
			opts: withSuccessfulApply(),
			expected: map[string]*expectedCondition{
				// True conditions carry no reason — emit() drops it.
				ConditionInstalled: {status: metav1.ConditionTrue, reason: ConditionInstalled},
			},
		},
		{
			name: "true on first install regardless of Updating flag when apply is complete",
			opts: append(withSuccessfulApply(), withVersionChanged()),
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
			name: "absent while manifests are still applying",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
			},
		},
		{
			// Scaled alone must not report a finished install: the version, URLs
			// and settings are committed under ManifestsApplied.
			name: "absent when Scaled arrives before manifests are applied",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Scaled"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
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
			opts: append(withSuccessfulApply(),
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withVersionChanged(),
			),
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: ConditionUpdateInstalled},
			},
		},
		{
			name: "absent when not installed",
			opts: append(withSuccessfulApply(),
				withVersionChanged(),
			),
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil,
			},
		},
		{
			name: "true when healthy after rollback (no version change)",
			opts: append(withSuccessfulApply(),
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				// In a real rollback scenario, UpdateInstalled was set to False during the failed update
				withExternalCondition(ConditionUpdateInstalled, metav1.ConditionFalse, "UpdateFailed"),
			),
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: ConditionUpdateInstalled},
			},
		},
		{
			name: "absent after fresh install with no updates",
			opts: append(withSuccessfulApply(),
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				// No UpdateInstalled in external state = no update ever happened
			),
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil, // Should not be present for fresh installs
			},
		},
		{
			name: "applying over a failed update refreshes the stale failure",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				// Previous update failed at manifests; the new version is now re-applying.
				withExternalCondition(ConditionUpdateInstalled, metav1.ConditionFalse, "ManifestsApplyFailed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonApplyingManifests)},
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
			// Readiness of an app that is not installed yet is Installed's to report.
			name: "absent while the first install is pending",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionPending), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "Pending"},
				ConditionReady:     nil,
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
			// On first install readiness tracks Installed, which waits for the apply.
			name: "absent when Scaled arrives before manifests are applied",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Scaled"),
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
		{
			name: "applying over a failed update refreshes the stale failure",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withExternalCondition(ConditionReady, metav1.ConditionFalse, "ManifestsApplyFailed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonApplyingManifests)},
			},
		},
		{
			name: "applying does not flap a healthy update still serving",
			opts: []mappingOption{
				// Healthy update: the previous version still serves (Ready=True),
				// so the apply window must leave it untouched, not flap it to False.
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withExternalCondition(ConditionReady, metav1.ConditionTrue, ConditionReady),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: nil,
			},
		},
	}

	runTestCases(t, cases)
}

func TestScaledRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when first install completes",
			opts: withSuccessfulApply(),
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionTrue, reason: ConditionInstalled},
				ConditionScaled:    {status: metav1.ConditionTrue, reason: ConditionScaled},
			},
		},
		{
			// The health monitor can report Scaled before the install pipeline finishes.
			name: "absent when Scaled arrives before manifests are applied",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
				ConditionScaled:    nil,
			},
		},
		{
			name: "absent while first install manifests are being applied",
			opts: append(withSuccessfulApply(),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
			),
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
				ConditionScaled:    nil,
			},
		},
		{
			name: "true when installed and Scaled",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionTrue, reason: ConditionScaled},
			},
		},
		{
			name: "absent during first install when Scaled is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Degraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: nil,
			},
		},
		{
			name: "absent during first install when Scaled is false with Reconciling reason",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Reconciling"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: nil,
			},
		},
		{
			name: "false when installed app health monitor reports Degraded",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionFalse, "Degraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionFalse, reason: "Degraded"},
			},
		},
		{
			name: "absent during first install when internal Scaled is absent",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "RequirementsNotMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: nil,
			},
		},
		{
			name: "absent during first install failure even if internal Scaled is true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionScaled), metav1.ConditionTrue, "Scaled"),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: nil,
			},
		},
		{
			name: "unknown with hook reason when update hook failure stops the new version",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HookInitializationFailed"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionUnknown, reason: "HookInitializationFailed"},
			},
		},
		{
			name: "false with manifest reason when update manifest apply fails",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, "boom"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionScaled: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
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
		{
			name: "applying over a failed update refreshes the stale failure",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withExternalCondition(ConditionManaged, metav1.ConditionFalse, "ManifestsApplyFailed"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonApplyingManifests)},
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
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: ConditionConfigurationApplied},
			},
		},
		{
			// The configuration lands before the workload is up: it is not reported
			// ahead of Installed.
			name: "absent on first install until the install completes",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled:            nil,
				ConditionConfigurationApplied: nil,
			},
		},
		{
			name: "false when Configured is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
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
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
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
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, "HelmFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
			},
		},
		{
			name: "applying over a failed update refreshes the stale failure",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withExternalCondition(ConditionConfigurationApplied, metav1.ConditionFalse, "ManifestsApplyFailed"),
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonApplyingManifests)},
			},
		},
	}

	runTestCases(t, cases)
}

// TestDependencyDisabled covers the case where an installed and running
// application loses a hard dependency (e.g. a module it depends on was
// disabled). Installed goes False — overriding its stickiness, the user must
// see the app stopped being installed — which closes the Installed gate: the
// other conditions are removed until the app is installed again.
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
			name: "only Installed reports the disabled dependency",
			opts: runningInternals,
			expected: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
			}),
		},
		{
			name: "only Installed reports the disabled dependency while updating",
			opts: append(runningInternals, withVersionChanged()),
			expected: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
			}),
		},
		{
			// The run after the switch reads Installed=False back: it is an install
			// blocked on requirements now, and the gate stays closed.
			name: "gate stays closed once Installed went False",
			opts: append(runningInternals,
				withExternalCondition(ConditionInstalled, metav1.ConditionFalse, "RequirementsUnmet"),
			),
			expected: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
			}),
		},
		{
			name: "first-install dependency unmet is reported by Installed alone",
			opts: []mappingOption{
				// No external Installed=True — this is a first install, not a running app.
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			},
			expected: gatedAbsent(map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsUnmet"},
			}),
		},
	}

	runTestCases(t, cases)
}

// TestInstalledGate covers what the gate hands the status service: while
// Installed is not True every gated condition is listed for removal, so a
// condition left on the resource does not outlive a closed gate; once it is
// True nothing is removed.
func TestInstalledGate(t *testing.T) {
	mapFor := func(opts ...mappingOption) condmap.Result {
		state := condmap.State{
			Internal: make(map[string]metav1.Condition),
			External: make(map[string]metav1.Condition),
		}
		for _, opt := range opts {
			opt(&state)
		}

		return buildMapper().Map(state)
	}

	t.Run("first install removes every gated condition", func(t *testing.T) {
		result := mapFor(
			withExternalCondition(ConditionReady, metav1.ConditionFalse, "Pending"),
			withInternalCondition(intPending, metav1.ConditionTrue, "Waiting"),
		)
		assert.ElementsMatch(t, gatedConditions, result.Remove)
	})

	t.Run("completing run opens the gate", func(t *testing.T) {
		result := mapFor(withSuccessfulApply()...)
		assert.Empty(t, result.Remove)
	})

	t.Run("installed app removes nothing", func(t *testing.T) {
		result := mapFor(running()...)
		assert.Empty(t, result.Remove)
	})

	t.Run("disabled dependency removes every gated condition", func(t *testing.T) {
		result := mapFor(running(intCond(intRequirementsMet, metav1.ConditionFalse, "DependencyNotEnabled"))...)
		assert.ElementsMatch(t, gatedConditions, result.Remove)
	})
}

// withSettingsChanged marks new settings that the Run task has not applied yet.
func withSettingsChanged() mappingOption {
	return withInternalCondition(intConfigured, metav1.ConditionFalse, string(intstatus.ConditionReasonSettingsChanged))
}

// TestSettingsChanged covers the window between a settings change and the Run
// task that applies it: Ready, Managed and ConfigurationApplied go False/SettingsChanged.
func TestSettingsChanged(t *testing.T) {
	reason := string(intstatus.ConditionReasonSettingsChanged)

	cases := []testCase{
		{
			name: "reconcile resets Ready, Managed and ConfigurationApplied",
			opts: running(withSettingsChanged()),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionFalse, reason: reason},
				ConditionManaged:              {status: metav1.ConditionFalse, reason: reason},
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: reason},
				ConditionScaled:               {status: metav1.ConditionTrue, reason: ConditionScaled},
				ConditionInstalled:            nil,
			},
		},
		{
			name: "reconcile keeps SettingsChanged while manifests apply",
			opts: running(
				withSettingsChanged(),
				withInternalCondition(intManifestsApplied, metav1.ConditionFalse, string(intstatus.ConditionReasonApplyingManifests)),
			),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionFalse, reason: reason},
				ConditionManaged:              {status: metav1.ConditionFalse, reason: reason},
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: reason},
			},
		},
		{
			name: "a failure of the applying run is not masked",
			opts: running(
				withSettingsChanged(),
				withInternalCondition(intManifestsApplied, metav1.ConditionFalse, "HelmFailed"),
			),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
				ConditionManaged:              {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
			},
		},
		{
			name: "invalid settings replace the marker",
			opts: running(withInternalCondition(intConfigured, metav1.ConditionFalse, "InvalidSettings")),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionTrue, reason: ConditionReady},
				ConditionManaged:              {status: metav1.ConditionTrue, reason: ConditionManaged},
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "SettingsInvalid"},
			},
		},
		{
			name: "update resets Ready, Managed and ConfigurationApplied",
			opts: running(withVersionChanged(), withSettingsChanged()),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionFalse, reason: reason},
				ConditionManaged:              {status: metav1.ConditionFalse, reason: reason},
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: reason},
			},
		},
		{
			name: "first install is left to Installed",
			opts: []mappingOption{withSettingsChanged()},
			expected: map[string]*expectedCondition{
				ConditionInstalled:            nil,
				ConditionReady:                nil,
				ConditionManaged:              nil,
				ConditionConfigurationApplied: nil,
			},
		},
		{
			name: "applied settings restore the conditions",
			opts: running(),
			expected: map[string]*expectedCondition{
				ConditionReady:                {status: metav1.ConditionTrue, reason: ConditionReady},
				ConditionManaged:              {status: metav1.ConditionTrue, reason: ConditionManaged},
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: ConditionConfigurationApplied},
			},
		},
	}

	runTestCases(t, cases)
}

// withMaintenanceMode sets the MaintenanceMode the last successful Run recorded.
func withMaintenanceMode(status metav1.ConditionStatus) mappingOption {
	reason := ""
	if status == metav1.ConditionTrue {
		reason = string(intstatus.ConditionReasonNoResourceReconciliation)
	}

	return withInternalCondition(intMaintenanceMode, status, reason)
}

func TestMaintenanceMode(t *testing.T) {
	installed := func(opts ...mappingOption) []mappingOption {
		return append(append(withSuccessfulApply(),
			withExternalCondition(ConditionInstalled, metav1.ConditionTrue, ConditionInstalled)), opts...)
	}

	cases := []testCase{
		{
			name: "applied maintenance breaks Managed only",
			opts: installed(withMaintenanceMode(metav1.ConditionTrue)),
			expected: map[string]*expectedCondition{
				ConditionManaged:              {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonNoResourceReconciliation)},
				ConditionReady:                {status: metav1.ConditionTrue, reason: ConditionReady},
				ConditionScaled:               {status: metav1.ConditionTrue, reason: ConditionScaled},
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: ConditionConfigurationApplied},
			},
		},
		{
			name: "managed mode keeps Managed true",
			opts: installed(withMaintenanceMode(metav1.ConditionFalse)),
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: ConditionManaged},
			},
		},
		{
			name: "unknown mode before the first run keeps Managed true",
			opts: installed(withMaintenanceMode(metav1.ConditionUnknown)),
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: ConditionManaged},
			},
		},
		{
			name: "a failure outranks maintenance",
			opts: installed(withMaintenanceMode(metav1.ConditionTrue),
				withInternalCondition(string(intstatus.ConditionManifestsApplied), metav1.ConditionFalse, "boom")),
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "ManifestsApplyFailed"},
			},
		},
		{
			name: "maintenance outranks pending settings",
			opts: installed(withMaintenanceMode(metav1.ConditionTrue),
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionFalse, string(intstatus.ConditionReasonSettingsChanged))),
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: string(intstatus.ConditionReasonNoResourceReconciliation)},
			},
		},
	}

	runTestCases(t, cases)
}

func TestMaintenanceModeKeepsSummaryReady(t *testing.T) {
	state := condmap.State{
		Internal: make(map[string]metav1.Condition),
		External: make(map[string]metav1.Condition),
	}

	opts := append(withSuccessfulApply(),
		withExternalCondition(ConditionInstalled, metav1.ConditionTrue, ConditionInstalled),
		withMaintenanceMode(metav1.ConditionTrue))
	for _, opt := range opts {
		opt(&state)
	}

	summary, _, _ := summarize(state)
	assert.Equal(t, stateReady, summary)
}
