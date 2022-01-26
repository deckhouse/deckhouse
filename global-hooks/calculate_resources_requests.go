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
	controlPlanePercent      = 50                     // %
	hardLimitMilliCPU        = 4 * 1000               // 4 Cpu
	hardLimitMemory          = 8 * 1024 * 1024 * 1024 // 8G ram
	managedHardLimitMilliCPU = 1 * 1000               // 1 Cpu
	managedHardLimitMemory   = 1 * 1024 * 1024 * 1024 // 1G ram
)

type Node struct {
	allocatableMilliCPU int64
	allocatableMemory   int64
}

func applyNodesResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	n := &Node{}

	n.allocatableMilliCPU = node.Status.Allocatable.Cpu().MilliValue()
	n.allocatableMemory = node.Status.Allocatable.Memory().Value()

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
						Key:      "node-role.kubernetes.io/master",
						Operator: "Exists",
					},
				}},
				FilterFunc: applyNodesResourcesFilter,
			},
		},
	}, calculateResourcesRequests)
)

func calculateResourcesRequests(input *go_hook.HookInput) error {
	var (
		calculatedMasterNodeMilliCPU   int64
		calculatedMasterNodeMemory     int64
		calculatedControlPlaneMilliCPU int64
		calculatedControlPlaneMemory   int64

		discoveryMasterNodeMilliCPU int64
		discoveryMasterNodeMemory   int64

		configEveryNodeMilliCPU int64
		configEveryNodeMemory   int64

		isManagedCloud bool
	)
	snapshots := input.Snapshots["NodesResources"]
	if len(snapshots) > 0 {
		// Hardcoded maximum values for master node resources
		discoveryMasterNodeMilliCPU = hardLimitMilliCPU
		discoveryMasterNodeMemory = hardLimitMemory
		for _, snapshot := range snapshots {
			n := snapshot.(*Node)
			if n.allocatableMilliCPU < discoveryMasterNodeMilliCPU {
				discoveryMasterNodeMilliCPU = n.allocatableMilliCPU
			}
			if n.allocatableMemory < discoveryMasterNodeMemory {
				discoveryMasterNodeMemory = n.allocatableMemory
			}
		}
	} else {
		isManagedCloud = true
		// Hardcoded maximum values for master nodes in managed clouds (GKE for example)
		discoveryMasterNodeMilliCPU = managedHardLimitMilliCPU
		discoveryMasterNodeMemory = managedHardLimitMemory
	}

	path := "global.modules.resourcesRequests.everyNode.cpu"
	if !input.Values.Exists(path) {
		return fmt.Errorf("%s must be set", path)
	}
	quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
	if err != nil {
		return err
	}
	configEveryNodeMilliCPU = quantity.MilliValue()
	if configEveryNodeMilliCPU <= 0 {
		return fmt.Errorf("%s must be greater 0", path)
	}

	path = "global.modules.resourcesRequests.everyNode.memory"
	if !input.Values.Exists(path) {
		return fmt.Errorf("%s must be set", path)
	}
	quantity, err = getAndParseResourceQuantity(input.Values.Get(path))
	if err != nil {
		return err
	}
	configEveryNodeMemory = quantity.Value()
	if configEveryNodeMemory <= 0 {
		return fmt.Errorf("%s must be greater 0", path)
	}

	path = "global.modules.resourcesRequests.masterNode.cpu"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedMasterNodeMilliCPU = quantity.MilliValue()
	} else {
		calculatedMasterNodeMilliCPU = discoveryMasterNodeMilliCPU - configEveryNodeMilliCPU
	}

	if calculatedMasterNodeMilliCPU <= 0 {
		return fmt.Errorf("cpu resources for allocating on master nodes must be greater than 0 (masterNode CPU must be greater than 0 or discovered minimal master node CPU must be greater than everyNode CPU)")
	}

	path = "global.modules.resourcesRequests.masterNode.memory"
	if input.Values.Exists(path) {
		quantity, err := getAndParseResourceQuantity(input.Values.Get(path))
		if err != nil {
			return err
		}
		calculatedMasterNodeMemory = quantity.Value()
	} else {
		calculatedMasterNodeMemory = discoveryMasterNodeMemory - configEveryNodeMemory
	}

	if calculatedMasterNodeMemory <= 0 {
		return fmt.Errorf("memory resources for allocating on master nodes must be greater than 0 (masterNode memory must be greater than 0 or discovered minimal master node memory must be greater than everyNode memory)")
	}

	if calculatedMasterNodeMilliCPU+configEveryNodeMilliCPU > discoveryMasterNodeMilliCPU {
		return fmt.Errorf("everyNode CPU + masterNode CPU must be less than discovered minimal master node CPU")
	}

	if calculatedMasterNodeMemory+configEveryNodeMemory > discoveryMasterNodeMemory {
		return fmt.Errorf("everyNode memory + masterNode memory must be less than discovered minimal master node memory")
	}

	// if cloud isn't managed (GKE for example), extract some resources on master nodes for control-plane components
	if !isManagedCloud {
		calculatedControlPlaneMilliCPU = calculatedMasterNodeMilliCPU * controlPlanePercent / 100
		calculatedControlPlaneMemory = calculatedMasterNodeMemory * controlPlanePercent / 100
		calculatedMasterNodeMilliCPU = calculatedMasterNodeMilliCPU * (100 - controlPlanePercent) / 100
		calculatedMasterNodeMemory = calculatedMasterNodeMemory * (100 - controlPlanePercent) / 100
	}

	input.Values.Set("global.internal.modules.resourcesRequests.milliCpuEveryNode", configEveryNodeMilliCPU)
	input.Values.Set("global.internal.modules.resourcesRequests.memoryEveryNode", configEveryNodeMemory)
	input.Values.Set("global.internal.modules.resourcesRequests.milliCpuControlPlane", calculatedControlPlaneMilliCPU)
	input.Values.Set("global.internal.modules.resourcesRequests.memoryControlPlane", calculatedControlPlaneMemory)
	input.Values.Set("global.internal.modules.resourcesRequests.milliCpuMaster", calculatedMasterNodeMilliCPU)
	input.Values.Set("global.internal.modules.resourcesRequests.memoryMaster", calculatedMasterNodeMemory)

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
