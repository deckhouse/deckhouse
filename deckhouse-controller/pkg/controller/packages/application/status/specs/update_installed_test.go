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

func TestUpdateInstalledSpec_OnlyDuringUpdate(t *testing.T) {
	spec := UpdateInstalledSpec()

	allTrue := map[string]types.InternalCondition{
		"Downloaded":        {Name: "Downloaded", Status: corev1.ConditionTrue},
		"ReadyOnFilesystem": {Name: "ReadyOnFilesystem", Status: corev1.ConditionTrue},
		"RequirementsMet":   {Name: "RequirementsMet", Status: corev1.ConditionTrue},
		"ReadyInRuntime":    {Name: "ReadyInRuntime", Status: corev1.ConditionTrue},
		"HooksProcessed":    {Name: "HooksProcessed", Status: corev1.ConditionTrue},
		"HelmApplied":       {Name: "HelmApplied", Status: corev1.ConditionTrue},
	}

	tests := []struct {
		name           string
		isInitial      bool
		versionChanged bool
		expectNil      bool
		expectedStatus corev1.ConditionStatus
	}{
		{
			name:           "initial install - should not apply",
			isInitial:      true,
			versionChanged: false,
			expectNil:      true,
		},
		{
			name:           "no version change - should not apply",
			isInitial:      false,
			versionChanged: false,
			expectNil:      true,
		},
		{
			name:           "version change after install - should apply",
			isInitial:      false,
			versionChanged: true,
			expectNil:      false,
			expectedStatus: corev1.ConditionTrue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types.MappingInput{
				InternalConditions: allTrue,
				CurrentConditions: map[types.ExternalConditionType]types.ExternalCondition{
					types.ConditionInstalled: {Type: types.ConditionInstalled, Status: corev1.ConditionTrue},
				},
				App:              &v1alpha1.Application{},
				IsInitialInstall: tt.isInitial,
				VersionChanged:   tt.versionChanged,
			}

			result := spec.Map(input)

			if tt.expectNil {
				assert.Nil(t, result, "UpdateInstalled should not apply")
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedStatus, result.Status)
			}
		})
	}
}

