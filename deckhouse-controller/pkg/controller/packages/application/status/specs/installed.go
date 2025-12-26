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

// InstalledSpec defines the Installed condition rules.
// Sticky: once True, never reverts to False.
func InstalledSpec() types.MappingSpec {
	return types.MappingSpec{
		Type:   types.ConditionInstalled,
		Sticky: true,
		MappingRules: []types.MappingRule{
			// Success: all conditions met
			{
				Name:    "all-conditions-met",
				Matcher: types.AllInstallationConditionsMet(),
				Status:  corev1.ConditionTrue,
			},
			// Failure: download failed
			{
				Name:        "download-failed",
				Matcher:     types.InternalFalse(status.ConditionDownloaded),
				Status:      corev1.ConditionFalse,
				Reason:      "DownloadWasFailed",
				MessageFrom: status.ConditionDownloaded,
			},
			// Failure: requirements not met
			{
				Name:        "requirements-not-met",
				Matcher:     types.InternalFalse(status.ConditionRequirementsMet),
				Status:      corev1.ConditionFalse,
				Reason:      "RequirementsNotMet",
				MessageFrom: status.ConditionRequirementsMet,
			},
			// Failure: helm/manifests failed
			{
				Name:        "manifests-failed",
				Matcher:     types.InternalFalse(status.ConditionHelmApplied),
				Status:      corev1.ConditionFalse,
				Reason:      "ManifestsDeploymentFailed",
				MessageFrom: status.ConditionHelmApplied,
			},
			// In progress: downloading
			{
				Name:    "downloading",
				Matcher: types.NotTrue(status.ConditionDownloaded),
				Status:  corev1.ConditionFalse,
				Reason:  "Downloading",
			},
			// Default fallback
			{
				Name:    "default-in-progress",
				Matcher: types.Always{},
				Status:  corev1.ConditionFalse,
				Reason:  "InstallationInProgress",
			},
		},
	}
}
