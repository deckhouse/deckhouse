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
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	controlPlanePercent     = 40                     // %
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
	var (
		calculatedMasterNodeMilliCPU int64
		calculatedMasterNodeMemory   int64

		calculatedControlPlaneMilliCPU int64
		calculatedControlPlaneMemory   int64

		discoveryMasterNodeMilliCPU int64
		discoveryMasterNodeMemory   int64
	)

	nodes, err := sdkobjectpatch.UnmarshalToStruct[Node](input.Snapshots, "NodesResources")
	if err != nil {
		return fmt.Errorf("unmarshal NodesResources snapshots: %v", err)
	}

	// Managed cloud
	if len(nodes) == 0 {
		return nil
	}

	// Hardcoded maximum values for master node resources
	discoveryMasterNodeMilliCPU = hardLimitMilliCPU
	discoveryMasterNodeMemory = hardLimitMemory

	for _, n := range nodes {
		effCPU, effMem := effectiveMasterResources(&n)
		if effCPU < discoveryMasterNodeMilliCPU {
			discoveryMasterNodeMilliCPU = effCPU
		}
		if effMem < discoveryMasterNodeMemory {
			discoveryMasterNodeMemory = effMem
		}
	}

	calculatedMasterNodeMilliCPU = discoveryMasterNodeMilliCPU - configEveryNodeMilliCPU
	calculatedMasterNodeMemory = discoveryMasterNodeMemory - configEveryNodeMemory

	if calculatedMasterNodeMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than %dm", configEveryNodeMilliCPU)
	}

	if calculatedMasterNodeMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than %dMi", configEveryNodeMemory/1024/1024)
	}

	calculatedControlPlaneMilliCPU = calculatedMasterNodeMilliCPU * controlPlanePercent / 100
	calculatedControlPlaneMemory = calculatedMasterNodeMemory * controlPlanePercent / 100

	path := "global.modules.resourcesRequests.controlPlane.cpu"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedControlPlaneMilliCPU = quantity.MilliValue()
	}

	path = "global.modules.resourcesRequests.controlPlane.memory"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedControlPlaneMemory = quantity.Value()
	}

	input.Values.Set("global.internal.modules.resourcesRequests.milliCpuControlPlane", calculatedControlPlaneMilliCPU)
	input.Values.Set("global.internal.modules.resourcesRequests.memoryControlPlane", calculatedControlPlaneMemory)

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

func absDiff(a, b int64) int64 {
	d := a - b
	if d > 0 {
		return d
	}
	return b - a
}
