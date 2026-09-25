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
		withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionTrue, "Enabled"),
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

func TestEnabledRule(t *testing.T) {
	cases := []testCase{
		{
			name: "absent before the first scheduling decision",
			opts: []mappingOption{},
			expected: map[string]*expectedCondition{
				ConditionEnabled: nil,
			},
		},
		{
			name: "absent while the scheduler verdict is unknown",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionUnknown, ""),
			},
			expected: map[string]*expectedCondition{
				ConditionEnabled: nil,
			},
		},
		{
			name: "true when the module is scheduled",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionTrue, "Enabled"),
			},
			expected: map[string]*expectedCondition{
				// True conditions use the external condition type as reason — emit() drops the internal one.
				ConditionEnabled: {status: metav1.ConditionTrue, reason: ConditionEnabled},
			},
		},
		{
			name: "false with the user disable reason passed through",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "Disabled"),
			},
			expected: map[string]*expectedCondition{
				ConditionEnabled: {status: metav1.ConditionFalse, reason: "Disabled"},
			},
		},
		{
			name: "false with the bundle reason passed through",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DisabledByBundle"),
			},
			expected: map[string]*expectedCondition{
				ConditionEnabled: {status: metav1.ConditionFalse, reason: "DisabledByBundle"},
			},
		},
	}

	runTestCases(t, cases)
}

// TestSchedulerReasonPassthrough covers the module-specific canonicalReason
// behavior: a scheduler forbid on the install pipeline surfaces its own
// decision reason instead of the application's collapsed RequirementsUnmet.
func TestSchedulerReasonPassthrough(t *testing.T) {
	cases := []testCase{
		{
			name: "install blocked by a dependency carries the scheduler reason",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			},
			expected: map[string]*expectedCondition{
				ConditionEnabled:   {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
				ConditionReady:     {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
			},
		},
		{
			name: "install blocked by an explicit disable carries the scheduler reason",
			opts: []mappingOption{
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "Disabled"),
			},
			expected: map[string]*expectedCondition{
				ConditionEnabled:   {status: metav1.ConditionFalse, reason: "Disabled"},
				ConditionInstalled: {status: metav1.ConditionFalse, reason: "Disabled"},
				ConditionReady:     {status: metav1.ConditionFalse, reason: "Disabled"},
			},
		},
	}

	runTestCases(t, cases)
}

// TestDisabledModule covers the isDisabled branch: the scheduler switches a
// previously-installed module off. User-facing signals go False with the
// scheduler reason, runtime and configuration signals go Unknown, and
// UpdateInstalled falls silent.
func TestDisabledModule(t *testing.T) {
	cases := []testCase{
		{
			name: "installed module switched off by the user",
			opts: append(withSuccessfulApply(),
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "Disabled"),
			),
			expected: map[string]*expectedCondition{
				ConditionEnabled:              {status: metav1.ConditionFalse, reason: "Disabled"},
				ConditionInstalled:            {status: metav1.ConditionFalse, reason: "Disabled"},
				ConditionReady:                {status: metav1.ConditionFalse, reason: "Disabled"},
				ConditionScaled:               {status: metav1.ConditionUnknown, reason: "Disabled"},
				ConditionManaged:              {status: metav1.ConditionUnknown, reason: "Disabled"},
				ConditionConfigurationApplied: {status: metav1.ConditionUnknown, reason: "Disabled"},
				ConditionUpdateInstalled:      nil,
			},
		},
		{
			name: "installed module switched off by a lost dependency",
			opts: append(withSuccessfulApply(),
				withExternalCondition(ConditionInstalled, metav1.ConditionTrue, "Installed"),
				withInternalCondition(string(intstatus.ConditionRequirementsMet), metav1.ConditionFalse, "DependencyNotEnabled"),
			),
			expected: map[string]*expectedCondition{
				ConditionEnabled:              {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
				ConditionInstalled:            {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
				ConditionReady:                {status: metav1.ConditionFalse, reason: "DependencyNotEnabled"},
				ConditionScaled:               {status: metav1.ConditionUnknown, reason: "DependencyNotEnabled"},
				ConditionManaged:              {status: metav1.ConditionUnknown, reason: "DependencyNotEnabled"},
				ConditionConfigurationApplied: {status: metav1.ConditionUnknown, reason: "DependencyNotEnabled"},
				ConditionUpdateInstalled:      nil,
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
	}

	runTestCases(t, cases)
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
