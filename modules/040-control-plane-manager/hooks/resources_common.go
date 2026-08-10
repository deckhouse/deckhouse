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
	"math"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
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
const (
	componentKubeApiserver         = "kubeApiserver"
	componentEtcd                  = "etcd"
	componentKubeControllerManager = "kubeControllerManager"
	componentKubeScheduler         = "kubeScheduler"
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

// componentMeta maps an internal component key to its static-pod container name
// (matches PodMetric selectors) and its fixed %-share of a combined budget
// (ModuleConfig override or legacy discovery): 45/35/10/10.
var componentMeta = map[string]struct {
	container string
	percent   int64
}{
	componentKubeApiserver:         {"kube-apiserver", 45},
	componentEtcd:                  {"etcd", 35},
	componentKubeControllerManager: {"kube-controller-manager", 10},
	componentKubeScheduler:         {"kube-scheduler", 10},
}

func fallbackSplit(total, percent int64) int64 {
	return total * percent / 100
}

const (
	obsoleteGlobalResourcesRequestsMetricName  = "d8_obsolete_global_control_plane_resources_requests"
	obsoleteGlobalResourcesRequestsMetricGroup = "D8ObsoleteGlobalControlPlaneResourcesRequests"
)

const (
	// 20h and not 24h so that the 03:00 cron run is never skipped because the
	// previous run happened a few minutes early or the scheduler drifted.
	metricsRunInterval = 20 * time.Hour

	// Each group expires independently, hence a separate group per hook.
	autotuneDegradedMetricName      = "d8_control_plane_manager_resources_autotune_degraded"
	autotuneDegradedMetricGroup     = "D8ControlPlaneResourcesAutotuneDegraded"
	autotuneSyncDegradedMetricGroup = "D8ControlPlaneResourcesAutotuneSyncDegraded"

	autotuneIncompleteMetricName  = "d8_control_plane_manager_resources_autotune_state_incomplete"
	autotuneIncompleteMetricGroup = "D8ControlPlaneResourcesAutotuneStateIncomplete"
)

// Reasons for autotuneDegradedMetricName.
const (
	degradedReasonBadNodes      = "bad_nodes"
	degradedReasonBadState      = "bad_state"
	degradedReasonBadOverride   = "bad_override"
	degradedReasonNodesTooSmall = "nodes_too_small"
	degradedReasonListPods      = "list_pods"
	degradedReasonReadThrough   = "read_through"
)

func setAutotuneDegraded(input *go_hook.HookInput, group, reason string) {
	input.MetricsCollector.Set(
		autotuneDegradedMetricName,
		1,
		map[string]string{"reason": reason},
		metrics.WithGroup(group),
	)
}

func countApplied(m *autotuneMeasurementState, kind resourceKind) int {
	if m == nil || m.Components == nil {
		return 0
	}
	n := 0
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], kind) > 0 {
			n++
		}
	}
	return n
}

func measurementIsComplete(m *autotuneMeasurementState, kind resourceKind) bool {
	return countApplied(m, kind) == len(controlPlaneComponents)
}

// metricsRunDue keeps event- and sync-driven runs from touching raise/lower
// outside the daily window.
func metricsRunDue(m *autotuneMeasurementState, now time.Time) bool {
	if m == nil || m.LastMetricsRun == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, m.LastMetricsRun)
	return err != nil || now.Sub(t) >= metricsRunInterval
}

// LastChange is touched only on an actual change: rewriting it every run would
// keep it permanently "just now" and block the next raise/lower on cooldown.
func setAppliedIfChanged(m *autotuneMeasurementState, comp string, kind resourceKind, val int64, ts string) bool {
	cs := m.Components[comp]
	switch kind {
	case resourceCPU:
		if cs.AppliedMilliCPU != nil && *cs.AppliedMilliCPU == val {
			return false
		}
	case resourceMemory:
		if cs.AppliedBytes != nil && *cs.AppliedBytes == val {
			return false
		}
	}
	setApplied(&cs, kind, val)
	cs.LastChange = ts
	m.Components[comp] = cs
	return true
}

// controlPlaneNodesBinding watches master Nodes.
func controlPlaneNodesBinding() go_hook.KubernetesConfig {
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
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(true),
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

// isAutotunedControlPlanePod reports whether the pod is one of the static pods
// whose requests are managed by resources autotune. Other tier=control-plane
// pods (e.g. component=kube-api-proxy) still consume capacity and count as
// "other".
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

// controlPlaneAutotuneActive is true when prometheus and prometheus-metrics-adapter
// are enabled — the custom.metrics.k8s.io path used by per-component autotune.
func controlPlaneAutotuneActive(input *go_hook.HookInput) bool {
	enabled := set.NewFromValues(input.Values, "global.enabledModules")
	return enabled.Has("prometheus") && enabled.Has("prometheus-metrics-adapter")
}
