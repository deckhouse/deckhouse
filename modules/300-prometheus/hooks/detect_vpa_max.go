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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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
	memoryPerPod   = int64(15) * 1024 * 1024                                 // in Megabytes
	milliCPUPerPod = int64(20)                                               // in milli cpu
	minMem         = resource.NewQuantity(1000*1024*1024, resource.BinarySI) // for reference see modules/300-prometheus/templates/prometheus/prometheus.yaml
	minCPU         = resource.NewMilliQuantity(200, resource.DecimalSI)      // for reference see modules/300-prometheus/templates/prometheus/prometheus.yaml
	minLongtermMem = resource.NewQuantity(500*1024*1024, resource.BinarySI)  // for reference see modules/300-prometheus/templates/prometheus/longterm/prometheus.yaml
	minLongtermCPU = resource.NewMilliQuantity(50, resource.DecimalSI)       // for reference see modules/300-prometheus/templates/prometheus/longterm/prometheus.yaml
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

func calculateNodesCapacity(_ context.Context, input *go_hook.HookInput) error {
	nodeSnap := input.Snapshots.Get("nodes")

	if len(nodeSnap) == 0 {
		return fmt.Errorf("no nodes snapshot found")
	}

	var totalPodsCapacity int64

	for node, err := range sdkobjectpatch.SnapshotIter[Node](nodeSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'nodes' snapshot: %v", err)
		}

		totalPodsCapacity += node.Capacity
	}

	totalPodsMemory := totalPodsCapacity * memoryPerPod
	totalPodsCPU := totalPodsCapacity * milliCPUPerPod

	// calculate prometheus usage
	maxMem := resource.NewQuantity(totalPodsMemory/1, resource.BinarySI)
	maxCPU := resource.NewMilliQuantity(totalPodsCPU/1, resource.DecimalSI)

	if maxMem.Cmp(*minMem) == -1 {
		maxMem = minMem
	}

	if maxCPU.Cmp(*minCPU) == -1 {
		maxCPU = minCPU
	}

	// calculate longterm prometheus usage
	maxLongtermMem := resource.NewQuantity(totalPodsMemory/3, resource.BinarySI)
	maxLongtermCPU := resource.NewMilliQuantity(totalPodsCPU/3, resource.DecimalSI)

	if maxLongtermMem.Cmp(*minLongtermMem) == -1 {
		maxLongtermMem = minLongtermMem
	}

	if maxLongtermCPU.Cmp(*minLongtermCPU) == -1 {
		maxLongtermCPU = minLongtermCPU
	}

	input.Values.Set("prometheus.internal.vpa.maxMemory", maxMem.String())
	input.Values.Set("prometheus.internal.vpa.maxCPU", maxCPU.String())

	input.Values.Set("prometheus.internal.vpa.longtermMaxMemory", maxLongtermMem.String())
	input.Values.Set("prometheus.internal.vpa.longtermMaxCPU", maxLongtermCPU.String())

	return nil
}
