/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	autotuneScheduleName = "autotune"
	autotuneQueue        = "/modules/control-plane-manager/autotune"
	// DEBUG cron (restore before production): */5 * * * *  ← prod "0 3 * * *"
	// lookbackWindow is baked into PodMetric PromQL in
	// templates/podmetrics-autotune.yaml and must stay in sync (DEBUG 7m ← prod 7d).
)

type runAutotuneOptions struct {
	// Evaluate runs the metrics → decide → commit path (schedule). When false,
	// only repopulate values and re-emit capacityBlocked metrics (sync).
	Evaluate bool
	// Fetch overrides the metrics client; nil uses fetchComponentUsage.
	Fetch componentUsageFunc
}

// Daily schedule path: full evaluation cycle.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: autotuneQueue,
	Schedule: []go_hook.ScheduleConfig{
		{Name: autotuneScheduleName, Crontab: "*/5 * * * *"}, // DEBUG: prod "0 3 * * *"
	},
	Kubernetes: []go_hook.KubernetesConfig{
		autotuneNodesBinding(false),
		autotuneStateBinding(false),
	},
}, dependency.WithExternalDependencies(autotuneResourcesRequestsSchedule))

func autotuneResourcesRequestsSchedule(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	return runAutotune(ctx, input, dc, runAutotuneOptions{Evaluate: true})
}

func runAutotune(ctx context.Context, input *go_hook.HookInput, dc dependency.Container, opts runAutotuneOptions) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %w", err)
	}
	// Managed cloud — no master Nodes visible; leave combined-budget hook alone.
	if len(nodes) == 0 {
		return nil
	}

	// Without prometheus + PMA there are no PodMetric series. Do not keep frozen
	// per-component applied* values — fall back to the legacy combined-budget
	// split from resources_requests_calculate.go.
	if !controlPlaneAutotuneActive(input) {
		return discardAutotuneForLegacy(input)
	}

	state, err := readAutotuneState(input)
	if err != nil {
		return err
	}
	stateDirty := false

	cpuOverridden := isMeasurementOverridden(input, resourceCPU)
	memoryOverridden := isMeasurementOverridden(input, resourceMemory)
	if cpuOverridden {
		input.Logger.Info("autotune: cpu measurement overridden by config, skipping cpu autotune")
	}
	if memoryOverridden {
		input.Logger.Info("autotune: memory measurement overridden by config, skipping memory autotune")
	}

	if cpuOverridden && state.CPU != nil {
		state.deleteMeasurement(resourceCPU)
		stateDirty = true
	}
	if memoryOverridden && state.Memory != nil {
		state.deleteMeasurement(resourceMemory)
		stateDirty = true
	}

	budgetCPU, budgetMem, _ := minMasterNodeBudget(nodes)
	combinedCPU := input.Values.Get(pathMilliCPUControlPlane).Int()
	combinedMem := input.Values.Get(pathMemoryControlPlane).Int()

	fetch := opts.Fetch
	if fetch == nil {
		fetch = fetchComponentUsage
	}

	// Evaluate path: recommendations from metrics. Repopulate values exactly
	// once at the end — a second Remove of `components` fails merge when Exists
	// still sees the pre-patch snapshot.
	if opts.Evaluate {
		now := dc.GetClock().Now().UTC()

		recsCPU, cpuUsageOK := fetchRecs(ctx, dc, fetch, resourceCPU, cpuOverridden, budgetCPU, func(comp string, ferr error) {
			input.Logger.Warn("autotune: metrics API cpu fetch failed", "component", comp, "error", ferr)
		})
		recsMem, memUsageOK := fetchRecs(ctx, dc, fetch, resourceMemory, memoryOverridden, budgetMem, func(comp string, ferr error) {
			input.Logger.Warn("autotune: metrics API memory fetch failed", "component", comp, "error", ferr)
		})
		usageOK := cpuUsageOK && memUsageOK

		plan := planInitialSnapshot(state, cpuOverridden, memoryOverridden, recsCPU, recsMem)
		if plan.InitialCPU && plan.InitialMem && !plan.CPUReady {
			input.Logger.Info("autotune: waiting for complete cpu+memory recommendations before initial snapshot",
				"cpuHave", len(recsCPU), "memoryHave", len(recsMem), "need", len(controlPlaneComponents))
		} else {
			if plan.InitialCPU && !plan.CPUReady {
				input.Logger.Info("autotune: waiting for complete cpu recommendations before initial snapshot",
					"have", len(recsCPU), "need", len(controlPlaneComponents))
			}
			if plan.InitialMem && !plan.MemReady {
				input.Logger.Info("autotune: waiting for complete memory recommendations before initial snapshot",
					"have", len(recsMem), "need", len(controlPlaneComponents))
			}
		}

		// Missing/failed metrics: do not mutate applied*; keep capacityBlocked as-is.
		// Evaluate each measurement independently — do NOT use `stateDirty || evaluate...`
		// or a successful cpu commit short-circuits and skips memory entirely.
		if usageOK || len(recsCPU) > 0 || len(recsMem) > 0 {
			if !cpuOverridden && plan.CPUReady {
				if len(recsCPU) == 0 {
					input.Logger.Warn("autotune: no cpu usage datapoints from metrics API, leaving cpu state unchanged")
				}
				if evaluateMeasurement(input, state, resourceCPU, recsCPU, budgetCPU, combinedCPU, now) {
					stateDirty = true
				}
				if plan.InitialCPU {
					if fillMissingAppliedFromFallback(state, resourceCPU, combinedCPU) {
						stateDirty = true
					}
				}
			}
			if !memoryOverridden && plan.MemReady {
				if len(recsMem) == 0 {
					input.Logger.Warn("autotune: no memory usage datapoints from metrics API, leaving memory state unchanged")
				}
				if evaluateMeasurement(input, state, resourceMemory, recsMem, budgetMem, combinedMem, now) {
					stateDirty = true
				}
				if plan.InitialMem {
					if fillMissingAppliedFromFallback(state, resourceMemory, combinedMem) {
						stateDirty = true
					}
				}
			}
		}
	}

	input.MetricsCollector.Expire(autotuneMetricGroup)
	emitCapacityBlockedMetrics(input, state)
	repopulateComponents(input, state, cpuOverridden, memoryOverridden)

	if stateDirty {
		return persistAutotuneState(input, state)
	}
	return nil
}
