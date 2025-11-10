/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packagestatusservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	applicationpackage "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/application-package"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestMapPackageStatuses(t *testing.T) {
	tests := []struct {
		name     string
		input    []applicationpackage.PackageStatus
		expected []v1alpha1.ApplicationStatusCondition
	}{
		{
			name: "all known types",
			input: []applicationpackage.PackageStatus{
				{Type: "requirementsMet", Status: true, Reason: "AllRequirementsMet", Message: "OK"},
				{Type: "startupHooksSuccessful", Status: false, Reason: "HookFailed", Message: "Failed"},
				{Type: "manifestsDeployed", Status: true, Reason: "Deployed", Message: "OK"},
				{Type: "replicasAvailable", Status: true, Reason: "Available", Message: "OK"},
			},
			expected: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "AllRequirementsMet", Message: "OK"},
				{Type: v1alpha1.ApplicationConditionStartupHooksSuccessful, Status: corev1.ConditionFalse, Reason: "HookFailed", Message: "Failed"},
				{Type: v1alpha1.ApplicationConditionManifestsDeployed, Status: corev1.ConditionTrue, Reason: "Deployed", Message: "OK"},
				{Type: v1alpha1.ApplicationConditionReplicasAvailable, Status: corev1.ConditionTrue, Reason: "Available", Message: "OK"},
			},
		},
		{
			name: "unknown type is skipped",
			input: []applicationpackage.PackageStatus{
				{Type: "requirementsMet", Status: true, Reason: "OK", Message: "OK"},
				{Type: "unknownType", Status: false, Reason: "Fail", Message: "Fail"},
			},
			expected: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK"},
			},
		},
		{
			name:     "empty input",
			input:    []applicationpackage.PackageStatus{},
			expected: []v1alpha1.ApplicationStatusCondition{},
		},
		{
			name: "bool to condition status conversion",
			input: []applicationpackage.PackageStatus{
				{Type: "requirementsMet", Status: true, Reason: "OK", Message: "OK"},
				{Type: "manifestsDeployed", Status: false, Reason: "Fail", Message: "Fail"},
			},
			expected: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK"},
				{Type: v1alpha1.ApplicationConditionManifestsDeployed, Status: corev1.ConditionFalse, Reason: "Fail", Message: "Fail"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewNop()
			ev := PackageEvent{Namespace: "test", Name: "test", PackageName: "test"}
			result := mapPackageStatuses(tt.input, logger, ev)
			// Sort both for comparison
			sortConditions(result)
			sortConditions(tt.expected)

			require.Len(t, result, len(tt.expected))
			for i := range result {
				assert.Equal(t, tt.expected[i].Type, result[i].Type)
				assert.Equal(t, tt.expected[i].Status, result[i].Status)
				assert.Equal(t, tt.expected[i].Reason, result[i].Reason)
				assert.Equal(t, tt.expected[i].Message, result[i].Message)
			}
		})
	}
}

func TestMergeConditions(t *testing.T) {
	now := metav1.Now()
	past := metav1.NewTime(now.Add(-1 * time.Hour))

	tests := []struct {
		name     string
		existing []v1alpha1.ApplicationStatusCondition
		incoming []v1alpha1.ApplicationStatusCondition
		validate func(t *testing.T, result []v1alpha1.ApplicationStatusCondition)
	}{
		{
			name:     "add new condition",
			existing: []v1alpha1.ApplicationStatusCondition{},
			incoming: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK"},
			},
			validate: func(t *testing.T, result []v1alpha1.ApplicationStatusCondition) {
				require.Len(t, result, 1)
				assert.Equal(t, v1alpha1.ApplicationConditionRequirementsMet, result[0].Type)
				assert.Equal(t, corev1.ConditionTrue, result[0].Status)
				assert.False(t, result[0].LastTransitionTime.IsZero())
			},
		},
		{
			name: "update existing condition - status changes",
			existing: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionFalse, Reason: "Old", Message: "Old", LastTransitionTime: past},
			},
			incoming: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "New", Message: "New"},
			},
			validate: func(t *testing.T, result []v1alpha1.ApplicationStatusCondition) {
				require.Len(t, result, 1)
				assert.Equal(t, corev1.ConditionTrue, result[0].Status)
				assert.Equal(t, "New", result[0].Reason)
				assert.Equal(t, "New", result[0].Message)
				// LastTransitionTime should be updated when status changes
				assert.True(t, result[0].LastTransitionTime.After(past.Time))
			},
		},
		{
			name: "update existing condition - only reason/message changes",
			existing: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "Old", Message: "Old", LastTransitionTime: past},
			},
			incoming: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "New", Message: "New"},
			},
			validate: func(t *testing.T, result []v1alpha1.ApplicationStatusCondition) {
				require.Len(t, result, 1)
				assert.Equal(t, corev1.ConditionTrue, result[0].Status)
				assert.Equal(t, "New", result[0].Reason)
				assert.Equal(t, "New", result[0].Message)
				// LastTransitionTime should NOT change when status doesn't change
				assert.Equal(t, past, result[0].LastTransitionTime)
			},
		},
		{
			name: "idempotent merge",
			existing: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK", LastTransitionTime: past},
			},
			incoming: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK"},
			},
			validate: func(t *testing.T, result []v1alpha1.ApplicationStatusCondition) {
				require.Len(t, result, 1)
				assert.Equal(t, past, result[0].LastTransitionTime)
			},
		},
		{
			name: "multiple conditions",
			existing: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK", LastTransitionTime: past},
			},
			incoming: []v1alpha1.ApplicationStatusCondition{
				{Type: v1alpha1.ApplicationConditionRequirementsMet, Status: corev1.ConditionTrue, Reason: "OK", Message: "OK"},
				{Type: v1alpha1.ApplicationConditionManifestsDeployed, Status: corev1.ConditionFalse, Reason: "Fail", Message: "Fail"},
			},
			validate: func(t *testing.T, result []v1alpha1.ApplicationStatusCondition) {
				require.Len(t, result, 2)
				// Should be sorted by Type
				assert.Equal(t, v1alpha1.ApplicationConditionManifestsDeployed, result[0].Type)
				assert.Equal(t, v1alpha1.ApplicationConditionRequirementsMet, result[1].Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeConditions(tt.existing, tt.incoming)
			tt.validate(t, result)
		})
	}
}

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"requirementsMet", v1alpha1.ApplicationConditionRequirementsMet},
		{"startupHooksSuccessful", v1alpha1.ApplicationConditionStartupHooksSuccessful},
		{"manifestsDeployed", v1alpha1.ApplicationConditionManifestsDeployed},
		{"replicasAvailable", v1alpha1.ApplicationConditionReplicasAvailable},
		{"unknownType", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBoolToCondStatus(t *testing.T) {
	assert.Equal(t, corev1.ConditionTrue, boolToCondStatus(true))
	assert.Equal(t, corev1.ConditionFalse, boolToCondStatus(false))
}

func TestConditionsEqual(t *testing.T) {
	now := metav1.Now()
	past := metav1.NewTime(now.Add(-1 * time.Hour))

	tests := []struct {
		name     string
		a        []v1alpha1.ApplicationStatusCondition
		b        []v1alpha1.ApplicationStatusCondition
		expected bool
	}{
		{
			name: "equal conditions",
			a: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue, Reason: "R1", Message: "M1", LastTransitionTime: now},
			},
			b: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue, Reason: "R1", Message: "M1", LastTransitionTime: past},
			},
			expected: true, // LTT ignored
		},
		{
			name: "different status",
			a: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue},
			},
			b: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionFalse},
			},
			expected: false,
		},
		{
			name: "different reason",
			a: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue, Reason: "R1"},
			},
			b: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue, Reason: "R2"},
			},
			expected: false,
		},
		{
			name:     "both empty",
			a:        []v1alpha1.ApplicationStatusCondition{},
			b:        []v1alpha1.ApplicationStatusCondition{},
			expected: true,
		},
		{
			name: "different length",
			a: []v1alpha1.ApplicationStatusCondition{
				{Type: "A", Status: corev1.ConditionTrue},
			},
			b:        []v1alpha1.ApplicationStatusCondition{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortConditions(tt.a)
			sortConditions(tt.b)
			result := conditionsEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
