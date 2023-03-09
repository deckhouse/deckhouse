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
	"fmt"
	"regexp"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	standbyStatusField = "standby"
)

type StandbyNodeGroupInfo struct {
	Name                 string
	NeedStandby          bool
	MaxPerZone           int
	ZonesCount           int
	Standby              *intstr.IntOrString
	OverprovisioningRate int64
	Taints               []v1.Taint
}

func standbyNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeGroup := new(ngv1.NodeGroup)
	err := sdk.FromUnstructured(obj, nodeGroup)
	if err != nil {
		return nil, err
	}

	var zonesCount int
	if nodeGroup.Spec.CloudInstances.Zones != nil {
		zonesCount = len(nodeGroup.Spec.CloudInstances.Zones)
	}

	taints := nodeGroup.Spec.NodeTemplate.Taints
	if len(taints) == 0 {
		taints = make([]v1.Taint, 0)
	}

	needStandby := false
	maxPerZone := 0
	overprovisioningRate := int64(50) // default: 50%

	if nodeGroup.Spec.NodeType == ngv1.NodeTypeCloudEphemeral {
		// No nil-checking for MaxPerZone and MinPerZone pointers as these fields are mandatory for CloudEphemeral NGs.
		maxPerZone = int(*nodeGroup.Spec.CloudInstances.MaxPerZone)
		if nodeGroup.Spec.CloudInstances.Standby != nil {
			if nodeGroup.Spec.CloudInstances.Standby.String() != "0" {
				if int(*nodeGroup.Spec.CloudInstances.MinPerZone) != int(*nodeGroup.Spec.CloudInstances.MaxPerZone) {
					needStandby = true
				}
			}
		}

		if nodeGroup.Spec.CloudInstances.StandbyHolder.OverprovisioningRate != nil {
			overprovisioningRate = *nodeGroup.Spec.CloudInstances.StandbyHolder.OverprovisioningRate
		}
	}

	return StandbyNodeGroupInfo{
		Name:                 nodeGroup.GetName(),
		NeedStandby:          needStandby,
		MaxPerZone:           maxPerZone,
		ZonesCount:           zonesCount,
		Standby:              nodeGroup.Spec.CloudInstances.Standby,
		OverprovisioningRate: overprovisioningRate,
		Taints:               taints,
	}, nil
}

type StandbyNodeInfo struct {
	Group             string
	AllocatableCPU    *resource.Quantity
	AllocatableMemory *resource.Quantity
	IsReady           bool
	IsUnschedulable   bool
	CreationTimestamp metav1.Time
}

func standbyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := new(v1.Node)
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	isReady := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			isReady = true
			break
		}
	}

	// .status.allocatable represents all available resources, not the only remaining.
	return StandbyNodeInfo{
		Group:             node.GetLabels()["node.deckhouse.io/group"],
		AllocatableCPU:    node.Status.Allocatable.Cpu(),
		AllocatableMemory: node.Status.Allocatable.Memory(),
		IsReady:           isReady,
		IsUnschedulable:   node.Spec.Unschedulable,
		CreationTimestamp: node.CreationTimestamp,
	}, nil
}

type StandbyPodInfo struct {
	Group   string
	IsReady bool
}

func standbyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := new(v1.Pod)
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, err
	}

	isReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			isReady = true
			break
		}
	}

	return StandbyPodInfo{
		Group:   pod.GetLabels()["ng"],
		IsReady: isReady,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/discover_standby_ng",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: standbyNodeGroupFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: standbyNodeFilter,
		},
		{
			Name:       "standby_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"standby-holder",
						},
					},
				},
			},
			FilterFunc: standbyPodFilter,
		},
	},
}, discoverStandbyNGHandler)

type StandbyNodeGroupForValues struct {
	Name          string     `json:"name"`
	Standby       int        `json:"standby"`
	ReserveCPU    string     `json:"reserveCPU"`
	ReserveMemory string     `json:"reserveMemory"`
	Taints        []v1.Taint `json:"taints"`
}

func discoverStandbyNGHandler(input *go_hook.HookInput) error {
	standbyNodeGroups := make([]StandbyNodeGroupForValues, 0)

	for _, node := range input.Snapshots["node_groups"] {
		ng := node.(StandbyNodeGroupInfo)

		if !ng.NeedStandby {
			setNodeGroupStatus(input.PatchCollector, ng.Name, standbyStatusField, nil)
			continue
		}

		actualStandby := 0
		for _, pod := range input.Snapshots["standby_pods"] {
			standbyPod := pod.(StandbyPodInfo)
			if standbyPod.Group == ng.Name && standbyPod.IsReady {
				actualStandby++
			}
		}
		setNodeGroupStatus(input.PatchCollector, ng.Name, standbyStatusField, &actualStandby)

		readyNodesCount := 0
		var (
			latestNodeTimestamp   *metav1.Time
			nodeAllocatableCPU    = resource.MustParse("4000m")
			nodeAllocatableMemory = resource.MustParse("8Gi")
		)

		for _, node := range input.Snapshots["nodes"] {
			standbyNode := node.(StandbyNodeInfo)
			if standbyNode.Group != ng.Name {
				continue
			}
			if standbyNode.IsReady && !standbyNode.IsUnschedulable {
				readyNodesCount++
			}

			// get resources from the latest created node. handle case when nodes are reordered with a new instance class
			if latestNodeTimestamp != nil && !latestNodeTimestamp.Before(&standbyNode.CreationTimestamp) {
				continue
			}

			if standbyNode.AllocatableCPU != nil {
				nodeAllocatableCPU = *standbyNode.AllocatableCPU
			}
			if standbyNode.AllocatableMemory != nil {
				nodeAllocatableMemory = *standbyNode.AllocatableMemory
			}
		}

		if ng.ZonesCount == 0 {
			if zones, ok := input.Values.GetOk("nodeManager.internal.cloudProvider.zones"); ok {
				ng.ZonesCount = len(zones.Array())
			} else {
				ng.ZonesCount = 1
			}
		}
		maxInstances := ng.MaxPerZone * ng.ZonesCount

		desiredStandby := intOrPercent(ng.Standby, maxInstances)
		totalNodesCount := readyNodesCount + desiredStandby - actualStandby
		if totalNodesCount > maxInstances {
			excessNodesCount := totalNodesCount - maxInstances
			desiredStandby -= excessNodesCount
		}

		// Always keep one Pending standby Pod to catch a Node on application scale down.
		if desiredStandby <= 0 {
			desiredStandby = 1
		}

		// calculate standby request as percent of the node
		standbyRequestCPU := resource.NewScaledQuantity(nodeAllocatableCPU.ScaledValue(resource.Milli)/100*ng.OverprovisioningRate, resource.Milli)
		standbyRequestMemory := resource.NewScaledQuantity(nodeAllocatableMemory.ScaledValue(resource.Milli)/100*ng.OverprovisioningRate, resource.Milli)

		standbyNodeGroups = append(standbyNodeGroups, StandbyNodeGroupForValues{
			Name:          ng.Name,
			Standby:       desiredStandby,
			ReserveCPU:    standbyRequestCPU.String(),
			ReserveMemory: fmt.Sprintf("%dMi", standbyRequestMemory.ScaledValue(resource.Mega)),
			Taints:        ng.Taints,
		})
	}

	input.Values.Set("nodeManager.internal.standbyNodeGroups", standbyNodeGroups)
	return nil
}

var NumPercentRegex = regexp.MustCompile(`^([0-9]+)%$`)

func intOrPercent(val *intstr.IntOrString, max int) int {
	matches := NumPercentRegex.FindStringSubmatch(val.StrVal)
	if len(matches) > 1 {
		percent, _ := strconv.Atoi(matches[1])
		return int(float64(max) * float64(percent) / 100.0)
	}

	return val.IntValue()
}
