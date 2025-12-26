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

// UpdateInstalledSpec defines the UpdateInstalled condition rules.
// Only applies when version is changing after initial install.
func UpdateInstalledSpec() types.MappingSpec {
	return types.MappingSpec{
		Type: types.ConditionUpdateInstalled,
		// Only evaluate during version updates
		AppliesWhen: types.Predicate{
			Name: "version-changing-after-install",
			Fn: func(input *types.MappingInput) bool {
				return !input.IsInitialInstall && input.VersionChanged
			},
		},
		MappingRules: []types.MappingRule{
			// Success
			{
				Name:    "update-complete",
				Matcher: types.AllInstallationConditionsMet(),
				Status:  corev1.ConditionTrue,
			},
			// Failure: download
			{
				Name:        "update-failed-download",
				Matcher:     types.InternalFalse(status.ConditionDownloaded),
				Status:      corev1.ConditionFalse,
				Reason:      "UpdateFailed",
				MessageFrom: status.ConditionDownloaded,
			},
			// Failure: requirements
			{
				Name:        "update-failed-requirements",
				Matcher:     types.InternalFalse(status.ConditionRequirementsMet),
				Status:      corev1.ConditionFalse,
				Reason:      "RequirementsNotMet",
				MessageFrom: status.ConditionRequirementsMet,
			},
			// In progress: downloading
			{
				Name:    "downloading",
				Matcher: types.NotTrue(status.ConditionDownloaded),
				Status:  corev1.ConditionFalse,
				Reason:  "Downloading",
			},
			// Default
			{
				Name:    "default-in-progress",
				Matcher: types.Always{},
				Status:  corev1.ConditionFalse,
				Reason:  "UpdateInProgress",
			},
		},
	}
}
