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

// Internal values paths for combined and per-component resource requests.
const (
	pathMilliCPUControlPlane = "controlPlaneManager.internal.resourcesRequests.milliCpuControlPlane"
	pathMemoryControlPlane   = "controlPlaneManager.internal.resourcesRequests.memoryControlPlane"
	pathComponents           = "controlPlaneManager.internal.resourcesRequests.components"

	pathCPMCPU       = "controlPlaneManager.resourcesRequests.cpu"
	pathCPMMemory    = "controlPlaneManager.resourcesRequests.memory"
	pathGlobalCPU    = "global.modules.resourcesRequests.controlPlane.cpu"
	pathGlobalMemory = "global.modules.resourcesRequests.controlPlane.memory"
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

func fallbackSplit(total, percent int64) int64 {
	return total * percent / 100
}

func autotuneMetricNameFor(resourceName resourceKind) string {
	return "d8-cpm-autotune-" + string(resourceName)
}
