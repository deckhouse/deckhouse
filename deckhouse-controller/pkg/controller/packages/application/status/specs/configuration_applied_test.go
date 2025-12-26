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

func TestConfigurationAppliedSpec_Composite(t *testing.T) {
	spec := ConfigurationAppliedSpec()

	tests := []struct {
		name           string
		helmApplied    corev1.ConditionStatus
		settingsValid  corev1.ConditionStatus
		expectedStatus corev1.ConditionStatus
		expectedReason string
	}{
		{"both true", corev1.ConditionTrue, corev1.ConditionTrue, corev1.ConditionTrue, ""},
		{"helm false", corev1.ConditionFalse, corev1.ConditionTrue, corev1.ConditionFalse, ""},
		{"settings false", corev1.ConditionTrue, corev1.ConditionFalse, corev1.ConditionFalse, "ConfigurationValidationFailed"},
		{"both false - settings priority", corev1.ConditionFalse, corev1.ConditionFalse, corev1.ConditionFalse, "ConfigurationValidationFailed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types.MappingInput{
				InternalConditions: map[string]types.InternalCondition{
					"HelmApplied":     {Name: "HelmApplied", Status: tt.helmApplied},
					"SettingsIsValid": {Name: "SettingsIsValid", Status: tt.settingsValid},
				},
				CurrentConditions: make(map[types.ExternalConditionType]types.ExternalCondition),
				App:               &v1alpha1.Application{},
			}

			result := spec.Map(input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, tt.expectedReason, result.Reason)
		})
	}
}

