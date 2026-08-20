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

// Package autotune keeps the resource requests of the four control-plane static
// pods in step with the masters they run on. hook_autotune.go resolves them and
// owns the state ConfigMap; hook_sync.go projects that ConfigMap into values.
package autotune

const kubeSystemNS = "kube-system"

// Values match the ConfigMap JSON, the metrics labels and the PodMetric names.
type resourceKind string

const (
	resourceCPU    resourceKind = "cpu"
	resourceMemory resourceKind = "memory"
)

const (
	componentKubeApiserver         = "kubeApiserver"
	componentEtcd                  = "etcd"
	componentKubeControllerManager = "kubeControllerManager"
	componentKubeScheduler         = "kubeScheduler"
)

const componentsValuesPath = "controlPlaneManager.internal.resourcesRequests.components"

var controlPlaneComponents = []string{
	componentKubeApiserver,
	componentEtcd,
	componentKubeControllerManager,
	componentKubeScheduler,
}

type componentInfo struct {
	container string
	percent   int64
}

var componentMeta = map[string]componentInfo{
	componentKubeApiserver:         {"kube-apiserver", 45},
	componentEtcd:                  {"etcd", 35},
	componentKubeControllerManager: {"kube-controller-manager", 10},
	componentKubeScheduler:         {"kube-scheduler", 10},
}

func percentOf(total, percent int64) int64 {
	return total * percent / 100
}
