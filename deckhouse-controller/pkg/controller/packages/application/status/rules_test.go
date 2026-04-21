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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmapper"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

type mappingOption func(state *condmapper.State)

func withInternalCondition(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return func(state *condmapper.State) {
		state.Internal[cond] = metav1.Condition{
			Type:   cond,
			Status: status,
			Reason: reason,
		}
	}
}

func withExternalCondition(cond string, status metav1.ConditionStatus, reason string) mappingOption {
	return func(state *condmapper.State) {
		state.External[cond] = metav1.Condition{
			Type:   cond,
			Status: status,
			Reason: reason,
		}
	}
}

func withVersionChanged() mappingOption {
	return func(state *condmapper.State) {
		state.VersionChanged = true
	}
}

func testMapping(opts ...mappingOption) map[string]metav1.Condition {
	state := &condmapper.State{
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
			name: "true when ReadyInCluster and no version change",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
		{
			name: "not true when version changed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: nil,
			},
		},
		{
			name: "false when ReadyOnFilesystem is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "MountFailed"},
			},
		},
		{
			name: "false when ReadyInRuntime is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeError"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RuntimeError"},
			},
		},
		{
			name: "false when ReadyInCluster is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterNotReady"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "ClusterNotReady"},
			},
		},
		{
			name: "sticky - not in result when already true externally",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "PreviouslyInstalled"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterNotReady"),
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
			name: "true when ReadyInCluster and version changed",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
		{
			name: "absent when not installed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
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
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
		{
			name: "absent after fresh install with no updates",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				// No UpdateInstalled in external state = no update ever happened
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil, // Should not be present for fresh installs
			},
		},
		{
			name: "false when core condition fails during update",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionFalse, reason: "MountFailed"},
			},
		},
		{
			name: "absent when core condition fails without version change",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
			},
			expected: map[string]*expectedCondition{
				// FalseIf only triggers when version changed, so no update to this condition
				ConditionUpdateInstalled: nil,
			},
		},
	}

	runTestCases(t, cases)
}

func TestReadyRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when ReadyInCluster",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
		{
			name: "false when core condition fails",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "MountFailed"},
			},
		},
		{
			// DH restart: runtime is rebuilding but the workload in the cluster keeps running.
			name: "stays true when ReadyInRuntime is Pending but ReadyInCluster is true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, string(intstatus.ConditionReasonPending)),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
		{
			// Non-Pending runtime failure still flips Ready.
			name: "false when ReadyInRuntime is false for non-Pending reason",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeCrashed"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "RuntimeCrashed"},
			},
		},
		{
			// Pending runtime must not mask a real filesystem failure.
			name: "false when filesystem fails even if ReadyInRuntime is Pending",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyOnFilesystem), metav1.ConditionFalse, "MountFailed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, string(intstatus.ConditionReasonPending)),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "MountFailed"},
			},
		},
	}

	runTestCases(t, cases)
}

func TestReadyPendingIsUnmanaged(t *testing.T) {
	// Companion to TestReadyRule: during a DH restart (ReadyInRuntime=Pending),
	// Ready stays True but Managed goes False because DH can't drive the package.
	opts := []mappingOption{
		withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
		withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
		withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, string(intstatus.ConditionReasonPending)),
		withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
	}

	result := testMapping(opts...)

	if assert.Contains(t, result, ConditionReady) {
		assert.Equal(t, metav1.ConditionTrue, result[ConditionReady].Status, "Ready should stay True")
	}
	if assert.Contains(t, result, ConditionManaged) {
		assert.Equal(t, metav1.ConditionFalse, result[ConditionManaged].Status, "Managed should be False while runtime is Pending")
		assert.Equal(t, string(intstatus.ConditionReasonPending), result[ConditionManaged].Reason)
	}
}

func TestPartiallyDegradedRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when ReadyInCluster is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterDegraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionTrue, reason: "ClusterDegraded"},
			},
		},
		{
			// ReadyInRuntime is DH-internal state (e.g. Pending during DH restart).
			// Not user-visible degradation.
			name: "false when only ReadyInRuntime is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeDegraded"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionFalse, reason: "ClusterReady"},
			},
		},
		{
			// HooksReady is DH-internal processing state, surfaced via ConfigurationApplied.
			name: "false when only HooksReady is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionFalse, reason: "ClusterReady"},
			},
		},
		{
			name: "false when ReadyInCluster is true",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionFalse, reason: "ClusterReady"},
			},
		},
		{
			// DH restart: runtime is rebuilding but the cluster workload is still healthy.
			name: "false when ReadyInRuntime is Pending and cluster is true",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, string(intstatus.ConditionReasonPending)),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionFalse, reason: "ClusterReady"},
			},
		},
		{
			name: "absent when not installed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterDegraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: nil,
			},
		},
	}

	runTestCases(t, cases)
}

func TestManagedRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when all managed conditions true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: "RuntimeReady"},
			},
		},
		{
			name: "false when ReadyInRuntime is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeNotReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "RuntimeNotReady"},
			},
		},
		{
			name: "false when ReadyInCluster is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterNotReady"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "ClusterNotReady"},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionFalse, "HooksFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "HooksFailed"},
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
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: "SettingsOK"},
			},
		},
		{
			name: "false when SettingsValid is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionFalse, "InvalidSettings"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "InvalidSettings"},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "HooksFailed"},
			},
		},
		{
			name: "false when HelmApplied is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionConfigured), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksReady), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionFalse, "HelmFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "HelmFailed"},
			},
		},
	}

	runTestCases(t, cases)
}
