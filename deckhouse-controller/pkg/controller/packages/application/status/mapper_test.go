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
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/specs"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

func TestDetectDuplicateMappingRules(t *testing.T) {
	mapper := NewConditionMapper(specs.DefaultSpecs())
	warnings := mapper.DetectDuplicateMappingRules()

	// No warnings expected for default specs
	assert.Empty(t, warnings, "default specs should have no duplicate rules")
}

func TestDetectDuplicateMappingRules_WithDuplicates(t *testing.T) {
	testSpecs := []types.MappingSpec{
		{
			Type: "Test",
			MappingRules: []types.MappingRule{
				{Name: "rule1", Matcher: types.Always{}, Status: corev1.ConditionTrue},
				{Name: "rule2", Matcher: types.Always{}, Status: corev1.ConditionFalse}, // shadowed
			},
		},
	}

	mapper := NewConditionMapper(testSpecs)
	warnings := mapper.DetectDuplicateMappingRules()
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "shadowed")
}

func TestConditionMapper_FullWorkflow(t *testing.T) {
	mapper := NewConditionMapper(specs.DefaultSpecs())

	// Helper to convert results slice to map for easier assertions
	toCondMap := func(results []types.ExternalCondition) map[types.ExternalConditionType]types.ExternalCondition {
		m := make(map[types.ExternalConditionType]types.ExternalCondition, len(results))
		for _, c := range results {
			m[c.Type] = c
		}
		return m
	}

	// Helper to create internal conditions map from status pairs
	makeInternals := func(pairs ...any) map[string]types.InternalCondition {
		m := make(map[string]types.InternalCondition)
		for i := 0; i < len(pairs); i += 2 {
			name := pairs[i].(string)
			status := pairs[i+1].(corev1.ConditionStatus)
			m[name] = types.InternalCondition{Name: name, Status: status}
		}
		return m
	}

	tests := []struct {
		name             string
		internals        map[string]types.InternalCondition
		isInitialInstall bool
		versionChanged   bool
		expected         map[types.ExternalConditionType]struct {
			status corev1.ConditionStatus
			reason string
		}
	}{
		{
			name:             "phase 1: downloading (initial)",
			internals:        makeInternals("Downloaded", corev1.ConditionUnknown),
			isInitialInstall: true,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:         {corev1.ConditionFalse, "Downloading"},
				types.ConditionReady:             {corev1.ConditionFalse, "NotReady"},
				types.ConditionPartiallyDegraded: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 2: downloaded, waiting for other conditions",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionUnknown,
			),
			isInitialInstall: true,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:         {corev1.ConditionFalse, "InstallationInProgress"},
				types.ConditionReady:             {corev1.ConditionFalse, "NotReady"},
				types.ConditionPartiallyDegraded: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 3: filesystem ready, installation in progress",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionTrue,
				"RequirementsMet", corev1.ConditionUnknown,
			),
			isInitialInstall: true,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:         {corev1.ConditionFalse, "InstallationInProgress"},
				types.ConditionReady:             {corev1.ConditionFalse, "NotReady"},
				types.ConditionPartiallyDegraded: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 4: all installed and ready",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionTrue,
				"RequirementsMet", corev1.ConditionTrue,
				"ReadyInRuntime", corev1.ConditionTrue,
				"HooksProcessed", corev1.ConditionTrue,
				"HelmApplied", corev1.ConditionTrue,
				"SettingsIsValid", corev1.ConditionTrue,
			),
			isInitialInstall: true,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:            {corev1.ConditionTrue, ""},
				types.ConditionReady:                {corev1.ConditionTrue, ""},
				types.ConditionPartiallyDegraded:    {corev1.ConditionFalse, ""},
				types.ConditionManaged:              {corev1.ConditionTrue, ""},
				types.ConditionConfigurationApplied: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 5: update in progress (version changed, all conditions met)",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionTrue,
				"RequirementsMet", corev1.ConditionTrue,
				"ReadyInRuntime", corev1.ConditionTrue,
				"HooksProcessed", corev1.ConditionTrue,
				"HelmApplied", corev1.ConditionTrue,
				"SettingsIsValid", corev1.ConditionTrue,
			),
			isInitialInstall: false,
			versionChanged:   true,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:       {corev1.ConditionTrue, ""},
				types.ConditionReady:           {corev1.ConditionTrue, ""},
				types.ConditionUpdateInstalled: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "degraded: runtime not ready",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionTrue,
				"RequirementsMet", corev1.ConditionTrue,
				"ReadyInRuntime", corev1.ConditionFalse,
				"HooksProcessed", corev1.ConditionTrue,
				"HelmApplied", corev1.ConditionTrue,
				"SettingsIsValid", corev1.ConditionTrue,
			),
			isInitialInstall: false,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:         {corev1.ConditionFalse, "InstallationInProgress"},
				types.ConditionReady:             {corev1.ConditionFalse, "NotReady"},
				types.ConditionPartiallyDegraded: {corev1.ConditionTrue, ""},
			},
		},
		{
			name: "configuration invalid",
			internals: makeInternals(
				"Downloaded", corev1.ConditionTrue,
				"ReadyOnFilesystem", corev1.ConditionTrue,
				"RequirementsMet", corev1.ConditionTrue,
				"ReadyInRuntime", corev1.ConditionTrue,
				"HooksProcessed", corev1.ConditionTrue,
				"HelmApplied", corev1.ConditionTrue,
				"SettingsIsValid", corev1.ConditionFalse,
			),
			isInitialInstall: false,
			expected: map[types.ExternalConditionType]struct {
				status corev1.ConditionStatus
				reason string
			}{
				types.ConditionInstalled:            {corev1.ConditionTrue, ""},
				types.ConditionReady:                {corev1.ConditionTrue, ""},
				types.ConditionConfigurationApplied: {corev1.ConditionFalse, "ConfigurationValidationFailed"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types.MappingInput{
				InternalConditions: tt.internals,
				CurrentConditions:  make(map[types.ExternalConditionType]types.ExternalCondition),
				App:                &v1alpha1.Application{},
				IsInitialInstall:   tt.isInitialInstall,
				VersionChanged:     tt.versionChanged,
			}

			results := mapper.Map(input)
			condMap := toCondMap(results)

			for condType, exp := range tt.expected {
				cond, ok := condMap[condType]
				assert.True(t, ok, "expected condition %s to be present", condType)
				assert.Equal(t, exp.status, cond.Status, "condition %s status mismatch", condType)
				if exp.reason != "" {
					assert.Equal(t, exp.reason, cond.Reason, "condition %s reason mismatch", condType)
				}
			}
		})
	}
}

func TestMatcher_String(t *testing.T) {
	tests := []struct {
		name     string
		matcher  types.Matcher
		expected string
	}{
		{"Always", types.Always{}, "Always"},
		{"InternalIs", types.InternalIs{Name: "Test", Status: corev1.ConditionTrue}, "Test=True"},
		{"InternalNotTrue", types.InternalNotTrue{Name: "Test"}, "Test!=True"},
		{"AllOf", types.AllOf{types.InternalTrue("A"), types.InternalTrue("B")}, "AllOf(A=True AND B=True)"},
		{"AnyOf", types.AnyOf{types.InternalTrue("A"), types.InternalTrue("B")}, "AnyOf(A=True OR B=True)"},
		{"Predicate", types.Predicate{Name: "custom"}, "Predicate(custom)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.matcher.String())
		})
	}
}
