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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmapper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
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
	// ConditionPartiallyDegraded indicates package is partially degraded
	ConditionPartiallyDegraded string = "PartiallyDegraded"
	// ConditionManaged indicates package is being managed
	ConditionManaged string = "Managed"
	// ConditionConfigurationApplied indicates configuration was applied
	ConditionConfigurationApplied string = "ConfigurationApplied"
)

// core conditions checked for install/ready states
var coreConds = []string{
	string(status.ConditionDownloaded),
	string(status.ConditionReadyOnFilesystem),
	string(status.ConditionReadyInRuntime),
	string(status.ConditionReadyInCluster),
	string(status.ConditionRequirementsMet),
}

// managed conditions for operational state
var managedConds = []string{
	string(status.ConditionReadyInRuntime),
	string(status.ConditionReadyInCluster),
	string(status.ConditionHooksProcessed),
}

// config conditions for configuration state
var configConds = []string{
	string(status.ConditionSettingsValid),
	string(status.ConditionHooksProcessed),
	string(status.ConditionHelmApplied),
}

// BuildMapper returns a mapper with all standard rules.
func buildMapper() condmapper.Mapper {
	return condmapper.Mapper{
		Rules: []condmapper.Rule{
			installedRule(),
			updateInstalledRule(),
			readyRule(),
			partiallyDegradedRule(),
			managedRule(),
			configAppliedRule(),
		},
	}
}

// installedRule: True when first install completes, stays True forever.
func installedRule() condmapper.Rule {
	return condmapper.Rule{
		Type: ConditionInstalled,
		TrueIf: condmapper.And(
			condmapper.IsTrue(string(status.ConditionReadyInCluster)),
			condmapper.Not(condmapper.VersionChanged()),
		),
		FalseIf: condmapper.AnyFalse(coreConds...),
		Sticky:  true,
	}
}

// updateInstalledRule: True when system is healthy after initial install.
// False only during an active update (version changed) when core conditions fail.
// This ensures that after a rollback from a failed update, the error is cleared.
func updateInstalledRule() condmapper.Rule {
	return condmapper.Rule{
		Type:   ConditionUpdateInstalled,
		TrueIf: condmapper.IsTrue(string(status.ConditionReadyInCluster)),
		FalseIf: condmapper.And(
			condmapper.VersionChanged(),
			condmapper.AnyFalse(coreConds...),
		),
		OnlyIf: condmapper.ExtTrue(ConditionInstalled),
	}
}

// readyRule: True when package is operational.
func readyRule() condmapper.Rule {
	return condmapper.Rule{
		Type:   ConditionReady,
		TrueIf: condmapper.IsTrue(string(status.ConditionReadyInCluster)),
		FalseIf: condmapper.Or(
			condmapper.AnyFalse(coreConds...),
			condmapper.And(
				condmapper.Not(condmapper.ExtTrue(ConditionInstalled)),
				condmapper.Or(
					condmapper.IsTrue(string(status.ConditionWaitConverge)),
					condmapper.IsTrue(string(status.ConditionRequirementsMet)),
				),
			),
		),
	}
}

// partiallyDegradedRule: True when functionality degraded (inverted semantics).
func partiallyDegradedRule() condmapper.Rule {
	return condmapper.Rule{
		Type:    ConditionPartiallyDegraded,
		TrueIf:  condmapper.AnyFalse(managedConds...),
		FalseIf: condmapper.AllTrue(managedConds...),
		OnlyIf:  condmapper.ExtTrue(ConditionInstalled),
	}
}

// managedRule: True when package is under active management.
func managedRule() condmapper.Rule {
	return condmapper.Rule{
		Type:   ConditionManaged,
		TrueIf: condmapper.AllTrue(managedConds...),
		FalseIf: condmapper.And(
			condmapper.ExtTrue(ConditionInstalled),
			condmapper.Or(
				condmapper.AnyFalse(managedConds...),
				condmapper.IsTrue(string(status.ConditionWaitConverge)),
			),
		),
	}
}

// configAppliedRule: True when configuration is applied.
func configAppliedRule() condmapper.Rule {
	return condmapper.Rule{
		Type:    ConditionConfigurationApplied,
		TrueIf:  condmapper.AllTrue(configConds...),
		FalseIf: condmapper.AnyFalse(configConds...),
	}
}
