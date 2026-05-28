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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase constants for Application module status.
const (
	PhasePending   = "Pending"
	PhaseFailed    = "Failed"
	PhaseUpdating  = "Updating"
	PhaseReady     = "Ready"
	PhaseDegraded  = "Degraded"
	PhaseSuspended = "Suspended"
)

// ConditionStatus represents a single condition's observed state.
// Nil pointer means the condition has not been set (absent).
type ConditionStatus struct {
	Status metav1.ConditionStatus
	Reason string
}

// ModuleConditions holds the six external user-facing Application conditions.
type ModuleConditions struct {
	Installed            *ConditionStatus
	UpdateInstalled      *ConditionStatus
	Ready                *ConditionStatus
	Scaled               *ConditionStatus
	ConfigurationApplied *ConditionStatus
	Managed              *ConditionStatus
}

// reasonToInstallBody maps a reason to the message fragment for install-scope failures.
var reasonToInstallBody = map[string]string{
	"Pending":                  "is waiting for dependent modules to converge",
	"RequirementsUnmet":        "is blocked: module requirements are not satisfied",
	"DownloadFailed":           "failed: module package could not be downloaded or mounted",
	"LoadFromFilesystemFailed": "failed: module package on disk could not be loaded",
	"SettingsInvalid":          "failed: module settings did not pass validation",
	"HookInitializationFailed": "failed: hook synchronization phase failed",
	"HookFailed":               "failed: startup or runtime hooks failed",
	"ManifestsApplyFailed":     "failed: Helm could not apply manifests",
}

// reasonToUpdateBody maps a reason to the message fragment for update-scope failures.
var reasonToUpdateBody = map[string]string{
	"Pending":                  "is waiting for dependent modules to converge",
	"DownloadFailed":           "is stalled: new version could not be downloaded",
	"LoadFromFilesystemFailed": "is stalled: new version could not be loaded from filesystem",
	"SettingsInvalid":          "is stalled: new settings did not pass validation",
	"HookInitializationFailed": "failed during hook synchronization",
	"HookFailed":               "failed: startup or runtime hooks of the new version failed",
	"ManifestsApplyFailed":     "failed: Helm could not apply manifests for the new version",
}

// reasonToReconcileBody maps a reason to the message fragment for reconcile-scope failures.
var reasonToReconcileBody = map[string]string{
	"DownloadFailed":           "failed: on-disk artifact failed verification; runtime state can no longer be trusted",
	"LoadFromFilesystemFailed": "failed: module could not be loaded from filesystem; runtime state can no longer be trusted",
	"SettingsInvalid":          "failed: new settings did not pass validation; previously applied configuration is still in effect",
	"HookFailed":               "failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
	"ManifestsApplyFailed":     "failed: Helm could not apply manifests",
}

// DeriveStatus returns the canonical Phase and Message for the given set of
// module conditions. It is a pure function — no I/O, no side effects.
func DeriveStatus(c *ModuleConditions) (string, string) {
	if c == nil {
		return PhasePending, "Installation is waiting for dependent modules to converge"
	}

	// 1. Suspended — dependency disabled under a running app.
	if isFalseReason(c.Installed, "RequirementsUnmet") &&
		isUnknown(c.Scaled) && isUnknown(c.ConfigurationApplied) && isUnknown(c.Managed) {
		return PhaseSuspended, "Module is suspended: a required dependency has been disabled"
	}

	// 2. Installed=False — install phase.
	if isFalse(c.Installed) {
		reason := ""
		if c.Installed != nil {
			reason = c.Installed.Reason
		}

		if reason == "Pending" || reason == "RequirementsUnmet" {
			return PhasePending, installPendingMessage(reason)
		}
		return PhaseFailed, installFailedMessage(reason)
	}

	// Installed=True from here on.

	// 3. UpdateInstalled=False — update in progress or failed.
	if isFalse(c.UpdateInstalled) {
		reason := c.UpdateInstalled.Reason

		// Update failed: previous version is no longer serving.
		if isFalse(c.Ready) {
			return PhaseFailed, updateFailedMessage(reason)
		}

		// Update stalled: previous version is still serving.
		return PhaseUpdating, updateStalledMessage(reason)
	}

	// 4. No active update — reconcile or ready.
	if isFalse(c.Ready) || isFalse(c.ConfigurationApplied) {
		return PhaseDegraded, degradedMessage(c)
	}

	return PhaseReady, "Module is installed and operating normally"
}

// --- helpers ---

func isFalse(c *ConditionStatus) bool   { return c != nil && c.Status == metav1.ConditionFalse }
func isUnknown(c *ConditionStatus) bool { return c != nil && c.Status == metav1.ConditionUnknown }

func isFalseReason(c *ConditionStatus, reason string) bool {
	return c != nil && c.Status == metav1.ConditionFalse && c.Reason == reason
}

// --- message builders ---

func installPendingMessage(reason string) string {
	switch reason {
	case "Pending":
		return "Installation is waiting for dependent modules to converge"
	case "RequirementsUnmet":
		return "Installation is blocked: module requirements are not satisfied"
	}
	return "Installation " + reason
}

func installFailedMessage(reason string) string {
	if body, ok := reasonToInstallBody[reason]; ok {
		return "Installation " + body
	}
	return "Installation failed: " + reason
}

func updateStalledMessage(reason string) string {
	if body, ok := reasonToUpdateBody[reason]; ok {
		return "Update " + body + "; previous version is still serving"
	}
	return "Update is stalled: " + reason + "; previous version is still serving"
}

func updateFailedMessage(reason string) string {
	if body, ok := reasonToUpdateBody[reason]; ok {
		return "Update " + body + "; previous version is no longer serving"
	}
	return "Update failed: " + reason + "; previous version is no longer serving"
}

func degradedMessage(c *ModuleConditions) string {
	// Find the leading failing condition for the message body.
	var reason string
	if isFalse(c.Ready) {
		reason = c.Ready.Reason
	} else if isFalse(c.ConfigurationApplied) {
		reason = c.ConfigurationApplied.Reason
	}

	if body, ok := reasonToReconcileBody[reason]; ok {
		return "Reconcile " + body
	}
	return "Reconcile failed: " + reason
}

// ConditionsFromMeta builds a ModuleConditions struct from a slice of
// metav1.Condition, picking out the six known external condition types.
// Conditions not present in the slice are left as nil (absent).
func ConditionsFromMeta(conditions []metav1.Condition) *ModuleConditions {
	c := &ModuleConditions{}
	for i := range conditions {
		cond := &conditions[i]
		cs := &ConditionStatus{Status: cond.Status, Reason: cond.Reason}
		switch cond.Type {
		case ConditionInstalled:
			c.Installed = cs
		case ConditionUpdateInstalled:
			c.UpdateInstalled = cs
		case ConditionReady:
			c.Ready = cs
		case ConditionScaled:
			c.Scaled = cs
		case ConditionConfigurationApplied:
			c.ConfigurationApplied = cs
		case ConditionManaged:
			c.Managed = cs
		}
	}
	return c
}
