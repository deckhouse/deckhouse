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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	configEveryNodeMilliCPU = 300                    // 0.3 Cpu
	configEveryNodeMemory   = 512 * 1024 * 1024      // 512Mb
	hardLimitMilliCPU       = 4 * 1000               // 4 Cpu
	hardLimitMemory         = 8 * 1024 * 1024 * 1024 // 8G ram

	// Minimum kubelet reservation we account for, regardless of what the kubelet
	// has actually reported on Node.Status.Allocatable at the moment the hook
	// runs. The hook uses Capacity (immutable) and subtracts max(actual kubelet
	// reservation, this floor) so the result is identical before and after the
	// kubelet finishes initialising — which avoids a second hook run later that
	// would re-render every control-plane static-pod manifest and cascade-restart
	// kube-apiserver/etcd/kcm/ks right in the middle of Deckhouse install.
	kubeletResourceReservationMemoryFloor = 900 * 1024 * 1024 // 900 MiB
	kubeletResourceReservationCPUFloor    = 100               // 0.1 cpu

	obsoleteGlobalResourcesRequestsMetricName  = "d8_obsolete_global_control_plane_resources_requests"
	obsoleteGlobalResourcesRequestsMetricGroup = "D8ObsoleteGlobalControlPlaneResourcesRequests"
)

type Node struct {
	CapacityMilliCPU    int64
	CapacityMemory      int64
	AllocatableMilliCPU int64
	AllocatableMemory   int64
}

func applyNodesResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	n := &Node{
		AllocatableMilliCPU: node.Status.Allocatable.Cpu().MilliValue(),
		AllocatableMemory:   node.Status.Allocatable.Memory().Value(),
		CapacityMilliCPU:    node.Status.Capacity.Cpu().MilliValue(),
		CapacityMemory:      node.Status.Capacity.Memory().Value(),
	}
	// Test fixtures and very early node objects may not report Capacity yet —
	// fall back to Allocatable. The downstream logic treats `Capacity == Allocatable`
	// as `kubelet has not subtracted its reservation yet` and applies the floor.
	if n.CapacityMilliCPU == 0 {
		n.CapacityMilliCPU = n.AllocatableMilliCPU
	}
	if n.CapacityMemory == 0 {
		n.CapacityMemory = n.AllocatableMemory
	}

	return n, nil
}

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

// effectiveMasterResources returns the per-node usable CPU/memory budget the
// control-plane allocation can be carved out of. Computed from Node.Status.Capacity
// (immutable for the lifetime of the node) minus max(actual kubelet reservation,
// our floor). The result is stable across the kubelet warm-up window, so the
// hook output does not flip a few minutes into the bootstrap.
func effectiveMasterResources(n *Node) (int64, int64) {
	cpuReservation := n.CapacityMilliCPU - n.AllocatableMilliCPU
	if cpuReservation < kubeletResourceReservationCPUFloor {
		cpuReservation = kubeletResourceReservationCPUFloor
	}
	memReservation := n.CapacityMemory - n.AllocatableMemory
	if memReservation < kubeletResourceReservationMemoryFloor {
		memReservation = kubeletResourceReservationMemoryFloor
	}
	return n.CapacityMilliCPU - cpuReservation, n.CapacityMemory - memReservation
}

func calculateResourcesRequests(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteGlobalResourcesRequestsMetricGroup)

	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %v", err)
	}

	// Managed cloud (no master nodes under our control, e.g. GKE) — leave the
	// requests at zero, control-plane static pods are not rendered there.
	if len(nodes) == 0 {
		return nil
	}

	// Validate the smallest master can host the per-node reservation budget.
	// In auto mode the actual per-component requests are sized by cluster node
	// count inside the static-pod templates, not by master capacity — this check
	// only guards against masters too small to run a control plane at all.
	discoveryMasterNodeMilliCPU := int64(hardLimitMilliCPU)
	discoveryMasterNodeMemory := int64(hardLimitMemory)
	for _, n := range nodes {
		effCPU, effMem := effectiveMasterResources(&n)
		if effCPU < discoveryMasterNodeMilliCPU {
			discoveryMasterNodeMilliCPU = effCPU
		}
		if effMem < discoveryMasterNodeMemory {
			discoveryMasterNodeMemory = effMem
		}
	}

	if discoveryMasterNodeMilliCPU-configEveryNodeMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than %dm", configEveryNodeMilliCPU)
	}
	if discoveryMasterNodeMemory-configEveryNodeMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than %dMi", configEveryNodeMemory/1024/1024)
	}

	// Auto mode keeps the pool at zero so the static-pod templates compute
	// per-component requests (floor + linear growth by node count, capped).
	// A manual override (controlPlaneManager.resourcesRequests, or the obsolete
	// global.modules.resourcesRequests.controlPlane fallback) switches a resource
	// back to the single-pool model that the templates split by the historical
	// component shares. CPU and memory are overridden independently.
	var (
		controlPlaneMilliCPU int64
		controlPlaneMemory   int64
		usedGlobalFallback   bool
	)

	cpmCPUPath := "controlPlaneManager.resourcesRequests.cpu"
	cpmMemoryPath := "controlPlaneManager.resourcesRequests.memory"
	globalCPUPath := "global.modules.resourcesRequests.controlPlane.cpu"
	globalMemoryPath := "global.modules.resourcesRequests.controlPlane.memory"

	if input.Values.Exists(cpmCPUPath) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(cpmCPUPath))
		if err != nil {
			return err
		}
		controlPlaneMilliCPU = quantity.MilliValue()
	} else if input.Values.Exists(globalCPUPath) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(globalCPUPath))
		if err != nil {
			return err
		}
		controlPlaneMilliCPU = quantity.MilliValue()
		usedGlobalFallback = true
	}

	if input.Values.Exists(cpmMemoryPath) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(cpmMemoryPath))
		if err != nil {
			return err
		}
		controlPlaneMemory = quantity.Value()
	} else if input.Values.Exists(globalMemoryPath) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(globalMemoryPath))
		if err != nil {
			return err
		}
		controlPlaneMemory = quantity.Value()
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

	input.Values.Set("controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane", controlPlaneMilliCPU)
	input.Values.Set("controlPlaneManager.internal.resourcesRequests.memoryControlPlane", controlPlaneMemory)

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
