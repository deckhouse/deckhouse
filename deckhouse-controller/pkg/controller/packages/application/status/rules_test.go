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
			name: "false when Downloaded is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionDownloaded), metav1.ConditionFalse, "DownloadFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "DownloadFailed"},
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
			name: "false when RequirementsMet is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "RequirementsNotMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "RequirementsNotMet"},
			},
		},
		{
			name: "sticky - not updated when already true externally",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "PreviouslyInstalled"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterNotReady"),
			},
			expected: map[string]*expectedCondition{
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
			name: "absent when no version change",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: nil,
			},
		},
		{
			name: "false when core condition fails",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionDownloaded), metav1.ConditionFalse, "DownloadFailed"),
				withVersionChanged(),
			},
			expected: map[string]*expectedCondition{
				ConditionUpdateInstalled: {status: metav1.ConditionFalse, reason: "DownloadFailed"},
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
				withInternalCondition(string(intstatus.ConditionDownloaded), metav1.ConditionFalse, "DownloadFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "DownloadFailed"},
			},
		},
		{
			name: "false when not installed and WaitConverge",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionWaitConverge), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "Waiting"},
			},
		},
		{
			name: "false when not installed and RequirementsMet",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionTrue, "RequirementsMet"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionFalse, reason: "RequirementsMet"},
			},
		},
		{
			name: "true when installed even with WaitConverge",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "Ready"),
				withInternalCondition(string(intstatus.ConditionWaitConverge), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionReady: {status: metav1.ConditionTrue, reason: "Ready"},
			},
		},
	}

	runTestCases(t, cases)
}

func TestPartiallyDegradedRule(t *testing.T) {
	cases := []testCase{
		{
			name: "true when ReadyInRuntime is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeDegraded"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionTrue, reason: "RuntimeDegraded"},
			},
		},
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
			name: "true when HooksProcessed is false",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionTrue, reason: "HooksFailed"},
			},
		},
		{
			name: "false when all managed conditions true",
			opts: []mappingOption{
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionPartiallyDegraded: {status: metav1.ConditionFalse, reason: "RuntimeReady"},
			},
		},
		{
			name: "absent when not installed",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeDegraded"),
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
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionTrue, reason: "RuntimeReady"},
			},
		},
		{
			name: "false when ReadyInRuntime is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionFalse, "RuntimeNotReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "RuntimeNotReady"},
			},
		},
		{
			name: "false when ReadyInCluster is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse, "ClusterNotReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "ClusterNotReady"},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "HooksFailed"},
			},
		},
		{
			name: "false when WaitConverge is true",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionReadyInRuntime), metav1.ConditionTrue, "RuntimeReady"),
				withInternalCondition(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue, "ClusterReady"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionWaitConverge), metav1.ConditionTrue, "Waiting"),
			},
			expected: map[string]*expectedCondition{
				ConditionManaged: {status: metav1.ConditionFalse, reason: "Waiting"},
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
				withInternalCondition(string(intstatus.ConditionSettingsValid), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionTrue, reason: "SettingsOK"},
			},
		},
		{
			name: "false when SettingsValid is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionSettingsValid), metav1.ConditionFalse, "InvalidSettings"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "InvalidSettings"},
			},
		},
		{
			name: "false when HooksProcessed is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionSettingsValid), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionFalse, "HooksFailed"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionTrue, "HelmOK"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "HooksFailed"},
			},
		},
		{
			name: "false when HelmApplied is false",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionSettingsValid), metav1.ConditionTrue, "SettingsOK"),
				withInternalCondition(string(intstatus.ConditionHooksProcessed), metav1.ConditionTrue, "HooksOK"),
				withInternalCondition(string(intstatus.ConditionHelmApplied), metav1.ConditionFalse, "HelmFailed"),
			},
			expected: map[string]*expectedCondition{
				ConditionConfigurationApplied: {status: metav1.ConditionFalse, reason: "HelmFailed"},
			},
		},
	}

	runTestCases(t, cases)
}
