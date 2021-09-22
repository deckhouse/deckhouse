/*
Copyright 2021 Flant JSC

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
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Node struct {
	Capacity int64
}

func filterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := new(v1.Node)

	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes obj to Node: %v", err)
	}

	return &Node{Capacity: node.Status.Capacity.Pods().Value()}, nil
}

var (
	onEventExec    = true
	memoryPerPod   = int64(15) * 1024 * 1024 // in Megabytes
	milliCPUPerPod = int64(20)               // in milli cpu
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/detect_vpa_max",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "detect_vpa_max",
			Crontab: "*/10 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "nodes",
			ApiVersion:          "v1",
			Kind:                "Node",
			ExecuteHookOnEvents: &onEventExec,
			FilterFunc:          filterNode,
		},
	},
}, calculateNodesCapacity)

func calculateNodesCapacity(input *go_hook.HookInput) error {
	nodeSnap, ok := input.Snapshots["nodes"]
	if !ok {
		return errors.New("no nodes snapshot found")
	}

	var totalPodsCapacity int64

	for _, nodeS := range nodeSnap {
		node := nodeS.(*Node)
		totalPodsCapacity += node.Capacity
	}

	totalPodsMemory := totalPodsCapacity * memoryPerPod
	totalPodsCPU := totalPodsCapacity * milliCPUPerPod

	// calculate prometheus usage
	maxMem := resource.NewQuantity(totalPodsMemory/1, resource.BinarySI)
	maxCPU := resource.NewMilliQuantity(totalPodsCPU/1, resource.DecimalSI)

	// calculate longterm prometheus
	maxLongtermMem := resource.NewQuantity(totalPodsMemory/3, resource.BinarySI)
	maxLongtermCPU := resource.NewMilliQuantity(totalPodsCPU/3, resource.DecimalSI)

	input.Values.Set("prometheus.internal.vpa.maxMemory", maxMem.String())
	input.Values.Set("prometheus.internal.vpa.maxCPU", maxCPU.String())

	input.Values.Set("prometheus.internal.vpa.longtermMaxMemory", maxLongtermMem.String())
	input.Values.Set("prometheus.internal.vpa.longtermMaxCPU", maxLongtermCPU.String())

	return nil
}
