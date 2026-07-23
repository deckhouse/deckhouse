// Copyright 2021 Flant JSC
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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
			{
				Name:       "NodesResources",
				ApiVersion: "v1",
				Kind:       "Node",
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: metav1.LabelSelectorOpExists,
					},
				}},
				FilterFunc: applyNodesResourcesFilter,
			},
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

	calculatedControlPlaneMilliCPU := calculatedMasterNodeMilliCPU * controlPlanePercent / 100
	calculatedControlPlaneMemory := calculatedMasterNodeMemory * controlPlanePercent / 100

	cpmCPUPath := "controlPlaneManager.resourcesRequests.cpu"
	cpmMemoryPath := "controlPlaneManager.resourcesRequests.memory"
	globalCPUPath := "global.modules.resourcesRequests.controlPlane.cpu"
	globalMemoryPath := "global.modules.resourcesRequests.controlPlane.memory"

	cpmCPUExists := input.Values.Exists(cpmCPUPath)
	cpmMemoryExists := input.Values.Exists(cpmMemoryPath)
	globalCPUExists := input.Values.Exists(globalCPUPath)
	globalMemoryExists := input.Values.Exists(globalMemoryPath)

	usedGlobalFallback := false

	if cpmCPUExists {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(cpmCPUPath))
		if err != nil {
			return err
		}
		calculatedControlPlaneMilliCPU = quantity.MilliValue()
	} else if globalCPUExists {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(globalCPUPath))
		if err != nil {
			return err
		}
		calculatedControlPlaneMilliCPU = quantity.MilliValue()
		usedGlobalFallback = true
	}

	if cpmMemoryExists {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(cpmMemoryPath))
		if err != nil {
			return err
		}
		calculatedControlPlaneMemory = quantity.Value()
	} else if globalMemoryExists {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(globalMemoryPath))
		if err != nil {
			return err
		}
		calculatedControlPlaneMemory = quantity.Value()
		usedGlobalFallback = true
	}

	if usedGlobalFallback {
		input.MetricsCollector.Set(
			obsoleteGlobalResourcesRequestsMetricName,
			1,
			map[string]string{},
			metrics.WithGroup(obsoleteGlobalResourcesRequestsMetricGroup),
		)
	}

	input.Values.Set("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane", calculatedControlPlaneMilliCPU)
	input.Values.Set("controlPlaneManager.internal.resourcesRequests.memoryControlPlane", calculatedControlPlaneMemory)

	return nil
}

func getAndParseResourceQuantity(input gjson.Result) (resource.Quantity, error) {
	var quantity resource.Quantity

	strVal := input.String()
	quantity, err := resource.ParseQuantity(strVal)
	if err != nil {
		return quantity, fmt.Errorf("cannot parse '%v': %v", strVal, err)
	}

	return quantity, nil
}
