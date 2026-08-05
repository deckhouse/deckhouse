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
