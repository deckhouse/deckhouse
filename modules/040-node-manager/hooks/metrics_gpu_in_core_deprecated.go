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

// This hook exports a metric for every NodeGroup that relies on the deprecated
// built-in (in-core) GPU support: `spec.gpu` is set while the `gpu` module is disabled.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	gpuInCoreDeprecatedMetricName  = "d8_node_group_gpu_in_core_deprecated"
	gpuInCoreDeprecatedMetricGroup = "d8_node_group_gpu_in_core_deprecated"
	gpuInCoreDeprecatedSnapshot    = "gpu_nodegroups"
	gpuModuleName                  = "gpu"
)

// Both triggers are required, they cover different inputs of the metric:
//   - NodeGroup events, so that editing or removing `spec.gpu` is reflected right away;
//   - OnAfterHelm, so that enabling or disabling the `gpu` module is reflected as well.
//     The alert derived from this metric asks the operator to enable the `gpu` module,
//     which only changes `global.enabledModules` and touches no NodeGroup, so without
//     this binding the metric would never be expired and the alert would keep firing
//     while asking to enable an already enabled module (and, symmetrically, disabling
//     the module would not produce the metric until the next NodeGroup event).
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/node-manager",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         gpuInCoreDeprecatedSnapshot,
			ExecuteHookOnSynchronization: ptr.To(true),
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			FilterFunc:                   filterNodeGroupGPUUsage,
		},
	},
}, handleGPUInCoreDeprecatedMetrics)

type nodeGroupGPUUsage struct {
	Name   string `json:"name"`
	HasGPU bool   `json:"hasGPU"`
}

func filterNodeGroupGPUUsage(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	if err := sdk.FromUnstructured(obj, &nodeGroup); err != nil {
		return nil, err
	}

	return nodeGroupGPUUsage{
		Name:   nodeGroup.Name,
		HasGPU: !nodeGroup.Spec.GPU.IsEmpty(),
	}, nil
}

// isGPUModuleEnabled repeats the check used by the GPU labeling hook (see gpu_enabled.go),
// so that the metric and the labeling never disagree on whether the module is enabled.
func isGPUModuleEnabled(input *go_hook.HookInput) bool {
	if !input.Values.Exists("global.enabledModules") {
		return false
	}

	for _, module := range input.Values.Get("global.enabledModules").Array() {
		if module.String() == gpuModuleName {
			return true
		}
	}

	return false
}

func handleGPUInCoreDeprecatedMetrics(_ context.Context, input *go_hook.HookInput) error {
	// Expire on every run, so that metrics of deleted NodeGroups (or of NodeGroups
	// that no longer declare `spec.gpu`) do not stick around.
	input.MetricsCollector.Expire(gpuInCoreDeprecatedMetricGroup)

	// The gpu module owns the GPU stack, in-core support stays idle - nothing to report.
	if isGPUModuleEnabled(input) {
		return nil
	}

	for ng, err := range sdkobjectpatch.SnapshotIter[nodeGroupGPUUsage](input.Snapshots.Get(gpuInCoreDeprecatedSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", gpuInCoreDeprecatedSnapshot, err)
		}

		if !ng.HasGPU {
			continue
		}

		input.MetricsCollector.Set(
			gpuInCoreDeprecatedMetricName,
			1,
			map[string]string{"name": ng.Name},
			metrics.WithGroup(gpuInCoreDeprecatedMetricGroup),
		)
	}

	return nil
}
