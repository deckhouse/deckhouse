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

package resources

// How much of a master may be spent on the control-plane. Two different budgets
// are derived here and they are not interchangeable:
//
//   - minMasterNodeBudget — what the static percent split is carved out of.
//   - minMasterFitBudget — the headroom a measured raise has to fit into.

import (
	"context"
	"fmt"
	"math"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	controlPlanePercent     = 35                // %
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

type Node struct {
	Name                string
	CapacityMilliCPU    int64
	CapacityMemory      int64
	AllocatableMilliCPU int64
	AllocatableMemory   int64
}

// effectiveMasterResources returns the per-node usable CPU/memory budget the
// control-plane allocation can be carved out of. Computed from Node.Status.Capacity
// (immutable for the lifetime of the node) minus max(actual kubelet reservation,
// our floor). The result is stable across the kubelet warm-up window, so the
// hook output does not flip a few minutes into the bootstrap.
func effectiveMasterResources(n *Node) (int64, int64) {
	cpuReservation := max(n.CapacityMilliCPU-n.AllocatableMilliCPU, kubeletResourceReservationCPUFloor)
	memReservation := max(n.CapacityMemory-n.AllocatableMemory, kubeletResourceReservationMemoryFloor)
	return n.CapacityMilliCPU - cpuReservation, n.CapacityMemory - memReservation
}

// minMasterNodeBudget returns effectiveMasterResources of the weakest master,
// capped by the hard limits. The configEveryNode reservation and the
// controlPlanePercent carve-out are applied by the caller.
func minMasterNodeBudget(nodes []Node) (int64, int64, bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}

	discoveryMasterNodeMilliCPU := int64(hardLimitMilliCPU)
	discoveryMasterNodeMemory := int64(hardLimitMemory)

	for i := range nodes {
		effCPU, effMem := effectiveMasterResources(&nodes[i])
		discoveryMasterNodeMilliCPU = min(discoveryMasterNodeMilliCPU, effCPU)
		discoveryMasterNodeMemory = min(discoveryMasterNodeMemory, effMem)
	}

	return discoveryMasterNodeMilliCPU, discoveryMasterNodeMemory, true
}

// nodeOtherRequests is the sum of non-control-plane pod requests on one node.
type nodeOtherRequests struct {
	MilliCPU    int64
	MemoryBytes int64
}

// minMasterFitBudget is the tightest free capacity across masters for fitting
// proposed control-plane requests:
//
//	effectiveMasterResources(node) − sum(requests of non-control-plane pods on node)
//
// Returns millicpu and bytes. Unlike minMasterNodeBudget it does not apply
// the configEveryNode / percent carve-out — those are for the static split.
func minMasterFitBudget(nodes []Node, otherByNode map[string]nodeOtherRequests) (int64, int64, bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}
	minCPU := int64(math.MaxInt64)
	minMemBytes := int64(math.MaxInt64)
	for i := range nodes {
		effCPU, effMem := effectiveMasterResources(&nodes[i])
		other := otherByNode[nodes[i].Name]
		minCPU = min(minCPU, max(effCPU-other.MilliCPU, 0))
		minMemBytes = min(minMemBytes, max(effMem-other.MemoryBytes, 0))
	}
	return minCPU, minMemBytes, true
}

// isAutotunedControlPlanePod reports whether the pod is one of the static pods
// whose requests are managed by this package. Other tier=control-plane pods
// (e.g. component=kube-api-proxy) still consume capacity and count as "other".
func isAutotunedControlPlanePod(pod *v1.Pod) bool {
	if pod.Labels["tier"] != "control-plane" {
		return false
	}
	for _, comp := range controlPlaneComponents {
		if componentMeta[comp].container == pod.Labels["component"] {
			return true
		}
	}
	return false
}

// sumContainerRequests is called for app and init containers separately, and the
// two results are summed. Scheduling uses max(init) + sum(app); summing both is a
// slightly stricter upper bound and avoids under-estimating reserved capacity.
func sumContainerRequests(containers []v1.Container) (int64, int64) {
	var milliCPU, memoryBytes int64
	for i := range containers {
		req := containers[i].Resources.Requests
		if req == nil {
			continue
		}
		milliCPU += req.Cpu().MilliValue()
		memoryBytes += req.Memory().Value()
	}
	return milliCPU, memoryBytes
}

func fetchOtherRequestsByMasterNodes(ctx context.Context, dc dependency.Container, nodes []Node) (map[string]nodeOtherRequests, error) {
	out := make(map[string]nodeOtherRequests, len(nodes))
	for i := range nodes {
		name := nodes[i].Name
		if name == "" {
			continue
		}
		items, err := listPodsOnNode(ctx, dc, name)
		if err != nil {
			return nil, err
		}
		var milliCPU, memoryBytes int64
		for j := range items {
			pod := &items[j]
			if isAutotunedControlPlanePod(pod) {
				continue
			}
			cpu, mem := sumContainerRequests(pod.Spec.Containers)
			initCPU, initMem := sumContainerRequests(pod.Spec.InitContainers)
			milliCPU += cpu + initCPU
			memoryBytes += mem + initMem
		}
		out[name] = nodeOtherRequests{MilliCPU: milliCPU, MemoryBytes: memoryBytes}
	}
	return out, nil
}

// listPodsOnNode lists non-terminated pods scheduled on nodeName.
// Overridable in unit tests (client-go fakes do not index spec.nodeName).
var listPodsOnNode = listPodsOnNodeFromAPI

func listPodsOnNodeFromAPI(ctx context.Context, dc dependency.Container, nodeName string) ([]v1.Pod, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}
	fieldSelector := fields.AndSelectors(
		fields.OneTermEqualSelector("spec.nodeName", nodeName),
		fields.OneTermNotEqualSelector("status.phase", string(v1.PodSucceeded)),
		fields.OneTermNotEqualSelector("status.phase", string(v1.PodFailed)),
	).String()
	list, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}
	return list.Items, nil
}
