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
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/types"
)

// ConfigurationAppliedSpec defines the ConfigurationApplied condition rules.
// Composite: HelmApplied AND SettingsValid.
func ConfigurationAppliedSpec() types.MappingSpec {
	return types.MappingSpec{
		Type: types.ConditionConfigurationApplied,
		MappingRules: []types.MappingRule{
			// Both helm and settings OK
			{
				Name: "configuration-applied",
				Matcher: types.AllOf{
					types.InternalTrue(status.ConditionHelmApplied),
					types.InternalTrue(status.ConditionSettingsIsValid),
				},
				Status: corev1.ConditionTrue,
			},
			// Settings invalid (higher priority than helm failure)
			{
				Name:        "settings-invalid",
				Matcher:     types.InternalFalse(status.ConditionSettingsIsValid),
				Status:      corev1.ConditionFalse,
				Reason:      "ConfigurationValidationFailed",
				MessageFrom: status.ConditionSettingsIsValid,
			},
			// Helm failed
			{
				Name:        "helm-failed",
				Matcher:     types.InternalFalse(status.ConditionHelmApplied),
				Status:      corev1.ConditionFalse,
				MessageFrom: status.ConditionHelmApplied,
			},
			// Default
			{
				Name:    "default-not-applied",
				Matcher: types.Always{},
				Status:  corev1.ConditionFalse,
			},
		},
	}
}
