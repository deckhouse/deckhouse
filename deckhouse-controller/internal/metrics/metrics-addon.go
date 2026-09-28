// Copyright 2026 Flant JSC
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

package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flant/shell-operator/pkg/task/queue"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Addon-operator metric names organized by functional area.
// Each constant represents a unique metric name used throughout addon-operator.
const (
	// ============================================================================
	// Configuration Metrics
	// ============================================================================
	// BindingCount tracks the number of bindings per module and hook
	BindingCount = "deckhouse_binding_count"
	// ConfigValuesErrorsTotal counts ConfigMap validation errors
	ConfigValuesErrorsTotal = "deckhouse_config_values_errors_total"

	// ============================================================================
	// Module Metrics
	// ============================================================================
	// ModulesDiscoverErrorsTotal counts errors during module discovery
	ModulesDiscoverErrorsTotal = "deckhouse_modules_discover_errors_total"
	// ModuleDeleteErrorsTotal counts errors during module deletion
	ModuleDeleteErrorsTotal = "deckhouse_module_delete_errors_total"
	// ModuleRunSeconds measures module execution time
	ModuleRunSeconds = "deckhouse_module_run_seconds"
	// ModuleRunErrorsTotal counts module execution errors
	ModuleRunErrorsTotal = "deckhouse_module_run_errors_total"
	// ModulesHelmReleaseRedeployedTotal counts Helm release redeployments
	ModulesHelmReleaseRedeployedTotal = "deckhouse_modules_helm_release_redeployed_total"
	// ModulesAbsentResourcesTotal counts absent resources per module
	ModulesAbsentResourcesTotal = "deckhouse_modules_absent_resources_total"
	// ModuleInfoMetricName tracks module information
	ModuleInfoMetricName = "deckhouse_mm_module_info"
	// ModuleEnabledMetricName tracks enabled modules with their deployed version
	ModuleEnabledMetricName = "deckhouse_mm_module_enabled"
	// ModuleEnabledTelemetryMetricName mirrors ModuleEnabledMetricName under the
	// d8_telemetry_ prefix so flant-integration ships the metric to DOP.
	// It intentionally has no deckhouse_ prefix and must stay that way.
	ModuleEnabledTelemetryMetricName = "d8_telemetry_module_enabled"
	// ModuleMaintenanceMetricName tracks module maintenance state
	ModuleMaintenanceMetricName = "deckhouse_mm_module_maintenance"

	// ============================================================================
	// Module Hook Metrics
	// ============================================================================
	// ModuleHookRunSeconds measures module hook execution time
	ModuleHookRunSeconds = "deckhouse_module_hook_run_seconds"
	// ModuleHookRunUserCPUSeconds measures module hook user CPU usage
	ModuleHookRunUserCPUSeconds = "deckhouse_module_hook_run_user_cpu_seconds"
	// ModuleHookRunSysCPUSeconds measures module hook system CPU usage
	ModuleHookRunSysCPUSeconds = "deckhouse_module_hook_run_sys_cpu_seconds"
	// ModuleHookRunMaxRSSBytes tracks maximum resident set size for module hooks
	ModuleHookRunMaxRSSBytes = "deckhouse_module_hook_run_max_rss_bytes"
	// ModuleHookAllowedErrorsTotal counts allowed module hook errors
	ModuleHookAllowedErrorsTotal = "deckhouse_module_hook_allowed_errors_total"
	// ModuleHookErrorsTotal counts module hook execution errors
	ModuleHookErrorsTotal = "deckhouse_module_hook_errors_total"
	// ModuleHookSuccessTotal counts successful module hook executions
	ModuleHookSuccessTotal = "deckhouse_module_hook_success_total"

	// ============================================================================
	// Global Hook Metrics
	// ============================================================================
	// GlobalHookRunSeconds measures global hook execution time
	GlobalHookRunSeconds = "deckhouse_global_hook_run_seconds"
	// GlobalHookRunUserCPUSeconds measures global hook user CPU usage
	GlobalHookRunUserCPUSeconds = "deckhouse_global_hook_run_user_cpu_seconds"
	// GlobalHookRunSysCPUSeconds measures global hook system CPU usage
	GlobalHookRunSysCPUSeconds = "deckhouse_global_hook_run_sys_cpu_seconds"
	// GlobalHookRunMaxRSSBytes tracks maximum resident set size for global hooks
	GlobalHookRunMaxRSSBytes = "deckhouse_global_hook_run_max_rss_bytes"
	// GlobalHookAllowedErrorsTotal counts allowed global hook errors
	GlobalHookAllowedErrorsTotal = "deckhouse_global_hook_allowed_errors_total"
	// GlobalHookErrorsTotal counts global hook execution errors
	GlobalHookErrorsTotal = "deckhouse_global_hook_errors_total"
	// GlobalHookSuccessTotal counts successful global hook executions
	GlobalHookSuccessTotal = "deckhouse_global_hook_success_total"

	// ============================================================================
	// Convergence Metrics
	// ============================================================================
	// ConvergenceSeconds measures convergence duration
	ConvergenceSeconds = "deckhouse_convergence_seconds"
	// ConvergenceTotal counts convergence executions
	ConvergenceTotal = "deckhouse_convergence_total"

	// ============================================================================
	// Helm Operations Metrics
	// ============================================================================
	// ModuleHelmSeconds measures Helm operation time for modules
	ModuleHelmSeconds = "deckhouse_module_helm_seconds"
	// HelmOperationSeconds measures specific Helm operation durations
	HelmOperationSeconds = "deckhouse_helm_operation_seconds"
	// HelmFallbackTotal counts how many times nelm operations had to fall back to helm3lib
	HelmFallbackTotal = "deckhouse_helm_fallback_total"
	// HelmFallbackTelemetryTotal mirrors HelmFallbackTotal under the d8_telemetry_ prefix so
	// flant-integration ships the metric to DOP.
	// It intentionally has no deckhouse_ prefix and must stay that way.
	HelmFallbackTelemetryTotal = "d8_telemetry_helm_fallback_total"

	// ============================================================================
	// Task Queue Metrics
	// ============================================================================
	// AddonTaskWaitInQueueSecondsTotal measures time tasks wait in queue
	AddonTaskWaitInQueueSecondsTotal = "deckhouse_task_wait_in_queue_seconds_total"
	// AddonTasksQueueLength shows current length of task queues
	AddonTasksQueueLength = "deckhouse_tasks_queue_length"
	// TasksQueueHeadInfo shows the head element of each non-empty task queue (module, task_type, hook)
	TasksQueueHeadInfo = "deckhouse_tasks_queue_head_info"

	// ============================================================================
	// Live Ticks Metrics
	// ============================================================================
	// AddonLiveTicks is a counter that increases every 10 seconds to indicate addon-operator is alive
	AddonLiveTicks = "deckhouse_live_ticks"
)

// tasksQueueHeadInfoMetricGroup is the metric group name (without prefix) for tasks_queue_head_info.
const tasksQueueHeadInfoMetricGroup = "tasks_queue_head_info"

// Standard histogram buckets for timing metrics (1ms to 10s)
var buckets1msTo10s = []float64{
	0.0,
	0.001, 0.002, 0.005, // 1,2,5 milliseconds
	0.01, 0.02, 0.05, // 10,20,50 milliseconds
	0.1, 0.2, 0.5, // 100,200,500 milliseconds
	1, 2, 5, // 1,2,5 seconds
	10, // 10 seconds
}

// ============================================================================
// Registration Functions
// ============================================================================

// RegisterAddonHookMetrics registers all addon-operator specific metrics with the provided storage.
// This includes configuration, module, hook, convergence, Helm, and task queue metrics.
// Returns an error if any metric registration fails.
func RegisterAddonHookMetrics(metricStorage metricsstorage.Storage) error {
	// Register configuration metrics
	if err := registerConfigurationMetrics(metricStorage); err != nil {
		return fmt.Errorf("register configuration metrics: %w", err)
	}

	// Register module metrics
	if err := registerModuleMetrics(metricStorage); err != nil {
		return fmt.Errorf("register module metrics: %w", err)
	}

	// Register module hook metrics
	if err := registerModuleHookMetrics(metricStorage); err != nil {
		return fmt.Errorf("register module hook metrics: %w", err)
	}

	// Register global hook metrics
	if err := registerGlobalHookMetrics(metricStorage); err != nil {
		return fmt.Errorf("register global hook metrics: %w", err)
	}

	// Register convergence metrics
	if err := registerConvergenceMetrics(metricStorage); err != nil {
		return fmt.Errorf("register convergence metrics: %w", err)
	}

	// Register Helm metrics
	if err := registerHelmMetrics(metricStorage); err != nil {
		return fmt.Errorf("register helm metrics: %w", err)
	}

	// Register task queue metrics
	if err := registerAddonTaskQueueMetrics(metricStorage); err != nil {
		return fmt.Errorf("register task queue metrics: %w", err)
	}

	return nil
}

// registerConfigurationMetrics registers metrics related to configuration and bindings
func registerConfigurationMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterGauge(
		BindingCount,
		[]string{LabelModule, LabelHook},
		options.WithHelp("Number of bindings per module and hook"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", BindingCount, err)
	}

	_, err = metricStorage.RegisterCounter(
		ConfigValuesErrorsTotal,
		[]string{},
		options.WithHelp("Counter of ConfigMap validation errors"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ConfigValuesErrorsTotal, err)
	}

	return nil
}

// registerModuleMetrics registers metrics related to module operations
func registerModuleMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		ModulesDiscoverErrorsTotal,
		[]string{},
		options.WithHelp("Counter of errors during module discovery"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModulesDiscoverErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModuleDeleteErrorsTotal,
		[]string{LabelModule},
		options.WithHelp("Counter of errors during module deletion"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleDeleteErrorsTotal, err)
	}

	_, err = metricStorage.RegisterHistogram(
		ModuleRunSeconds,
		[]string{LabelModule, LabelActivation},
		buckets1msTo10s,
		options.WithHelp("Histogram of module execution times in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleRunSeconds, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModuleRunErrorsTotal,
		[]string{LabelModule},
		options.WithHelp("Counter of module execution errors"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleRunErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModulesHelmReleaseRedeployedTotal,
		[]string{LabelModule},
		options.WithHelp("Counter of Helm release redeployments per module"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModulesHelmReleaseRedeployedTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModulesAbsentResourcesTotal,
		[]string{LabelModule, LabelResource},
		options.WithHelp("Counter of absent resources per module and resource"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModulesAbsentResourcesTotal, err)
	}

	return nil
}

// registerModuleHookMetrics registers metrics related to module hook execution
func registerModuleHookMetrics(metricStorage metricsstorage.Storage) error {
	moduleHookLabels := []string{
		LabelModule,
		LabelHook,
		LabelBinding,
		LabelQueue,
		LabelActivation,
	}

	_, err := metricStorage.RegisterHistogram(
		ModuleHookRunSeconds,
		moduleHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of module hook execution times in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookRunSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		ModuleHookRunUserCPUSeconds,
		moduleHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of module hook user CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookRunUserCPUSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		ModuleHookRunSysCPUSeconds,
		moduleHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of module hook system CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookRunSysCPUSeconds, err)
	}

	_, err = metricStorage.RegisterGauge(
		ModuleHookRunMaxRSSBytes,
		moduleHookLabels,
		options.WithHelp("Gauge of maximum resident set size used by module hook in bytes"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookRunMaxRSSBytes, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModuleHookAllowedErrorsTotal,
		moduleHookLabels,
		options.WithHelp("Counter of module hook execution errors that are allowed to fail (allowFailure: true)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookAllowedErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModuleHookErrorsTotal,
		moduleHookLabels,
		options.WithHelp("Counter of module hook execution errors (allowFailure: false)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		ModuleHookSuccessTotal,
		moduleHookLabels,
		options.WithHelp("Counter of successful module hook executions"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHookSuccessTotal, err)
	}

	return nil
}

// registerGlobalHookMetrics registers metrics related to global hook execution
func registerGlobalHookMetrics(metricStorage metricsstorage.Storage) error {
	globalHookLabels := []string{
		LabelHook,
		LabelBinding,
		LabelQueue,
		LabelActivation,
	}

	_, err := metricStorage.RegisterHistogram(
		GlobalHookRunSeconds,
		globalHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of global hook execution times in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookRunSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		GlobalHookRunUserCPUSeconds,
		globalHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of global hook user CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookRunUserCPUSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		GlobalHookRunSysCPUSeconds,
		globalHookLabels,
		buckets1msTo10s,
		options.WithHelp("Histogram of global hook system CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookRunSysCPUSeconds, err)
	}

	_, err = metricStorage.RegisterGauge(
		GlobalHookRunMaxRSSBytes,
		globalHookLabels,
		options.WithHelp("Gauge of maximum resident set size used by global hook in bytes"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookRunMaxRSSBytes, err)
	}

	_, err = metricStorage.RegisterCounter(
		GlobalHookAllowedErrorsTotal,
		globalHookLabels,
		options.WithHelp("Counter of global hook execution errors that are allowed to fail (allowFailure: true)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookAllowedErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		GlobalHookErrorsTotal,
		globalHookLabels,
		options.WithHelp("Counter of global hook execution errors (allowFailure: false)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		GlobalHookSuccessTotal,
		globalHookLabels,
		options.WithHelp("Counter of successful global hook executions"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", GlobalHookSuccessTotal, err)
	}

	return nil
}

// registerConvergenceMetrics registers metrics related to convergence operations
func registerConvergenceMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		ConvergenceSeconds,
		[]string{LabelActivation},
		options.WithHelp("Counter of convergence duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ConvergenceSeconds, err)
	}

	_, err = metricStorage.RegisterCounter(
		ConvergenceTotal,
		[]string{LabelActivation},
		options.WithHelp("Counter of convergence executions"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ConvergenceTotal, err)
	}

	return nil
}

// registerHelmMetrics registers metrics related to Helm operations
func registerHelmMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterHistogram(
		ModuleHelmSeconds,
		[]string{LabelModule, LabelActivation},
		buckets1msTo10s,
		options.WithHelp("Histogram of Helm operation times for modules in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", ModuleHelmSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		HelmOperationSeconds,
		[]string{LabelModule, LabelActivation, LabelOperation},
		buckets1msTo10s,
		options.WithHelp("Histogram of specific Helm operation durations in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HelmOperationSeconds, err)
	}

	_, err = metricStorage.RegisterCounter(
		HelmFallbackTotal,
		[]string{LabelModule, LabelOperation, LabelErrorType},
		options.WithHelp("Counter of nelm Helm operations that fell back to helm3lib, labeled by module, operation and error type"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HelmFallbackTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		HelmFallbackTelemetryTotal,
		[]string{LabelModule, LabelOperation, LabelErrorType},
		options.WithHelp("Counter of nelm Helm operations that fell back to helm3lib, labeled by module, operation and error type"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HelmFallbackTelemetryTotal, err)
	}

	return nil
}

// registerAddonTaskQueueMetrics registers metrics related to task queue operations
func registerAddonTaskQueueMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		AddonTaskWaitInQueueSecondsTotal,
		[]string{LabelModule, LabelHook, LabelBinding, LabelQueue},
		options.WithHelp("Counter of seconds that tasks waited in queue before execution"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", AddonTaskWaitInQueueSecondsTotal, err)
	}

	return nil
}

// ============================================================================
// Live Metric Updaters
// ============================================================================

// StartAddonLiveTicksUpdater starts a goroutine that periodically updates
// the live_ticks metric every 10 seconds.
// This metric can be used to verify that addon-operator is alive and functioning.
func StartAddonLiveTicksUpdater(metricStorage metricsstorage.Storage) {
	// Register the live ticks counter
	_, _ = metricStorage.RegisterCounter(
		AddonLiveTicks,
		[]string{},
		options.WithHelp("Counter that increases every 10 seconds to indicate addon-operator is alive"),
	)

	// Start the updater goroutine
	go func() {
		for {
			metricStorage.CounterAdd(AddonLiveTicks, 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()
}

// StartAddonTasksQueueLengthUpdater starts a goroutine that periodically updates
// the tasks_queue_length and tasks_queue_head_info metrics every 5 seconds.
// These metrics show the number of pending tasks and the head element info
// (module, task_type, hook) of each non-empty queue.
// headInfoExtractor is a callback that extracts (module, hook) from a task's raw metadata.
func StartAddonTasksQueueLengthUpdater(metricStorage metricsstorage.Storage, tqs *queue.TaskQueueSet, headInfoExtractor func(metadata interface{}) (module, hook string)) {
	// Register the tasks queue length gauge
	_, _ = metricStorage.RegisterGauge(
		AddonTasksQueueLength,
		[]string{LabelQueue},
		options.WithHelp("Gauge showing the length of the task queue"),
	)

	// Register the tasks queue head info gauge
	_, _ = metricStorage.RegisterGauge(
		TasksQueueHeadInfo,
		[]string{LabelQueue, LabelModule, LabelTaskType, LabelHook},
		options.WithHelp("Head info of each non-empty task queue (module, task_type, hook)"),
	)

	// Start the updater goroutine
	go func() {
		for {
			// Gather task queues lengths.
			tqs.IterateSnapshot(context.TODO(), func(_ context.Context, q *queue.TaskQueue) {
				queueLen := float64(q.Length())
				metricStorage.GaugeSet(AddonTasksQueueLength, queueLen, map[string]string{LabelQueue: q.Name})
			})

			// Publish head_info for each non-empty queue, expiring old series first.
			updateTasksQueueHeadInfo(tqs, metricStorage, headInfoExtractor)

			time.Sleep(5 * time.Second)
		}
	}()
}

// updateTasksQueueHeadInfo publishes head_info metrics for all non-empty queues,
// properly expiring old series before republishing to prevent phantom series
// from persisting after a queue head changes.
func updateTasksQueueHeadInfo(tqs *queue.TaskQueueSet, metricStorage metricsstorage.Storage, headInfoExtractor func(metadata interface{}) (module, hook string)) {
	metricStorage.Grouped().ExpireGroupMetricByName(tasksQueueHeadInfoMetricGroup, TasksQueueHeadInfo)

	tqs.IterateSnapshot(context.TODO(), func(_ context.Context, q *queue.TaskQueue) {
		t := q.GetFirst()
		if t == nil {
			return
		}

		module, hook := headInfoExtractor(t.GetMetadata())

		// Normalize ParallelModuleRun synthetic module names:
		// "Parallel run for a, b, c" -> "" to avoid false joins with deckhouse_mm_module_info.
		if strings.HasPrefix(module, "Parallel run for ") {
			module = ""
		}

		metricStorage.Grouped().GaugeSet(
			tasksQueueHeadInfoMetricGroup,
			TasksQueueHeadInfo,
			1,
			map[string]string{
				LabelQueue:    q.Name,
				LabelModule:   module,
				LabelTaskType: string(t.GetType()),
				LabelHook:     hook,
			},
		)
	})
}
