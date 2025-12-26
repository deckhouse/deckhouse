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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

func TestInstalledSpec_Sticky(t *testing.T) {
	spec := InstalledSpec()

	// Input where Installed was already True but internal conditions are now failing
	input := &types.MappingInput{
		InternalConditions: map[string]types.InternalCondition{
			"ReadyInRuntime": {Name: "ReadyInRuntime", Status: corev1.ConditionFalse},
		},
		CurrentConditions: map[types.ExternalConditionType]types.ExternalCondition{
			types.ConditionInstalled: {Type: types.ConditionInstalled, Status: corev1.ConditionTrue},
		},
		App:              &v1alpha1.Application{},
		IsInitialInstall: false,
	}

	result := spec.Map(input)

	// Should stay True despite ReadyInRuntime being False (sticky behavior)
	require.NotNil(t, result)
	assert.Equal(t, corev1.ConditionTrue, result.Status)
}

func TestInstalledSpec_Initial(t *testing.T) {
	spec := InstalledSpec()

	tests := []struct {
		name           string
		internal       map[string]types.InternalCondition
		expectedStatus corev1.ConditionStatus
		expectedReason string
	}{
		{
			name: "all conditions met",
			internal: map[string]types.InternalCondition{
				"Downloaded":        {Name: "Downloaded", Status: corev1.ConditionTrue},
				"ReadyOnFilesystem": {Name: "ReadyOnFilesystem", Status: corev1.ConditionTrue},
				"RequirementsMet":   {Name: "RequirementsMet", Status: corev1.ConditionTrue},
				"ReadyInRuntime":    {Name: "ReadyInRuntime", Status: corev1.ConditionTrue},
				"HooksProcessed":    {Name: "HooksProcessed", Status: corev1.ConditionTrue},
				"HelmApplied":       {Name: "HelmApplied", Status: corev1.ConditionTrue},
			},
			expectedStatus: corev1.ConditionTrue,
			expectedReason: "",
		},
		{
			name: "download failed",
			internal: map[string]types.InternalCondition{
				"Downloaded": {Name: "Downloaded", Status: corev1.ConditionFalse, Reason: "GetImageReader", Message: "unauthorized"},
			},
			expectedStatus: corev1.ConditionFalse,
			expectedReason: "DownloadWasFailed",
		},
		{
			name: "requirements not met",
			internal: map[string]types.InternalCondition{
				"Downloaded":        {Name: "Downloaded", Status: corev1.ConditionTrue},
				"ReadyOnFilesystem": {Name: "ReadyOnFilesystem", Status: corev1.ConditionTrue},
				"RequirementsMet":   {Name: "RequirementsMet", Status: corev1.ConditionFalse, Reason: "RequirementsDeckhouse", Message: "deckhouse version >=1.70 required"},
			},
			expectedStatus: corev1.ConditionFalse,
			expectedReason: "RequirementsNotMet",
		},
		{
			name: "downloading in progress",
			internal: map[string]types.InternalCondition{
				"Downloaded": {Name: "Downloaded", Status: corev1.ConditionUnknown},
			},
			expectedStatus: corev1.ConditionFalse,
			expectedReason: "Downloading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types.MappingInput{
				InternalConditions: tt.internal,
				CurrentConditions:  make(map[types.ExternalConditionType]types.ExternalCondition),
				App:                &v1alpha1.Application{},
				IsInitialInstall:   true,
			}

			result := spec.Map(input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, tt.expectedReason, result.Reason)
		})
	}
}

func TestInstalledSpec_MessageFromInternal(t *testing.T) {
	spec := InstalledSpec()

	input := &types.MappingInput{
		InternalConditions: map[string]types.InternalCondition{
			"Downloaded": {
				Name:    "Downloaded",
				Status:  corev1.ConditionFalse,
				Reason:  "GetImageReader",
				Message: "unauthorized: access denied",
			},
		},
		CurrentConditions: make(map[types.ExternalConditionType]types.ExternalCondition),
		App:               &v1alpha1.Application{},
		IsInitialInstall:  true,
	}

	result := spec.Map(input)
	require.NotNil(t, result)
	assert.Equal(t, corev1.ConditionFalse, result.Status)
	assert.Equal(t, "DownloadWasFailed", result.Reason)
	assert.Equal(t, "unauthorized: access denied", result.Message)
}

