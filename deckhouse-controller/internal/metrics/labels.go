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

// Package metrics provides centralized metric label key constants for deckhouse-controller.
// All metric label keys are defined here to ensure consistency and prevent typos.
package metrics

const (
	// ============================================================================
	// Common Label Keys
	// ============================================================================

	// LabelModule is the label key for the module name.
	LabelModule = "module"

	// LabelSource is the label key for the module source name.
	LabelSource = "source"

	// LabelVersion is the label key for the release or module version.
	LabelVersion = "version"

	// LabelError is the label key for an error message.
	LabelError = "error"

	// LabelName is the label key for a generic resource name.
	LabelName = "name"

	// ============================================================================
	// Module Controller Label Keys
	// ============================================================================

	// LabelModuleRelease is the label key for the module release name.
	LabelModuleRelease = "module_release"

	// LabelRegistry is the label key for the registry URL.
	LabelRegistry = "registry"

	// LabelActualVersion is the label key for the currently deployed version.
	LabelActualVersion = "actual_version"

	// ============================================================================
	// Release Controller Label Keys
	// ============================================================================

	// LabelDeployingRelease is the label key for the name of the release being deployed.
	LabelDeployingRelease = "deployingRelease"

	// ============================================================================
	// Release Info Metric Label Keys
	// Used in the d8_release_info and d8_module_release_info metrics.
	// ============================================================================

	// LabelManualApprovalRequired indicates whether manual approval is required.
	LabelManualApprovalRequired = "manualApproval"

	// LabelDisruptionApprovalRequired indicates whether disruption approval is required.
	LabelDisruptionApprovalRequired = "disruptionApproval"

	// LabelRequirementsNotMet indicates whether release requirements are not met.
	LabelRequirementsNotMet = "requirementsNotMet"

	// LabelReleaseQueueDepth is the label key for the release queue depth.
	LabelReleaseQueueDepth = "releaseQueueDepth"

	// LabelMajorReleaseDepth is the label key for the major release queue depth.
	LabelMajorReleaseDepth = "majorReleaseDepth"

	// LabelMajorReleaseName is the label key for the major release name.
	LabelMajorReleaseName = "majorReleaseName"

	// LabelFromToName is the label key for the from-to release name in step-by-step updates.
	LabelFromToName = "fromToName"

	// LabelNotificationNotSent indicates whether an update notification has not been sent.
	LabelNotificationNotSent = "notificationNotSent"
)
