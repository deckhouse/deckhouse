// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	controlPlanePercent     = 40                     // %
	configEveryNodeMilliCPU = 300                    // 0.3 Cpu
	configEveryNodeMemory   = 512 * 1024 * 1024      // 512Mb
	hardLimitMilliCPU       = 4 * 1000               // 4 Cpu
	hardLimitMemory         = 8 * 1024 * 1024 * 1024 // 8G ram

	// it needs for prevent multiple restarts control-plane manager on cluster bootstrap
	// for 8x4 installations
	kubeletResourceReservationMemory = 900 * 1024 * 1024 // 900 mb
	kubeletResourceReservationCPU    = 100               // 0.1 cpu
)

type Node struct {
	AllocatableMilliCPU int64
	AllocatableMemory   int64
}

func applyNodesResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	n := &Node{}

	n.AllocatableMilliCPU = node.Status.Allocatable.Cpu().MilliValue()
	n.AllocatableMemory = node.Status.Allocatable.Memory().Value()

	return n, nil
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeAll: &go_hook.OrderedConfig{Order: 20},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "NodesResources",
				ApiVersion: "v1",
				Kind:       "Node",
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: metav1.LabelSelectorOpExists,
					},
				}},
				FilterFunc: applyNodesResourcesFilter,
			},
		},
	}, calculateResourcesRequests)
)

func calculateResourcesRequests(input *go_hook.HookInput) error {
	var (
		calculatedMasterNodeMilliCPU int64
		calculatedMasterNodeMemory   int64

		calculatedControlPlaneMilliCPU int64
		calculatedControlPlaneMemory   int64

		discoveryMasterNodeMilliCPU int64
		discoveryMasterNodeMemory   int64
	)
	snapshots := input.Snapshots["NodesResources"]

	// Managed cloud
	if len(snapshots) == 0 {
		return nil
	}

	// Hardcoded maximum values for master node resources
	discoveryMasterNodeMilliCPU = hardLimitMilliCPU
	discoveryMasterNodeMemory = hardLimitMemory
	for _, snapshot := range snapshots {
		n := snapshot.(*Node)
		if n.AllocatableMilliCPU < discoveryMasterNodeMilliCPU && absDiff(n.AllocatableMilliCPU, discoveryMasterNodeMilliCPU) > kubeletResourceReservationCPU {
			discoveryMasterNodeMilliCPU = n.AllocatableMilliCPU
		}

		if n.AllocatableMemory < discoveryMasterNodeMemory && absDiff(n.AllocatableMemory, discoveryMasterNodeMemory) > kubeletResourceReservationMemory {
			discoveryMasterNodeMemory = n.AllocatableMemory
		}
	}

	calculatedMasterNodeMilliCPU = discoveryMasterNodeMilliCPU - configEveryNodeMilliCPU
	calculatedMasterNodeMemory = discoveryMasterNodeMemory - configEveryNodeMemory

	if calculatedMasterNodeMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than %dm", configEveryNodeMilliCPU)
	}

	if calculatedMasterNodeMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than %dMi", configEveryNodeMemory/1024/1024)
	}

	calculatedControlPlaneMilliCPU = calculatedMasterNodeMilliCPU * controlPlanePercent / 100
	calculatedControlPlaneMemory = calculatedMasterNodeMemory * controlPlanePercent / 100

	path := "global.modules.resourcesRequests.controlPlane.cpu"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedControlPlaneMilliCPU = quantity.MilliValue()
	}

	path = "global.modules.resourcesRequests.controlPlane.memory"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedControlPlaneMemory = quantity.Value()
	}

	input.Values.Set("global.internal.modules.resourcesRequests.milliCpuControlPlane", calculatedControlPlaneMilliCPU)
	input.Values.Set("global.internal.modules.resourcesRequests.memoryControlPlane", calculatedControlPlaneMemory)

	return nil
}

func getAndParseResourceQuantity(input gjson.Result) (resource.Quantity, error) {
	var quantity resource.Quantity

	strVal := input.String()
	quantity, err := resource.ParseQuantity(strVal)
	if err != nil {
		return quantity, fmt.Errorf("cannot parse '%v': %v", strVal, err)
	}

	return quantity, nil
}

func absDiff(a, b int64) int64 {
	d := a - b
	if d > 0 {
		return d
	}
	return b - a
}
