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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// resourceKind identifies a control-plane measurement (cpu or memory).
// String values match ConfigMap JSON, metrics labels, and PodMetric names.
type resourceKind string

const (
	resourceCPU    resourceKind = "cpu"
	resourceMemory resourceKind = "memory"
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
)

// Internal values path for per-component resource requests.
const (
	pathComponents = "controlPlaneManager.internal.resourcesRequests.components"
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

// componentFallbackPercent is the fixed %-split of a combined budget
// (ModuleConfig override or legacy discovery). apiserver/etcd/kcm/scheduler =
// 45/35/10/10.
var componentFallbackPercent = map[string]int64{
	componentKubeApiserver:         45,
	componentEtcd:                  35,
	componentKubeControllerManager: 10,
	componentKubeScheduler:         10,
}

func fallbackSplit(total, percent int64) int64 {
	return total * percent / 100
}

const (
	obsoleteGlobalResourcesRequestsMetricName  = "d8_obsolete_global_control_plane_resources_requests"
	obsoleteGlobalResourcesRequestsMetricGroup = "D8ObsoleteGlobalControlPlaneResourcesRequests"
)

// controlPlaneNodesBinding watches master Nodes (shared by Hook A).
func controlPlaneNodesBinding(onSync, onEvents bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       "NodesResources",
		ApiVersion: "v1",
		Kind:       "Node",
		LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpExists,
			},
		}},
		FilterFunc:                   applyNodesResourcesFilter,
		ExecuteHookOnEvents:          ptr.To(onEvents),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

const (
	controlPlanePercent     = 35                // %
	configEveryNodeMilliCPU = 300               // 0.3 Cpu
	configEveryNodeMemory   = 512 * 1024 * 1024 // 512Mb
	hardLimitMilliCPU       = 4 * 1000          // 4 Cpu
	hardLimitMemory         = 8 * 1024 * 1024 * 1024

	// Asymmetric deadband for autotune raise/lower. Go constants, not config-values.
	// Change is significant only when delta > raiseThreshold or
	// delta < -lowerThreshold (delta = (rec - applied) / applied).
	raiseThreshold = 0.20 // +20%
	lowerThreshold = 0.30 // −30%

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

func applyNodesResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	n := &Node{
		Name:                node.GetName(),
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
func minMasterNodeBudget(nodes []Node) (int64, int64, bool) {
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

// nodeOtherRequests is the sum of non-control-plane pod requests on one node.
type nodeOtherRequests struct {
	MilliCPU    int64
	MemoryBytes int64
}

// autotunedControlPlaneComponents are static-pod component labels whose requests
// are managed by resources autotune. Other tier=control-plane pods (e.g.
// component=kube-api-proxy) still consume capacity and count as "other".
var autotunedControlPlaneComponents = map[string]struct{}{
	containerKubeApiserver:         {},
	containerEtcd:                  {},
	containerKubeControllerManager: {},
	containerKubeScheduler:         {},
}

func isAutotunedControlPlanePod(pod *v1.Pod) bool {
	if pod.Labels["tier"] != "control-plane" {
		return false
	}
	_, ok := autotunedControlPlaneComponents[pod.Labels["component"]]
	return ok
}

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

func otherRequestsFromPod(pod *v1.Pod) (int64, int64) {
	cpu, mem := sumContainerRequests(pod.Spec.Containers)
	initCPU, initMem := sumContainerRequests(pod.Spec.InitContainers)
	// Scheduling uses max(init) + sum(app); summing both is a slightly stricter
	// upper bound and avoids under-estimating reserved capacity.
	return cpu + initCPU, mem + initMem
}

// minMasterFitBudget is the tightest free capacity across masters for fitting
// proposed control-plane requests:
//
//	effectiveMasterResources(node) − sum(requests of non-control-plane pods on node)
//
// Returns millicpu and bytes. Unlike minMasterNodeBudget it does not apply
// the configEveryNode / 40% carve-out — those are for combined-budget discovery.
func minMasterFitBudget(nodes []Node, otherByNode map[string]nodeOtherRequests) (int64, int64, bool) {
	if len(nodes) == 0 {
		return 0, 0, false
	}
	minCPU := int64(-1)
	minMemBytes := int64(-1)
	for i := range nodes {
		effCPU, effMem := effectiveMasterResources(&nodes[i])
		other := otherByNode[nodes[i].Name]
		availCPU := effCPU - other.MilliCPU
		availMem := effMem - other.MemoryBytes
		if availCPU < 0 {
			availCPU = 0
		}
		if availMem < 0 {
			availMem = 0
		}
		if minCPU < 0 || availCPU < minCPU {
			minCPU = availCPU
		}
		if minMemBytes < 0 || availMem < minMemBytes {
			minMemBytes = availMem
		}
	}
	return minCPU, minMemBytes, true
}

// controlPlaneAutotuneActive is true when prometheus and prometheus-metrics-adapter
// are enabled — the custom.metrics.k8s.io path used by per-component autotune.
func controlPlaneAutotuneActive(input *go_hook.HookInput) bool {
	enabled := set.NewFromValues(input.Values, "global.enabledModules")
	return enabled.Has("prometheus") && enabled.Has("prometheus-metrics-adapter")
}

type resolvedCombinedBudget struct {
	MilliCPU         int64
	MemoryBytes      int64
	CPUFromConfig    bool
	MemoryFromConfig bool
}
