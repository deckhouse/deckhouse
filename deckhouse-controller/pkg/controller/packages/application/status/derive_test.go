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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func F(cs metav1.ConditionStatus, reason string) *ConditionStatus {
	return &ConditionStatus{Status: cs, Reason: reason}
}

func TestDeriveStatus_All22Scenarios(t *testing.T) {
	tests := []struct {
		name    string
		conds   *ModuleConditions
		phase   string
		message string
	}{
		// ── Install (Installed=False) ──────────────────────────────────

		{
			name: "1. Install: waiting for dependent modules to converge",
			conds: &ModuleConditions{
				Installed: F(metav1.ConditionFalse, "Pending"),
				Ready:     F(metav1.ConditionFalse, "Pending"),
			},
			phase:   PhasePending,
			message: "Installation is waiting for dependent modules to converge",
		},
		{
			name: "2. Install: requirements unmet",
			conds: &ModuleConditions{
				Installed: F(metav1.ConditionFalse, "RequirementsUnmet"),
				Ready:     F(metav1.ConditionFalse, "RequirementsUnmet"),
			},
			phase:   PhasePending,
			message: "Installation is blocked: module requirements are not satisfied",
		},
		{
			name: "3. Install: download/mount failed",
			conds: &ModuleConditions{
				Installed: F(metav1.ConditionFalse, "DownloadFailed"),
				Ready:     F(metav1.ConditionFalse, "DownloadFailed"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: module package could not be downloaded or mounted",
		},
		{
			name: "4. Install: load from filesystem failed",
			conds: &ModuleConditions{
				Installed: F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
				Ready:     F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: module package on disk could not be loaded",
		},
		{
			name: "5. Install: invalid settings",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionFalse, "SettingsInvalid"),
				Ready:                F(metav1.ConditionFalse, "SettingsInvalid"),
				ConfigurationApplied: F(metav1.ConditionFalse, "SettingsInvalid"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: module settings did not pass validation",
		},
		{
			name: "6. Install: hook sync phase failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionFalse, "HookInitializationFailed"),
				Ready:                F(metav1.ConditionFalse, "HookInitializationFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "HookInitializationFailed"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: hook synchronization phase failed",
		},
		{
			name: "7. Install: startup/runtime hooks failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionFalse, "HookFailed"),
				Ready:                F(metav1.ConditionFalse, "HookFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "HookFailed"),
				Managed:              F(metav1.ConditionFalse, "HookFailed"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: startup or runtime hooks failed",
		},
		{
			name: "8. Install: Helm apply failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Ready:                F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Managed:              F(metav1.ConditionFalse, "ManifestsApplyFailed"),
			},
			phase:   PhaseFailed,
			message: "Installation failed: Helm could not apply manifests",
		},

		// ── Update (Installed=True sticky, UpdateInstalled=False) ─────

		{
			name: "9. Update: waiting for dependent modules to converge",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "Pending"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseUpdating,
			message: "Update is waiting for dependent modules to converge; previous version is still serving",
		},
		{
			name: "10. Update: download/mount failed, old version serving",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "DownloadFailed"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseUpdating,
			message: "Update is stalled: new version could not be downloaded; previous version is still serving",
		},
		{
			name: "11. Update: load from filesystem failed, old version serving",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseUpdating,
			message: "Update is stalled: new version could not be loaded from filesystem; previous version is still serving",
		},
		{
			name: "12. Update: invalid settings, old version serving",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "SettingsInvalid"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseUpdating,
			message: "Update is stalled: new settings did not pass validation; previous version is still serving",
		},
		{
			name: "13. Update: hook sync failed, old version down",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "HookInitializationFailed"),
				Ready:                F(metav1.ConditionFalse, "HookInitializationFailed"),
				Scaled:               F(metav1.ConditionUnknown, "HookInitializationFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "HookInitializationFailed"),
				Managed:              F(metav1.ConditionFalse, "HookInitializationFailed"),
			},
			phase:   PhaseFailed,
			message: "Update failed during hook synchronization; previous version is no longer serving",
		},
		{
			name: "14. Update: startup/runtime hooks failed, old version down",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "HookFailed"),
				Ready:                F(metav1.ConditionFalse, "HookFailed"),
				Scaled:               F(metav1.ConditionUnknown, "HookFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "HookFailed"),
				Managed:              F(metav1.ConditionFalse, "HookFailed"),
			},
			phase:   PhaseFailed,
			message: "Update failed: startup or runtime hooks of the new version failed; previous version is no longer serving",
		},
		{
			name: "15. Update: Helm apply failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				UpdateInstalled:      F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Ready:                F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Scaled:               F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Managed:              F(metav1.ConditionFalse, "ManifestsApplyFailed"),
			},
			phase:   PhaseFailed,
			message: "Update failed: Helm could not apply manifests for the new version; previous version is no longer serving",
		},

		// ── Reconcile (Installed=True, no active update) ──────────────

		{
			name: "16. Reconcile: all working",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseReady,
			message: "Module is installed and operating normally",
		},
		{
			name: "17. Reconcile: artifact tampered (download verification failed)",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionFalse, "DownloadFailed"),
				Scaled:               F(metav1.ConditionUnknown, "DownloadFailed"),
				ConfigurationApplied: F(metav1.ConditionUnknown, "DownloadFailed"),
				Managed:              F(metav1.ConditionFalse, "DownloadFailed"),
			},
			phase:   PhaseDegraded,
			message: "Reconcile failed: on-disk artifact failed verification; runtime state can no longer be trusted",
		},
		{
			name: "18. Reconcile: load from filesystem failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
				Managed:              F(metav1.ConditionFalse, "LoadFromFilesystemFailed"),
			},
			phase:   PhaseDegraded,
			message: "Reconcile failed: module could not be loaded from filesystem; runtime state can no longer be trusted",
		},
		{
			name: "19. Reconcile: invalid settings",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionTrue, "Ready"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionFalse, "SettingsInvalid"),
				Managed:              F(metav1.ConditionTrue, "Managed"),
			},
			phase:   PhaseDegraded,
			message: "Reconcile failed: new settings did not pass validation; previously applied configuration is still in effect",
		},
		{
			name: "20. Reconcile: startup/runtime hooks failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionFalse, "HookFailed"),
				Scaled:               F(metav1.ConditionTrue, "Scaled"),
				ConfigurationApplied: F(metav1.ConditionFalse, "HookFailed"),
				Managed:              F(metav1.ConditionFalse, "HookFailed"),
			},
			phase:   PhaseDegraded,
			message: "Reconcile failed: startup or runtime hooks failed; workload remains scaled but is no longer managed",
		},
		{
			name: "21. Reconcile: Helm apply failed",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionTrue, "Installed"),
				Ready:                F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				ConfigurationApplied: F(metav1.ConditionFalse, "ManifestsApplyFailed"),
				Managed:              F(metav1.ConditionFalse, "ManifestsApplyFailed"),
			},
			phase:   PhaseDegraded,
			message: "Reconcile failed: Helm could not apply manifests",
		},

		// ── Suspended (dependency disabled) ───────────────────────────

		{
			name: "22. Dependency disabled",
			conds: &ModuleConditions{
				Installed:            F(metav1.ConditionFalse, "RequirementsUnmet"),
				Ready:                F(metav1.ConditionFalse, "RequirementsUnmet"),
				Scaled:               F(metav1.ConditionUnknown, "RequirementsUnmet"),
				ConfigurationApplied: F(metav1.ConditionUnknown, "RequirementsUnmet"),
				Managed:              F(metav1.ConditionUnknown, "RequirementsUnmet"),
			},
			phase:   PhaseSuspended,
			message: "Module is suspended: a required dependency has been disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, message := DeriveStatus(tt.conds)
			if phase != tt.phase {
				t.Errorf("phase = %q, want %q", phase, tt.phase)
			}
			if message != tt.message {
				t.Errorf("message = %q, want %q", message, tt.message)
			}
		})
	}
}

func TestDeriveStatus_EdgeCases(t *testing.T) {
	t.Run("nil conditions", func(t *testing.T) {
		phase, message := DeriveStatus(nil)
		if phase != PhasePending {
			t.Errorf("phase = %q, want %q", phase, PhasePending)
		}
		if message != "Installation is waiting for dependent modules to converge" {
			t.Errorf("message = %q", message)
		}
	})

	t.Run("all nil fields", func(t *testing.T) {
		phase, _ := DeriveStatus(&ModuleConditions{})
		// Installed is not set → no explicit phase rule matches → falls to Ready
		// because installed is not False
		if phase != PhaseReady {
			t.Errorf("phase = %q, want %q", phase, PhaseReady)
		}
	})

	t.Run("unknown reason on Installed=False", func(t *testing.T) {
		phase, message := DeriveStatus(&ModuleConditions{
			Installed: F(metav1.ConditionFalse, "SomeNewReason"),
			Ready:     F(metav1.ConditionFalse, "SomeNewReason"),
		})
		if phase != PhaseFailed {
			t.Errorf("phase = %q, want %q", phase, PhaseFailed)
		}
		if message != "Installation failed: SomeNewReason" {
			t.Errorf("message = %q", message)
		}
	})

	t.Run("unknown reason on update failed", func(t *testing.T) {
		phase, message := DeriveStatus(&ModuleConditions{
			Installed:       F(metav1.ConditionTrue, "Installed"),
			UpdateInstalled: F(metav1.ConditionFalse, "NewFailureMode"),
			Ready:           F(metav1.ConditionFalse, "NewFailureMode"),
		})
		if phase != PhaseFailed {
			t.Errorf("phase = %q, want %q", phase, PhaseFailed)
		}
		if message != "Update failed: NewFailureMode; previous version is no longer serving" {
			t.Errorf("message = %q", message)
		}
	})

	t.Run("unknown reason on degraded", func(t *testing.T) {
		phase, message := DeriveStatus(&ModuleConditions{
			Installed:            F(metav1.ConditionTrue, "Installed"),
			Ready:                F(metav1.ConditionFalse, "WeirdError"),
			ConfigurationApplied: F(metav1.ConditionTrue, "ConfigurationApplied"),
			Scaled:               F(metav1.ConditionTrue, "Scaled"),
			Managed:              F(metav1.ConditionTrue, "Managed"),
		})
		if phase != PhaseDegraded {
			t.Errorf("phase = %q, want %q", phase, PhaseDegraded)
		}
		if message != "Reconcile failed: WeirdError" {
			t.Errorf("message = %q", message)
		}
	})
}

func TestDeriveStatus_SuspendedVsPending(t *testing.T) {
	// Both have Installed=False/RequirementsUnmet.
	// Suspended: Scaled/ConfigApplied/Managed are Unknown (was running, dependency off).
	// Pending:   Scaled/ConfigApplied/Managed are absent (never installed, blocked).

	t.Run("suspended when runtime conditions are Unknown", func(t *testing.T) {
		phase, _ := DeriveStatus(&ModuleConditions{
			Installed:            F(metav1.ConditionFalse, "RequirementsUnmet"),
			Ready:                F(metav1.ConditionFalse, "RequirementsUnmet"),
			Scaled:               F(metav1.ConditionUnknown, "RequirementsUnmet"),
			ConfigurationApplied: F(metav1.ConditionUnknown, "RequirementsUnmet"),
			Managed:              F(metav1.ConditionUnknown, "RequirementsUnmet"),
		})
		if phase != PhaseSuspended {
			t.Errorf("phase = %q, want %q (Suspended)", phase, PhaseSuspended)
		}
	})

	t.Run("pending when runtime conditions are absent (nil)", func(t *testing.T) {
		phase, _ := DeriveStatus(&ModuleConditions{
			Installed: F(metav1.ConditionFalse, "RequirementsUnmet"),
			Ready:     F(metav1.ConditionFalse, "RequirementsUnmet"),
			// Scaled, ConfigurationApplied, Managed are nil (absent)
		})
		if phase != PhasePending {
			t.Errorf("phase = %q, want %q (Pending)", phase, PhasePending)
		}
	})

	t.Run("pending when only some runtime conditions are Unknown", func(t *testing.T) {
		phase, _ := DeriveStatus(&ModuleConditions{
			Installed:            F(metav1.ConditionFalse, "RequirementsUnmet"),
			Ready:                F(metav1.ConditionFalse, "RequirementsUnmet"),
			Scaled:               F(metav1.ConditionUnknown, "RequirementsUnmet"),
			ConfigurationApplied: F(metav1.ConditionUnknown, "RequirementsUnmet"),
			// Managed is absent — not all three are Unknown → falls through to Pending
		})
		if phase != PhasePending {
			t.Errorf("phase = %q, want %q (Pending)", phase, PhasePending)
		}
	})
}
