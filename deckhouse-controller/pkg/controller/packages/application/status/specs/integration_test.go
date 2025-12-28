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

package specs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

func TestDefaultSpecs_NoDuplicateCases(t *testing.T) {
	mapper := statusmapper.New(DefaultSpecs())
	warnings := mapper.DetectDuplicateCases()

	// No warnings expected for default specs
	assert.Empty(t, warnings, "default specs should have no duplicate cases")
}

func TestDefaultSpecs_FullWorkflow(t *testing.T) {
	mapper := statusmapper.New(DefaultSpecs())

	// Helper to convert results slice to map for easier assertions
	toCondMap := func(results []status.Condition) map[status.ConditionName]status.Condition {
		m := make(map[status.ConditionName]status.Condition, len(results))
		for _, c := range results {
			m[c.Name] = c
		}
		return m
	}

	// Helper to create internal conditions map from status pairs
	makeInternalConditions := func(pairs ...any) map[status.ConditionName]status.Condition {
		m := make(map[status.ConditionName]status.Condition)
		for i := 0; i < len(pairs); i += 2 {
			name := status.ConditionName(pairs[i].(string))
			st := pairs[i+1].(metav1.ConditionStatus)
			m[name] = status.Condition{Name: name, Status: st}
		}
		return m
	}

	tests := []struct {
		name               string
		internalConditions map[status.ConditionName]status.Condition
		isInitialInstall   bool
		versionChanged     bool
		expected           map[status.ConditionName]struct {
			status metav1.ConditionStatus
			reason string
		}
	}{
		{
			name:               "phase 1: downloading (initial)",
			internalConditions: makeInternalConditions("Downloaded", metav1.ConditionUnknown),
			isInitialInstall:   true,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:         {metav1.ConditionFalse, "Downloading"},
				status.ConditionReady:             {metav1.ConditionFalse, "NotReady"},
				status.ConditionPartiallyDegraded: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 2: downloaded, waiting for other conditions",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionUnknown,
			),
			isInitialInstall: true,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:         {metav1.ConditionFalse, "InstallationInProgress"},
				status.ConditionReady:             {metav1.ConditionFalse, "NotReady"},
				status.ConditionPartiallyDegraded: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 3: filesystem ready, installation in progress",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionTrue,
				"RequirementsMet", metav1.ConditionUnknown,
			),
			isInitialInstall: true,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:         {metav1.ConditionFalse, "InstallationInProgress"},
				status.ConditionReady:             {metav1.ConditionFalse, "NotReady"},
				status.ConditionPartiallyDegraded: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 4: all installed and ready",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionTrue,
				"RequirementsMet", metav1.ConditionTrue,
				"ReadyInRuntime", metav1.ConditionTrue,
				"HooksProcessed", metav1.ConditionTrue,
				"HelmApplied", metav1.ConditionTrue,
				"SettingsIsValid", metav1.ConditionTrue,
			),
			isInitialInstall: true,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:            {metav1.ConditionTrue, ""},
				status.ConditionReady:                {metav1.ConditionTrue, ""},
				status.ConditionPartiallyDegraded:    {metav1.ConditionFalse, ""},
				status.ConditionManaged:              {metav1.ConditionTrue, ""},
				status.ConditionConfigurationApplied: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "phase 5: update in progress (version changed, all conditions met)",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionTrue,
				"RequirementsMet", metav1.ConditionTrue,
				"ReadyInRuntime", metav1.ConditionTrue,
				"HooksProcessed", metav1.ConditionTrue,
				"HelmApplied", metav1.ConditionTrue,
				"SettingsIsValid", metav1.ConditionTrue,
			),
			isInitialInstall: false,
			versionChanged:   true,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:       {metav1.ConditionTrue, ""},
				status.ConditionReady:           {metav1.ConditionTrue, ""},
				status.ConditionUpdateInstalled: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "degraded: runtime not ready",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionTrue,
				"RequirementsMet", metav1.ConditionTrue,
				"ReadyInRuntime", metav1.ConditionFalse,
				"HooksProcessed", metav1.ConditionTrue,
				"HelmApplied", metav1.ConditionTrue,
				"SettingsIsValid", metav1.ConditionTrue,
			),
			isInitialInstall: false,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:         {metav1.ConditionFalse, "InstallationInProgress"},
				status.ConditionReady:             {metav1.ConditionFalse, "NotReady"},
				status.ConditionPartiallyDegraded: {metav1.ConditionTrue, ""},
			},
		},
		{
			name: "configuration invalid",
			internalConditions: makeInternalConditions(
				"Downloaded", metav1.ConditionTrue,
				"ReadyOnFilesystem", metav1.ConditionTrue,
				"RequirementsMet", metav1.ConditionTrue,
				"ReadyInRuntime", metav1.ConditionTrue,
				"HooksProcessed", metav1.ConditionTrue,
				"HelmApplied", metav1.ConditionTrue,
				"SettingsIsValid", metav1.ConditionFalse,
			),
			isInitialInstall: false,
			expected: map[status.ConditionName]struct {
				status metav1.ConditionStatus
				reason string
			}{
				status.ConditionInstalled:            {metav1.ConditionTrue, ""},
				status.ConditionReady:                {metav1.ConditionTrue, ""},
				status.ConditionConfigurationApplied: {metav1.ConditionFalse, "ConfigurationValidationFailed"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &statusmapper.Input{
				InternalConditions: tt.internalConditions,
				ExternalConditions: make(map[status.ConditionName]status.Condition),
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
					assert.Equal(t, status.ConditionReason(exp.reason), cond.Reason, "condition %s reason mismatch", condType)
				}
			}
		})
	}
}
