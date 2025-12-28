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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/specs"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// =============================================================================
// buildInput Tests
// =============================================================================

func TestBuildInput_IsInitialInstall(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name            string
		externalConds   []v1alpha1.ApplicationStatusCondition
		expectedInitial bool
	}{
		{
			name:            "no conditions - initial install",
			externalConds:   nil,
			expectedInitial: true,
		},
		{
			name: "Installed=False - initial install",
			externalConds: []v1alpha1.ApplicationStatusCondition{
				{Type: "Installed", Status: corev1.ConditionFalse},
			},
			expectedInitial: true,
		},
		{
			name: "Installed=Unknown - initial install",
			externalConds: []v1alpha1.ApplicationStatusCondition{
				{Type: "Installed", Status: corev1.ConditionUnknown},
			},
			expectedInitial: true,
		},
		{
			name: "Installed=True - not initial install",
			externalConds: []v1alpha1.ApplicationStatusCondition{
				{Type: "Installed", Status: corev1.ConditionTrue},
			},
			expectedInitial: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &v1alpha1.Application{
				Status: v1alpha1.ApplicationStatus{
					Conditions: tt.externalConds,
				},
			}

			input := svc.buildInput(app, nil)

			assert.Equal(t, tt.expectedInitial, input.IsInitialInstall)
		})
	}
}

func TestBuildInput_VersionChanged(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name            string
		specVersion     string
		currentVersion  *v1alpha1.ApplicationStatusVersion
		expectedChanged bool
	}{
		{
			name:            "no current version - not changed",
			specVersion:     "1.0.0",
			currentVersion:  nil,
			expectedChanged: false,
		},
		{
			name:            "empty current version - not changed",
			specVersion:     "1.0.0",
			currentVersion:  &v1alpha1.ApplicationStatusVersion{Current: ""},
			expectedChanged: false,
		},
		{
			name:            "same version - not changed",
			specVersion:     "1.0.0",
			currentVersion:  &v1alpha1.ApplicationStatusVersion{Current: "1.0.0"},
			expectedChanged: false,
		},
		{
			name:            "different version - changed",
			specVersion:     "2.0.0",
			currentVersion:  &v1alpha1.ApplicationStatusVersion{Current: "1.0.0"},
			expectedChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Version: tt.specVersion,
				},
				Status: v1alpha1.ApplicationStatus{
					CurrentVersion: tt.currentVersion,
				},
			}

			input := svc.buildInput(app, nil)

			assert.Equal(t, tt.expectedChanged, input.VersionChanged)
		})
	}
}

func TestBuildInput_MapsConditions(t *testing.T) {
	svc := newTestService()

	internalConds := []status.Condition{
		{Name: "Downloaded", Status: metav1.ConditionTrue, Reason: "Success"},
		{Name: "ReadyInRuntime", Status: metav1.ConditionFalse, Reason: "Failed", Message: "error"},
	}

	app := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{Type: "Installed", Status: corev1.ConditionTrue, Reason: "Ready"},
				{Type: "Ready", Status: corev1.ConditionFalse, Reason: "NotReady", Message: "msg"},
			},
		},
	}

	input := svc.buildInput(app, internalConds)

	// Check internal conditions mapped correctly
	require.Len(t, input.InternalConditions, 2)
	assert.Equal(t, metav1.ConditionTrue, input.InternalConditions["Downloaded"].Status)
	assert.Equal(t, status.ConditionReason("Success"), input.InternalConditions["Downloaded"].Reason)
	assert.Equal(t, metav1.ConditionFalse, input.InternalConditions["ReadyInRuntime"].Status)
	assert.Equal(t, "error", input.InternalConditions["ReadyInRuntime"].Message)

	// Check external conditions mapped correctly
	require.Len(t, input.ExternalConditions, 2)
	assert.Equal(t, metav1.ConditionTrue, input.ExternalConditions["Installed"].Status)
	assert.Equal(t, metav1.ConditionFalse, input.ExternalConditions["Ready"].Status)
	assert.Equal(t, "msg", input.ExternalConditions["Ready"].Message)
}

// =============================================================================
// setCondition Tests
// =============================================================================

func TestSetCondition_AddsNewCondition(t *testing.T) {
	svc := newTestService()
	app := &v1alpha1.Application{}
	now := metav1.Now()

	svc.setCondition(app, "Ready", metav1.ConditionTrue, "AllGood", "message", now)

	require.Len(t, app.Status.Conditions, 1)
	cond := app.Status.Conditions[0]
	assert.Equal(t, "Ready", cond.Type)
	assert.Equal(t, corev1.ConditionTrue, cond.Status)
	assert.Equal(t, "AllGood", cond.Reason)
	assert.Equal(t, "message", cond.Message)
	assert.Equal(t, now, cond.LastTransitionTime)
}

func TestSetCondition_UpdatesExisting_SameStatus_PreservesTime(t *testing.T) {
	svc := newTestService()
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	app := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{
					Type:               "Ready",
					Status:             corev1.ConditionTrue,
					Reason:             "OldReason",
					LastTransitionTime: oldTime,
				},
			},
		},
	}
	now := metav1.Now()

	// Same status (True -> True) - should preserve LastTransitionTime
	svc.setCondition(app, "Ready", metav1.ConditionTrue, "NewReason", "new message", now)

	require.Len(t, app.Status.Conditions, 1)
	cond := app.Status.Conditions[0]
	assert.Equal(t, "Ready", cond.Type)
	assert.Equal(t, corev1.ConditionTrue, cond.Status)
	assert.Equal(t, "NewReason", cond.Reason)
	assert.Equal(t, "new message", cond.Message)
	// LastTransitionTime should be preserved because status didn't change
	assert.Equal(t, oldTime, cond.LastTransitionTime)
	// LastProbeTime should be updated
	assert.Equal(t, now, cond.LastProbeTime)
}

func TestSetCondition_UpdatesExisting_DifferentStatus_UpdatesTime(t *testing.T) {
	svc := newTestService()
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	app := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Conditions: []v1alpha1.ApplicationStatusCondition{
				{
					Type:               "Ready",
					Status:             corev1.ConditionFalse,
					Reason:             "OldReason",
					LastTransitionTime: oldTime,
				},
			},
		},
	}
	now := metav1.Now()

	// Different status (False -> True) - should update LastTransitionTime
	svc.setCondition(app, "Ready", metav1.ConditionTrue, "NewReason", "", now)

	require.Len(t, app.Status.Conditions, 1)
	cond := app.Status.Conditions[0]
	assert.Equal(t, corev1.ConditionTrue, cond.Status)
	// LastTransitionTime should be updated because status changed
	assert.Equal(t, now, cond.LastTransitionTime)
}

// =============================================================================
// applyInternalConditions Tests
// =============================================================================

func TestApplyInternalConditions_PreservesLastTransitionTime(t *testing.T) {
	svc := newTestService()
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	app := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			InternalConditions: []v1alpha1.ApplicationInternalStatusCondition{
				{
					Type:               "Downloaded",
					Status:             corev1.ConditionTrue,
					LastTransitionTime: oldTime,
				},
				{
					Type:               "ReadyInRuntime",
					Status:             corev1.ConditionFalse,
					LastTransitionTime: oldTime,
				},
			},
		},
	}

	// Downloaded stays True, ReadyInRuntime changes to True
	internalConds := []status.Condition{
		{Name: "Downloaded", Status: metav1.ConditionTrue},
		{Name: "ReadyInRuntime", Status: metav1.ConditionTrue},
	}

	svc.applyInternalConditions(app, internalConds)

	require.Len(t, app.Status.InternalConditions, 2)

	downloaded := findInternalCondition(app, "Downloaded")
	require.NotNil(t, downloaded)
	// Status unchanged - LastTransitionTime preserved
	assert.Equal(t, oldTime, downloaded.LastTransitionTime)

	runtime := findInternalCondition(app, "ReadyInRuntime")
	require.NotNil(t, runtime)
	// Status changed - LastTransitionTime updated
	assert.NotEqual(t, oldTime, runtime.LastTransitionTime)
}

// =============================================================================
// Integration: Full Workflow
// =============================================================================

func TestService_FullWorkflow_InitialInstall(t *testing.T) {
	svc := newTestService()

	app := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Version: "1.0.0",
		},
	}

	// Simulate all conditions becoming True
	internalConds := []status.Condition{
		{Name: "Downloaded", Status: metav1.ConditionTrue},
		{Name: "ReadyOnFilesystem", Status: metav1.ConditionTrue},
		{Name: "RequirementsMet", Status: metav1.ConditionTrue},
		{Name: "ReadyInRuntime", Status: metav1.ConditionTrue},
		{Name: "HooksProcessed", Status: metav1.ConditionTrue},
		{Name: "HelmApplied", Status: metav1.ConditionTrue},
		{Name: "SettingsIsValid", Status: metav1.ConditionTrue},
	}

	svc.applyInternalConditions(app, internalConds)

	// Check external conditions were computed
	installed := findCondition(app, "Installed")
	require.NotNil(t, installed, "Installed condition should be set")
	assert.Equal(t, corev1.ConditionTrue, installed.Status)

	ready := findCondition(app, "Ready")
	require.NotNil(t, ready, "Ready condition should be set")
	assert.Equal(t, corev1.ConditionTrue, ready.Status)
}

func TestService_FullWorkflow_DownloadFailed(t *testing.T) {
	svc := newTestService()

	app := &v1alpha1.Application{}

	internalConds := []status.Condition{
		{Name: "Downloaded", Status: metav1.ConditionFalse, Reason: "GetImageReader", Message: "unauthorized"},
	}

	svc.applyInternalConditions(app, internalConds)

	installed := findCondition(app, "Installed")
	require.NotNil(t, installed)
	assert.Equal(t, corev1.ConditionFalse, installed.Status)
	assert.Equal(t, "DownloadWasFailed", installed.Reason)
	// Message should be copied from internal condition
	assert.Equal(t, "unauthorized", installed.Message)
}

// =============================================================================
// Helpers
// =============================================================================

func newTestService() *Service {
	return &Service{
		mapper: statusmapper.New(specs.DefaultSpecs()),
		logger: log.NewNop(),
	}
}

func findCondition(app *v1alpha1.Application, condType string) *v1alpha1.ApplicationStatusCondition {
	for i := range app.Status.Conditions {
		if app.Status.Conditions[i].Type == condType {
			return &app.Status.Conditions[i]
		}
	}
	return nil
}

func findInternalCondition(app *v1alpha1.Application, condType string) *v1alpha1.ApplicationInternalStatusCondition {
	for i := range app.Status.InternalConditions {
		if app.Status.InternalConditions[i].Type == condType {
			return &app.Status.InternalConditions[i]
		}
	}
	return nil
}
