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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	obsoleteGlobalResourcesRequestsMetricName  = "d8_obsolete_global_control_plane_resources_requests"
	obsoleteGlobalResourcesRequestsMetricGroup = "D8ObsoleteGlobalControlPlaneResourcesRequests"
)

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeAll: &go_hook.OrderedConfig{Order: 20},
		Kubernetes: []go_hook.KubernetesConfig{
			controlPlaneNodesBinding(true, true),
		},
	}, calculateResourcesRequests)
)

func calculateResourcesRequests(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)

	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %v", err)
	}

	// Managed cloud
	if len(nodes) == 0 {
		return nil
	}

	calculatedMasterNodeMilliCPU, calculatedMasterNodeMemory, ok := minMasterNodeBudget(nodes)
	if !ok {
		return nil
	}

	if calculatedMasterNodeMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than %dm", configEveryNodeMilliCPU)
	}

	if calculatedMasterNodeMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than %dMi", configEveryNodeMemory/1024/1024)
	}

	discoveryMilliCPU := calculatedMasterNodeMilliCPU * controlPlanePercent / 100
	discoveryMemory := calculatedMasterNodeMemory * controlPlanePercent / 100

	resolved, err := resolveCombinedBudget(input, discoveryMilliCPU, discoveryMemory)
	if err != nil {
		return err
	}

	if resolved.UsedGlobal {
		input.MetricsCollector.Set(
			obsoleteGlobalResourcesRequestsMetricName,
			1,
			map[string]string{},
			metrics.WithGroup(obsoleteGlobalResourcesRequestsMetricGroup),
		)
	}

	// When prometheus + PMA are enabled, discovery must not stomp the combined
	// budget on every run: clearing a manual resourcesRequests override would
	// otherwise jump to a fresh %-split and restart CP pods before autotune's
	// first full per-component snapshot. Keep the previous budget sticky under
	// the same asymmetric deadband; always apply manual/global overrides and
	// always refresh when autotune cannot run (PMA/prometheus off).
	autotuneActive := controlPlaneAutotuneActive(input)
	if resolved.CPUFromConfig || !autotuneActive || significantResourceChange(resolved.MilliCPU, input.Values.Get(pathMilliCPUControlPlane).Int()) {
		input.Values.Set(pathMilliCPUControlPlane, resolved.MilliCPU)
	}
	if resolved.MemoryFromConfig || !autotuneActive || significantResourceChange(resolved.MemoryBytes, input.Values.Get(pathMemoryControlPlane).Int()) {
		input.Values.Set(pathMemoryControlPlane, resolved.MemoryBytes)
	}

	return nil
}
