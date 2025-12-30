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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/conditionmapper"
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

func buildMapper() conditionmapper.Mapper {
	return conditionmapper.Mapper{
		Rules: []conditionmapper.Rule{
			installedRule(),
			updateInstalledRule(),
			readyRule(),
			partiallyDegradedRule(),
			managedRule(),
			configurationAppliedRule(),
		},
	}
}

// installedRule creates the Installed condition that tracks initial package installation.
// Only triggers once during initial install (when Installed is not yet True).
// Becomes True when package is downloaded, ready on filesystem, runtime, and cluster.
func installedRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionInstalled,
		Filter:     conditionmapper.ExternalNotTrue(ConditionInstalled),
		DependOn: []string{
			string(status.ConditionDownloaded),
			string(status.ConditionReadyOnFilesystem),
			string(status.ConditionReadyInRuntime),
			string(status.ConditionReadyInCluster),
		},
	}
}

// updateInstalledRule creates the UpdateInstalled condition that tracks package updates.
// Only triggers when version changed AND package was previously installed.
// Becomes True when new version is fully deployed (downloaded, filesystem, runtime, cluster ready).
func updateInstalledRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionUpdateInstalled,
		Filter: conditionmapper.And(
			conditionmapper.VersionChanged(true),
			conditionmapper.ExternalTrue(ConditionInstalled),
		),
		DependOn: []string{
			string(status.ConditionDownloaded),
			string(status.ConditionReadyOnFilesystem),
			string(status.ConditionReadyInRuntime),
			string(status.ConditionReadyInCluster),
		},
	}
}

// readyRule creates the Ready condition that indicates package operational readiness.
// Always evaluated (no predicate).
// True when package is downloaded, ready on filesystem, runtime, and cluster.
func readyRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionReady,
		DependOn: []string{
			string(status.ConditionDownloaded),
			string(status.ConditionReadyOnFilesystem),
			string(status.ConditionReadyInRuntime),
			string(status.ConditionReadyInCluster),
		},
	}
}

// partiallyDegradedRule creates the PartiallyDegraded condition with inverted semantics.
// Uses Invert=true so True means "not degraded" and False means "is degraded".
// True when runtime, cluster, and hooks are all healthy (not degraded).
// False when any of runtime, cluster, or hooks are unhealthy (degraded).
func partiallyDegradedRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionPartiallyDegraded,
		Invert:     true,
		DependOn: []string{
			string(status.ConditionReadyInRuntime),
			string(status.ConditionReadyInCluster),
			string(status.ConditionHooksProcessed),
		},
	}
}

// managedRule creates the Managed condition that indicates package lifecycle management.
// True when cluster resources are ready and hooks have been processed successfully.
func managedRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionManaged,
		DependOn: []string{
			string(status.ConditionReadyInCluster),
			string(status.ConditionHooksProcessed),
		},
	}
}

// configurationAppliedRule creates the ConfigurationApplied condition.
// True when settings are valid and hooks have been processed successfully.
func configurationAppliedRule() conditionmapper.Rule {
	return conditionmapper.Rule{
		TargetType: ConditionConfigurationApplied,
		DependOn: []string{
			string(status.ConditionSettingsValid),
			string(status.ConditionHooksProcessed),
			string(status.ConditionHelmApplied),
		},
	}
}
