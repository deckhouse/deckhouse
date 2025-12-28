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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

func TestInstalledSpec_Sticky(t *testing.T) {
	spec := InstalledSpec()

	// Input where Installed was already True but internal conditions are now failing
	input := &statusmapper.Input{
		InternalConditions: map[status.ConditionName]status.Condition{
			"ReadyInRuntime": {Name: "ReadyInRuntime", Status: metav1.ConditionFalse},
		},
		ExternalConditions: map[status.ConditionName]status.Condition{
			status.ConditionInstalled: {Name: status.ConditionInstalled, Status: metav1.ConditionTrue},
		},
		IsInitialInstall: false,
	}

	result := spec.Map(input)

	// Should stay True despite ReadyInRuntime being False (sticky behavior)
	require.NotNil(t, result)
	assert.Equal(t, metav1.ConditionTrue, result.Status)
}

func TestInstalledSpec_Initial(t *testing.T) {
	spec := InstalledSpec()

	tests := []struct {
		name               string
		internalConditions map[status.ConditionName]status.Condition
		expectedStatus     metav1.ConditionStatus
		expectedReason     string
	}{
		{
			name: "all conditions met",
			internalConditions: map[status.ConditionName]status.Condition{
				"Downloaded":        {Name: "Downloaded", Status: metav1.ConditionTrue},
				"ReadyOnFilesystem": {Name: "ReadyOnFilesystem", Status: metav1.ConditionTrue},
				"RequirementsMet":   {Name: "RequirementsMet", Status: metav1.ConditionTrue},
				"ReadyInRuntime":    {Name: "ReadyInRuntime", Status: metav1.ConditionTrue},
				"HooksProcessed":    {Name: "HooksProcessed", Status: metav1.ConditionTrue},
				"HelmApplied":       {Name: "HelmApplied", Status: metav1.ConditionTrue},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: "",
		},
		{
			name: "download failed",
			internalConditions: map[status.ConditionName]status.Condition{
				"Downloaded": {Name: "Downloaded", Status: metav1.ConditionFalse, Reason: "GetImageReader", Message: "unauthorized"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: "DownloadWasFailed",
		},
		{
			name: "requirements not met",
			internalConditions: map[status.ConditionName]status.Condition{
				"Downloaded":        {Name: "Downloaded", Status: metav1.ConditionTrue},
				"ReadyOnFilesystem": {Name: "ReadyOnFilesystem", Status: metav1.ConditionTrue},
				"RequirementsMet":   {Name: "RequirementsMet", Status: metav1.ConditionFalse, Reason: "RequirementsDeckhouse", Message: "deckhouse version >=1.70 required"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: "RequirementsNotMet",
		},
		{
			name: "downloading in progress",
			internalConditions: map[status.ConditionName]status.Condition{
				"Downloaded": {Name: "Downloaded", Status: metav1.ConditionUnknown},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: "Downloading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &statusmapper.Input{
				InternalConditions: tt.internalConditions,
				ExternalConditions: make(map[status.ConditionName]status.Condition),
				IsInitialInstall:   true,
			}

			result := spec.Map(input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, status.ConditionReason(tt.expectedReason), result.Reason)
		})
	}
}

func TestInstalledSpec_MessageFromInternal(t *testing.T) {
	spec := InstalledSpec()

	input := &statusmapper.Input{
		InternalConditions: map[status.ConditionName]status.Condition{
			"Downloaded": {
				Name:    "Downloaded",
				Status:  metav1.ConditionFalse,
				Reason:  "GetImageReader",
				Message: "unauthorized: access denied",
			},
		},
		ExternalConditions: make(map[status.ConditionName]status.Condition),
		IsInitialInstall:   true,
	}

	result := spec.Map(input)
	require.NotNil(t, result)
	assert.Equal(t, metav1.ConditionFalse, result.Status)
	assert.Equal(t, status.ConditionReason("DownloadWasFailed"), result.Reason)
	assert.Equal(t, "unauthorized: access denied", result.Message)
}
