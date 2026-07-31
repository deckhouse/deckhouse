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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// controlPlaneAutotuneActive is true when prometheus and prometheus-metrics-adapter
// are enabled — the custom.metrics.k8s.io path used by per-component autotune.
func controlPlaneAutotuneActive(input *go_hook.HookInput) bool {
	enabled := set.NewFromValues(input.Values, "global.enabledModules")
	return enabled.Has("prometheus") && enabled.Has("prometheus-metrics-adapter")
}

func configQuantityPresent(v gjson.Result) bool {
	return v.Exists() && strings.TrimSpace(v.String()) != ""
}

func getAndParseResourceQuantity(input gjson.Result) (resource.Quantity, error) {
	strVal := input.String()
	quantity, err := resource.ParseQuantity(strVal)
	if err != nil {
		return quantity, fmt.Errorf("cannot parse '%v': %v", strVal, err)
	}
	return quantity, nil
}

// measurementOverridePaths returns ModuleConfig then global fallback paths for a measurement.
func measurementOverridePaths(resourceName resourceKind) []string {
	switch resourceName {
	case resourceCPU:
		return []string{pathCPMCPU, pathGlobalCPU}
	case resourceMemory:
		return []string{pathCPMMemory, pathGlobalMemory}
	default:
		return nil
	}
}

// isMeasurementOverridden is true when CPM or global config sets a non-empty quantity
// for the measurement. Empty strings left by openapi/merge after clearing ModuleConfig
// are ignored so autotune is not permanently skipped.
func isMeasurementOverridden(input *go_hook.HookInput, resourceName resourceKind) bool {
	for _, path := range measurementOverridePaths(resourceName) {
		if configQuantityPresent(input.Values.Get(path)) {
			return true
		}
	}
	return false
}

type resolvedCombinedBudget struct {
	MilliCPU         int64
	MemoryBytes      int64
	CPUFromConfig    bool
	MemoryFromConfig bool
	UsedGlobal       bool
}

// resolveCombinedBudget applies CPM/global overrides on top of discovery-calculated
// combined control-plane budgets.
func resolveCombinedBudget(
	input *go_hook.HookInput,
	discoveryMilliCPU, discoveryMemory int64,
) (resolvedCombinedBudget, error) {
	out := resolvedCombinedBudget{
		MilliCPU:    discoveryMilliCPU,
		MemoryBytes: discoveryMemory,
	}

	cpmCPU := input.Values.Get(pathCPMCPU)
	cpmMemory := input.Values.Get(pathCPMMemory)
	globalCPU := input.Values.Get(pathGlobalCPU)
	globalMemory := input.Values.Get(pathGlobalMemory)

	cpmCPUExists := configQuantityPresent(cpmCPU)
	cpmMemoryExists := configQuantityPresent(cpmMemory)
	globalCPUExists := configQuantityPresent(globalCPU)
	globalMemoryExists := configQuantityPresent(globalMemory)

	if cpmCPUExists {
		quantity, err := getAndParseResourceQuantity(cpmCPU)
		if err != nil {
			return out, err
		}
		out.MilliCPU = quantity.MilliValue()
		out.CPUFromConfig = true
	} else if globalCPUExists {
		quantity, err := getAndParseResourceQuantity(globalCPU)
		if err != nil {
			return out, err
		}
		out.MilliCPU = quantity.MilliValue()
		out.CPUFromConfig = true
		out.UsedGlobal = true
	}

	if cpmMemoryExists {
		quantity, err := getAndParseResourceQuantity(cpmMemory)
		if err != nil {
			return out, err
		}
		out.MemoryBytes = quantity.Value()
		out.MemoryFromConfig = true
	} else if globalMemoryExists {
		quantity, err := getAndParseResourceQuantity(globalMemory)
		if err != nil {
			return out, err
		}
		out.MemoryBytes = quantity.Value()
		out.MemoryFromConfig = true
		out.UsedGlobal = true
	}

	return out, nil
}
