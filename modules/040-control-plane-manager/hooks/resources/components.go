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

// Package resources keeps the control-plane resource requests of the four
// static-pod components in step with the masters they run on.
//
// Two hooks: hook_autotune.go resolves the requests and owns the state
// ConfigMap, hook_sync.go projects that ConfigMap into values for the templates.
// resolve*.go holds the resolution chain the first one runs.
package resources

// Namespace of the control-plane static pods, and of this package's own state
// ConfigMap.
const kubeSystemNS = "kube-system"

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
const pathComponents = "controlPlaneManager.internal.resourcesRequests.components"

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
