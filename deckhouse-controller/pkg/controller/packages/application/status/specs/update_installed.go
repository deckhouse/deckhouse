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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
)

// UpdateInstalledSpec defines the UpdateInstalled condition rules.
// Only applies when version is changing after initial install.
func UpdateInstalledSpec() statusmapper.Spec {
	return statusmapper.Spec{
		Type: status.ConditionUpdateInstalled,
		// Only evaluate during version updates
		AppliesWhen: statusmapper.Predicate{
			Name: "version-changing-after-install",
			Fn: func(input *statusmapper.Input) bool {
				return !input.IsInitialInstall && input.VersionChanged
			},
		},
		Rule: statusmapper.FirstMatch{
			// Success: all installation conditions met
			{
				When: statusmapper.AllTrue(
					status.ConditionDownloaded,
					status.ConditionReadyOnFilesystem,
					status.ConditionRequirementsMet,
					status.ConditionReadyInRuntime,
					status.ConditionHooksProcessed,
					status.ConditionHelmApplied,
				),
				Status: metav1.ConditionTrue,
			},
			// Failure: download
			{
				When:        statusmapper.IsFalse(status.ConditionDownloaded),
				Status:      metav1.ConditionFalse,
				Reason:      "UpdateFailed",
				MessageFrom: status.ConditionDownloaded,
			},
			// Failure: requirements
			{
				When:        statusmapper.IsFalse(status.ConditionRequirementsMet),
				Status:      metav1.ConditionFalse,
				Reason:      "RequirementsNotMet",
				MessageFrom: status.ConditionRequirementsMet,
			},
			// In progress: downloading
			{
				When:   statusmapper.NotTrue(status.ConditionDownloaded),
				Status: metav1.ConditionFalse,
				Reason: "Downloading",
			},
			// Default
			{
				Status: metav1.ConditionFalse,
				Reason: "UpdateInProgress",
			},
		},
	}
}
