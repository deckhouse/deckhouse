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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmap"
	intstatus "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// =============================================================================
// External Condition Types (computed from internal conditions)
// =============================================================================

const (
	// ConditionInstalled indicates package was successfully installed
	ConditionInstalled string = "Installed"
	// ConditionUpdateInstalled indicates package update was installed
	ConditionUpdateInstalled string = "UpdateInstalled"
	// ConditionReady indicates package is ready and operational
	ConditionReady string = "Ready"
	// ConditionScaled indicates package is fully scaled
	ConditionScaled string = "Scaled"
	// ConditionManaged indicates package is being managed
	ConditionManaged string = "Managed"
	// ConditionConfigurationApplied indicates configuration was applied
	ConditionConfigurationApplied string = "ConfigurationApplied"
)

// BuildMapper returns a mapper with all standard rules.
func buildMapper() condmap.Mapper {
	return condmap.Mapper{
		Maps: []condmap.Map{
			mapInstalled,
			mapUpdateInstalled,
			mapReady,
			mapScaled,
			mapManaged,
			mapConfigurationApplied,
		},
	}
}

// mapInstalled is sticky: after the first successful install, it stops emitting
// updates so the existing Installed=True condition is preserved.
func mapInstalled(state condmap.State) metav1.Condition {
	if state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		return metav1.Condition{}
	}

	if state.IntEqual(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue) && !state.IsUpdating() {
		return state.ConditionByInt(ConditionInstalled, metav1.ConditionTrue, string(intstatus.ConditionReadyInCluster))
	}

	if cond, ok := firstInstallBlocker(state); ok {
		return state.ConditionByInt(ConditionInstalled, metav1.ConditionFalse, cond)
	}

	return metav1.Condition{}
}

// mapUpdateInstalled reports update progress only after the Application has
// been installed and either the desired version changed or an update condition
// already exists from an earlier update attempt.
func mapUpdateInstalled(state condmap.State) metav1.Condition {
	if !state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		return metav1.Condition{}
	}

	if !state.IsUpdating() && !state.HasExt(ConditionUpdateInstalled) {
		return metav1.Condition{}
	}

	if state.IntEqual(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue) {
		return state.ConditionByInt(ConditionUpdateInstalled, metav1.ConditionTrue, string(intstatus.ConditionReadyInCluster))
	}

	if state.IsUpdating() {
		if cond, ok := firstInstallBlocker(state); ok {
			return state.ConditionByInt(ConditionUpdateInstalled, metav1.ConditionFalse, cond)
		}
	}

	return metav1.Condition{}
}

// mapReady answers the user-facing question "why is the application not ready
// right now?". RequirementsMet=True is a passed gate, so it never produces
// Ready=False. RequirementsMet=False still blocks readiness.
func mapReady(state condmap.State) metav1.Condition {
	if state.IntEqual(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue) {
		return state.ConditionByInt(ConditionReady, metav1.ConditionTrue, string(intstatus.ConditionReadyInCluster))
	}

	for _, cond := range []string{
		string(intstatus.ConditionPending),
		string(intstatus.ConditionRequirementsMet),
		string(intstatus.ConditionReadyOnFilesystem),
		string(intstatus.ConditionReadyInRuntime),
		string(intstatus.ConditionSettingsValid),
		string(intstatus.ConditionHooksProcessed),
		string(intstatus.ConditionHelmApplied),
		string(intstatus.ConditionReadyInCluster),
	} {
		if cond == string(intstatus.ConditionPending) {
			if state.IntEqual(cond, metav1.ConditionTrue) {
				return state.ConditionByInt(ConditionReady, metav1.ConditionFalse, cond)
			}

			continue
		}

		if state.IntEqual(cond, metav1.ConditionFalse) {
			return state.ConditionByInt(ConditionReady, metav1.ConditionFalse, cond)
		}
	}

	return metav1.Condition{}
}

// mapScaled is intentionally bound only to ReadyInCluster. Other lifecycle
// gates can affect Ready, but they should not make Scaled report unrelated
// reasons such as RequirementsMet.
func mapScaled(state condmap.State) metav1.Condition {
	if state.IntEqual(string(intstatus.ConditionReadyInCluster), metav1.ConditionTrue) {
		return state.ConditionByInt(ConditionScaled, metav1.ConditionTrue, string(intstatus.ConditionReadyInCluster))
	}

	if state.IntEqual(string(intstatus.ConditionReadyInCluster), metav1.ConditionFalse) {
		return state.ConditionByInt(ConditionScaled, metav1.ConditionFalse, string(intstatus.ConditionReadyInCluster))
	}

	return metav1.Condition{}
}

// mapManaged describes whether runtime management is active for an already
// installed Application. Before first install there is no useful Managed update.
func mapManaged(state condmap.State) metav1.Condition {
	if state.ExtEqual(ConditionInstalled, metav1.ConditionTrue) {
		if state.IntEqual(string(intstatus.ConditionPending), metav1.ConditionTrue) {
			return state.ConditionByInt(ConditionManaged, metav1.ConditionFalse, string(intstatus.ConditionPending))
		}

		for _, cond := range []string{
			string(intstatus.ConditionReadyInRuntime),
			string(intstatus.ConditionReadyInCluster),
			string(intstatus.ConditionHooksProcessed),
		} {
			if state.IntEqual(cond, metav1.ConditionFalse) {
				return state.ConditionByInt(ConditionManaged, metav1.ConditionFalse, cond)
			}
		}
	}

	if state.AllIntEqual(metav1.ConditionTrue,
		string(intstatus.ConditionReadyInRuntime),
		string(intstatus.ConditionReadyInCluster),
		string(intstatus.ConditionHooksProcessed),
	) {
		return state.ConditionByInt(ConditionManaged, metav1.ConditionTrue, string(intstatus.ConditionReadyInRuntime))
	}

	return metav1.Condition{}
}

// mapConfigurationApplied tracks settings validation, hook processing, and
// manifest apply. The internal reason/message explain the concrete blocker.
func mapConfigurationApplied(state condmap.State) metav1.Condition {
	if state.AllIntEqual(metav1.ConditionTrue,
		string(intstatus.ConditionSettingsValid),
		string(intstatus.ConditionHooksProcessed),
		string(intstatus.ConditionHelmApplied),
	) {
		return state.ConditionByInt(ConditionConfigurationApplied, metav1.ConditionTrue, string(intstatus.ConditionSettingsValid))
	}

	for _, cond := range []string{
		string(intstatus.ConditionSettingsValid),
		string(intstatus.ConditionHooksProcessed),
		string(intstatus.ConditionHelmApplied),
	} {
		if state.IntEqual(cond, metav1.ConditionFalse) {
			return state.ConditionByInt(ConditionConfigurationApplied, metav1.ConditionFalse, cond)
		}
	}

	return metav1.Condition{}
}

// firstInstallBlocker keeps Installed and UpdateInstalled false reasons stable
// without coupling them to Ready's broader user-facing priority order.
func firstInstallBlocker(state condmap.State) (string, bool) {
	for _, cond := range []string{
		string(intstatus.ConditionRequirementsMet),
		string(intstatus.ConditionReadyOnFilesystem),
		string(intstatus.ConditionReadyInRuntime),
		string(intstatus.ConditionReadyInCluster),
	} {
		if state.IntEqual(cond, metav1.ConditionFalse) {
			return cond, true
		}
	}

	return "", false
}
