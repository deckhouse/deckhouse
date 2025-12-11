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

// Package metrics provides centralized metric names for deckhouse-controller.
// All metric names use constants to ensure consistency and prevent typos.
// The deckhouse_ placeholder is replaced by the metrics storage with the appropriate prefix.
package metrics

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Metric name constants organized by functional area.
// Each constant represents a unique metric name used throughout deckhouse-controller.
const (
	// ============================================================================
	// Module Manager Metrics
	// ============================================================================
	// Migration and experimental module tracking
	MigratedModuleNotFoundMetricName        = "d8_migrated_module_not_found"
	ExperimentalModulesAreAllowedMetricName = "is_experimental_modules_allowed"
	ExperimentalModuleIsEnabled             = "is_experimental_module_enabled"
	DeprecatedModuleIsEnabled               = "is_deprecated_module_enabled"

	// ============================================================================
	// Deckhouse Release Controller Metrics
	// ============================================================================
	// Registry and digest check counters
	DeckhouseRegistryCheckTotal          = "deckhouse_registry_check_total"
	DeckhouseKubeImageDigestCheckTotal   = "deckhouse_kube_image_digest_check_total"
	DeckhouseRegistryCheckErrorsTotal    = "deckhouse_registry_check_errors_total"
	DeckhouseKubeImageDigestCheckSuccess = "deckhouse_kube_image_digest_check_success"

	// Update status gauges
	D8IsUpdating       = "d8_is_updating"
	D8UpdatingIsFailed = "d8_updating_is_failed"

	// ============================================================================
	// Module Config Controller Metrics
	// ============================================================================
	// Configuration status tracking
	D8ModuleConfigObsoleteVersion  = "d8_module_config_obsolete_version"
	D8ModuleAtConflict             = "d8_module_at_conflict"
	D8ModuleConfigAllowedToDisable = "d8_moduleconfig_allowed_to_disable"

	// ============================================================================
	// Module Source Controller Metrics
	// ============================================================================
	// Source validation and sequence tracking
	D8ModuleUpdatingModuleIsNotValid = "d8_module_updating_module_is_not_valid"
	D8ModuleUpdatingBrokenSequence   = "d8_module_updating_broken_sequence"

	// ============================================================================
	// Module Release Controller Metrics
	// ============================================================================
	ModulePullSecondsTotal     = "deckhouse_module_pull_seconds_total"
	ModuleSizeBytesTotal       = "deckhouse_module_size_bytes_total"
	ModuleUpdatePolicyNotFound = "deckhouse_module_update_policy_not_found"
	ModuleConfigurationError   = "deckhouse_module_configuration_error"
)

// ============================================================================
// Metric Groups
// ============================================================================
const (
	MigratedModuleNotFoundGroup = "migrated_module_not_found"
	D8Updating                  = "d8_updating"
	D8ModuleUpdatingGroup       = "d8_module_updating_group"
)

// Group templates for dynamic metric names using fmt.Sprintf
const (
	ObsoleteConfigMetricGroupTemplate = "obsoleteVersion_%s"
	ModuleConflictMetricGroupTemplate = "module_%s_at_conflict"
)

// ============================================================================
// Metric Registration Functions
// ============================================================================

// RegisterDeckhouseControllerMetrics registers all metrics used by deckhouse-controller.
// This function should be called during controller initialization to ensure all metrics
// are available when needed.
func RegisterDeckhouseControllerMetrics(metricStorage metricsstorage.Storage) error {
	if err := RegisterModuleManagerMetrics(metricStorage); err != nil {
		return fmt.Errorf("register module manager metrics: %w", err)
	}

	if err := RegisterDeckhouseReleaseMetrics(metricStorage); err != nil {
		return fmt.Errorf("register deckhouse release metrics: %w", err)
	}

	if err := RegisterModuleControllerMetrics(metricStorage); err != nil {
		return fmt.Errorf("register module controller metrics: %w", err)
	}

	return nil
}

// RegisterModuleManagerMetrics registers metrics related to module management,
// including experimental modules, migrations, and deprecation tracking.
func RegisterModuleManagerMetrics(metricStorage metricsstorage.Storage) error {
	// Register module manager metrics
	moduleLabels := []string{"module"}

	// Register experimental modules allowed metric (global setting)
	_, err := metricStorage.RegisterGauge(
		WrapTelemetryMetric(ExperimentalModulesAreAllowedMetricName),
		moduleLabels,
		options.WithHelp("Gauge indicating whether experimental modules are allowed (0.0 = disabled, 1.0 = enabled)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ExperimentalModulesAreAllowedMetricName, err)
	}

	// Register per-module experimental and deprecated metrics
	_, err = metricStorage.RegisterGauge(
		WrapTelemetryMetric(ExperimentalModuleIsEnabled),
		moduleLabels,
		options.WithHelp("Gauge indicating whether a specific experimental module is enabled (0.0 = disabled, 1.0 = enabled)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ExperimentalModuleIsEnabled, err)
	}

	_, err = metricStorage.RegisterGauge(
		WrapTelemetryMetric(DeprecatedModuleIsEnabled),
		moduleLabels,
		options.WithHelp("Gauge indicating whether a specific deprecated module is enabled (0.0 = disabled, 1.0 = enabled)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", DeprecatedModuleIsEnabled, err)
	}

	// Register migrated module metric
	_, err = metricStorage.RegisterGauge(
		MigratedModuleNotFoundMetricName,
		moduleLabels,
		options.WithHelp("Gauge indicating migrated modules that were not found during startup (1.0 = not found)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", MigratedModuleNotFoundMetricName, err)
	}

	return nil
}

// RegisterDeckhouseReleaseMetrics registers metrics for Deckhouse release operations,
// including registry checks, image digest validation, and update status tracking.
func RegisterDeckhouseReleaseMetrics(metricStorage metricsstorage.Storage) error {
	// Register registry check counters
	// These counters are incremented during registry operations
	_, err := metricStorage.RegisterCounter(
		DeckhouseRegistryCheckTotal,
		[]string{},
		options.WithHelp("Counter of total registry connectivity checks performed"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", DeckhouseRegistryCheckTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		DeckhouseKubeImageDigestCheckTotal,
		[]string{},
		options.WithHelp("Counter of total Kubernetes image digest checks performed"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", DeckhouseKubeImageDigestCheckTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		DeckhouseRegistryCheckErrorsTotal,
		[]string{},
		options.WithHelp("Counter of failed registry connectivity checks"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", DeckhouseRegistryCheckErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		DeckhouseKubeImageDigestCheckSuccess,
		[]string{},
		options.WithHelp("Counter of successful Kubernetes image digest checks"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", DeckhouseKubeImageDigestCheckSuccess, err)
	}

	// Register update status gauges
	// These gauges are managed via grouped metrics and are set during release operations
	releaseLabels := []string{"deployingRelease"}

	_, err = metricStorage.RegisterGauge(
		D8IsUpdating,
		releaseLabels,
		options.WithHelp("Gauge indicating whether Deckhouse is currently updating (1.0 = updating)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8IsUpdating, err)
	}

	_, err = metricStorage.RegisterGauge(
		D8UpdatingIsFailed,
		releaseLabels,
		options.WithHelp("Gauge indicating whether Deckhouse update has failed (1.0 = failed, 0.0 = not failed)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8UpdatingIsFailed, err)
	}

	return nil
}

// RegisterModuleControllerMetrics registers metrics for module controllers,
// including config, source, and release controller metrics.
func RegisterModuleControllerMetrics(metricStorage metricsstorage.Storage) error {
	// Define common labels for module controller metrics
	moduleLabels := []string{"module", "source"}
	configLabels := []string{"module"}

	// Register counter for module operations
	// Note: These metrics use deckhouse_ placeholder which is replaced by metrics storage
	_, err := metricStorage.RegisterCounter(
		ModuleUpdatePolicyNotFound,
		moduleLabels,
		options.WithHelp("Counter of modules that do not have an update policy configured"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ModuleUpdatePolicyNotFound, err)
	}

	// Register gauges for module status tracking
	_, err = metricStorage.RegisterGauge(
		ModulePullSecondsTotal,
		moduleLabels,
		options.WithHelp("Gauge showing the duration in seconds for pulling a module release"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ModulePullSecondsTotal, err)
	}

	_, err = metricStorage.RegisterGauge(
		ModuleSizeBytesTotal,
		moduleLabels,
		options.WithHelp("Gauge showing the size in bytes of a module release"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ModuleSizeBytesTotal, err)
	}

	// ModuleConfigurationError uses different labels than other module metrics
	// because it tracks configuration errors per module version, not per source
	moduleConfigErrorLabels := []string{"module", "version"}
	_, err = metricStorage.RegisterGauge(
		ModuleConfigurationError,
		moduleConfigErrorLabels,
		options.WithHelp("Gauge indicating module configuration errors (1.0 = error present, 0.0 = no error)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", ModuleConfigurationError, err)
	}

	// Register module config controller metrics
	_, err = metricStorage.RegisterGauge(
		D8ModuleConfigObsoleteVersion,
		configLabels,
		options.WithHelp("Gauge indicating modules with obsolete configuration versions (1.0 = obsolete)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8ModuleConfigObsoleteVersion, err)
	}

	_, err = metricStorage.RegisterGauge(
		D8ModuleAtConflict,
		configLabels,
		options.WithHelp("Gauge indicating modules that are in conflict state (1.0 = conflict present)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8ModuleAtConflict, err)
	}

	_, err = metricStorage.RegisterGauge(
		D8ModuleConfigAllowedToDisable,
		configLabels,
		options.WithHelp("Gauge indicating whether a module configuration is allowed to be disabled (1.0 = allowed, 0.0 = not allowed)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8ModuleConfigAllowedToDisable, err)
	}

	// Register module source controller metrics
	_, err = metricStorage.RegisterGauge(
		D8ModuleUpdatingModuleIsNotValid,
		configLabels,
		options.WithHelp("Gauge indicating modules that are updating but not in valid state (1.0 = invalid)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8ModuleUpdatingModuleIsNotValid, err)
	}

	_, err = metricStorage.RegisterGauge(
		D8ModuleUpdatingBrokenSequence,
		configLabels,
		options.WithHelp("Gauge indicating modules with broken update sequence (1.0 = broken sequence)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", D8ModuleUpdatingBrokenSequence, err)
	}

	return nil
}

// ============================================================================
// Utility Functions
// ============================================================================

// WrapTelemetryMetric wraps a metric name with telemetry for consistent usage.
// This is a convenience function to standardize telemetry wrapping across the codebase.
func WrapTelemetryMetric(metricName string) string {
	return telemetry.WrapName(metricName)
}
