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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	controlPlanePercent     = 40                // %
	configEveryNodeMilliCPU = 300               // 0.3 Cpu
	configEveryNodeMemory   = 512 * 1024 * 1024 // 512Mb
	hardLimitMilliCPU       = 4 * 1000          // 4 Cpu
	hardLimitMemory         = 8 * 1024 * 1024 * 1024

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

// Control-plane component keys used in internal values / ConfigMap / templates.
// Container names match static-pod container names and PodMetric selectors.
const (
	componentKubeApiserver         = "kubeApiserver"
	componentEtcd                  = "etcd"
	componentKubeControllerManager = "kubeControllerManager"
	componentKubeScheduler         = "kubeScheduler"

	containerKubeApiserver         = "kube-apiserver"
	containerEtcd                  = "etcd"
	containerKubeControllerManager = "kube-controller-manager"
	containerKubeScheduler         = "kube-scheduler"

	resourceCPU    = "cpu"
	resourceMemory = "memory"
)

// controlPlaneComponents lists components in a stable order.
var controlPlaneComponents = []string{
	componentKubeApiserver,
	componentEtcd,
	componentKubeControllerManager,
	componentKubeScheduler,
}

// componentContainer maps internal component key → static-pod container name.
var componentContainer = map[string]string{
	componentKubeApiserver:         containerKubeApiserver,
	componentEtcd:                  containerEtcd,
	componentKubeControllerManager: containerKubeControllerManager,
	componentKubeScheduler:         containerKubeScheduler,
}

// componentFallbackPercent is the fixed %-split used when per-component
// autotune values are absent (bootstrap / manual override / cold start).
var componentFallbackPercent = map[string]int64{
	componentKubeApiserver:         33,
	componentEtcd:                  35,
	componentKubeControllerManager: 20,
	componentKubeScheduler:         10,
}

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

// minMasterNodeBudget returns the control-plane allocatable budget of the
// weakest master: effectiveMasterResources(minNode) − configEveryNode.
// Returns false when there are no master nodes (managed cloud).
func minMasterNodeBudget(nodes []Node) (milliCPU, memoryBytes int64, ok bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}

	discoveryMasterNodeMilliCPU := int64(hardLimitMilliCPU)
	discoveryMasterNodeMemory := int64(hardLimitMemory)

	for i := range nodes {
		effCPU, effMem := effectiveMasterResources(&nodes[i])
		if effCPU < discoveryMasterNodeMilliCPU {
			discoveryMasterNodeMilliCPU = effCPU
		}
		if effMem < discoveryMasterNodeMemory {
			discoveryMasterNodeMemory = effMem
		}
	}

	return discoveryMasterNodeMilliCPU - configEveryNodeMilliCPU,
		discoveryMasterNodeMemory - configEveryNodeMemory,
		true
}

func fallbackSplit(total, percent int64) int64 {
	return total * percent / 100
}
