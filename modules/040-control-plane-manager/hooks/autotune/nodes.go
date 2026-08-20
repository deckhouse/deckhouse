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

package autotune

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
	controlPlanePercent          = 35
	everyNodeReservationMilliCPU = 300
	everyNodeReservationMemory   = 512 * 1024 * 1024
	maxBudgetMilliCPU            = 4 * 1000
	maxBudgetMemory              = 8 * 1024 * 1024 * 1024

	// Subtracted from Capacity even when the kubelet reports no reservation yet: a
	// number that changes mid-bootstrap restarts the whole control plane.
	minKubeletReservationMilliCPU = 100
	minKubeletReservationMemory   = 900 * 1024 * 1024
)

type Node struct {
	Name                string
	CapacityMilliCPU    int64
	CapacityMemory      int64
	AllocatableMilliCPU int64
	AllocatableMemory   int64
}

func usableMasterResources(n *Node) (int64, int64) {
	cpuReservation := max(n.CapacityMilliCPU-n.AllocatableMilliCPU, minKubeletReservationMilliCPU)
	memReservation := max(n.CapacityMemory-n.AllocatableMemory, minKubeletReservationMemory)
	return n.CapacityMilliCPU - cpuReservation, n.CapacityMemory - memReservation
}

func weakestMasterBudget(nodes []Node) (int64, int64, bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}

	minCPU := int64(maxBudgetMilliCPU)
	minMemory := int64(maxBudgetMemory)
	for i := range nodes {
		usableCPU, usableMemory := usableMasterResources(&nodes[i])
		minCPU = min(minCPU, usableCPU)
		minMemory = min(minMemory, usableMemory)
	}

	return minCPU, minMemory, true
}

type otherPodRequests struct {
	MilliCPU    int64
	MemoryBytes int64
}

// No percent carve-out here: that one belongs to the static split alone.
func weakestMasterHeadroom(nodes []Node, otherRequests map[string]otherPodRequests) (int64, int64, bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}

	minCPU := int64(math.MaxInt64)
	minMemory := int64(math.MaxInt64)
	for i := range nodes {
		usableCPU, usableMemory := usableMasterResources(&nodes[i])
		other := otherRequests[nodes[i].Name]
		minCPU = min(minCPU, max(usableCPU-other.MilliCPU, 0))
		minMemory = min(minMemory, max(usableMemory-other.MemoryBytes, 0))
	}

	return minCPU, minMemory, true
}

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

// Callers add up app and init containers: scheduling uses max(init) + sum(app),
// and the stricter sum cannot under-estimate what the node reserves.
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

func readOtherPodRequests(ctx context.Context, dc dependency.Container, nodes []Node) (map[string]otherPodRequests, error) {
	out := make(map[string]otherPodRequests, len(nodes))
	for i := range nodes {
		name := nodes[i].Name
		if name == "" {
			continue
		}
		pods, err := listPodsOnNode(ctx, dc, name)
		if err != nil {
			return nil, err
		}

		var milliCPU, memoryBytes int64
		for j := range pods {
			pod := &pods[j]
			if isAutotunedControlPlanePod(pod) {
				continue
			}
			appCPU, appMemory := sumContainerRequests(pod.Spec.Containers)
			initCPU, initMemory := sumContainerRequests(pod.Spec.InitContainers)
			// The scheduler adds a RuntimeClass pod's sandbox overhead on top.
			overhead := pod.Spec.Overhead
			milliCPU += appCPU + initCPU + overhead.Cpu().MilliValue()
			memoryBytes += appMemory + initMemory + overhead.Memory().Value()
		}
		out[name] = otherPodRequests{MilliCPU: milliCPU, MemoryBytes: memoryBytes}
	}
	return out, nil
}

var notTerminatedPods = fields.AndSelectors(
	fields.OneTermNotEqualSelector("status.phase", string(v1.PodSucceeded)),
	fields.OneTermNotEqualSelector("status.phase", string(v1.PodFailed)),
)

// Overridable in unit tests: client-go fakes do not index spec.nodeName.
var listPodsOnNode = listPodsOnNodeFromAPI

func listPodsOnNodeFromAPI(ctx context.Context, dc dependency.Container, nodeName string) ([]v1.Pod, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}

	onThisNode := fields.AndSelectors(
		fields.OneTermEqualSelector("spec.nodeName", nodeName),
		notTerminatedPods,
	)
	list, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: onThisNode.String(),
		// From a cache rather than a quorum read, as in hooks/helm.go.
		ResourceVersion:      "0",
		ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}
	return list.Items, nil
}
