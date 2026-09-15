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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	standbyStatusField = "standby"
)

type StandbyNodeGroupInfo struct {
	Name                 string
	NeedStandby          bool
	CurrentStandby       *int
	MaxPerZone           int
	ZonesCount           int
	Standby              *intstr.IntOrString
	OverprovisioningRate int64
	Taints               []v1.Taint
}

// Fields other than Name, NeedStandby and CurrentStandby are left zero for NodeGroups without
// standby, so that unrelated spec changes do not alter the filter result checksum and wake the hook.
func standbyNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeGroup := new(ngv1.NodeGroup)
	if err := sdk.FromUnstructured(obj, nodeGroup); err != nil {
		return nil, fmt.Errorf("convert nodegroup from unstructured: %w", err)
	}

	currentStandby, err := standbyStatus(obj, nodeGroup)
	if err != nil {
		return nil, err
	}

	if !needStandby(nodeGroup) {
		return StandbyNodeGroupInfo{
			Name:           nodeGroup.GetName(),
			NeedStandby:    false,
			CurrentStandby: currentStandby,
		}, nil
	}

	var zonesCount int
	if nodeGroup.Spec.CloudInstances.Zones != nil {
		zonesCount = len(nodeGroup.Spec.CloudInstances.Zones)
	}

	taints := nodeGroup.Spec.NodeTemplate.Taints
	if len(taints) == 0 {
		taints = make([]v1.Taint, 0)
	}

	overprovisioningRate := int64(50) // default: 50%
	if nodeGroup.Spec.CloudInstances.StandbyHolder.OverprovisioningRate != nil {
		overprovisioningRate = *nodeGroup.Spec.CloudInstances.StandbyHolder.OverprovisioningRate
	}

	return StandbyNodeGroupInfo{
		Name:                 nodeGroup.GetName(),
		NeedStandby:          true,
		CurrentStandby:       currentStandby,
		MaxPerZone:           int(*nodeGroup.Spec.CloudInstances.MaxPerZone),
		ZonesCount:           zonesCount,
		Standby:              nodeGroup.Spec.CloudInstances.Standby,
		OverprovisioningRate: overprovisioningRate,
		Taints:               taints,
	}, nil
}

func needStandby(nodeGroup *ngv1.NodeGroup) bool {
	if nodeGroup.Spec.NodeType != ngv1.NodeTypeCloudEphemeral {
		return false
	}
	if nodeGroup.Spec.CloudInstances.Standby == nil {
		return false
	}
	if nodeGroup.Spec.CloudInstances.Standby.String() == "0" {
		return false
	}

	// No nil-checking for MaxPerZone and MinPerZone pointers as these fields are mandatory for CloudEphemeral NGs.
	return *nodeGroup.Spec.CloudInstances.MinPerZone != *nodeGroup.Spec.CloudInstances.MaxPerZone
}

// The field is read through unstructured only to tell an absent status.standby from an explicit zero,
// the value itself is already decoded into the typed NodeGroup.
func standbyStatus(obj *unstructured.Unstructured, nodeGroup *ngv1.NodeGroup) (*int, error) {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", standbyStatusField)
	if err != nil {
		return nil, fmt.Errorf("read status.%s of nodegroup %s: %w", standbyStatusField, nodeGroup.GetName(), err)
	}
	if !found || value == nil {
		return nil, nil
	}

	standby := int(nodeGroup.Status.Standby)
	return &standby, nil
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

func discoverStandbyNGHandler(_ context.Context, input *go_hook.HookInput) error {
	standbyNodeGroups := make([]StandbyNodeGroupForValues, 0)
	for nodeGroup, err := range sdkobjectpatch.SnapshotIter[StandbyNodeGroupInfo](input.Snapshots.Get("node_groups")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'node_groups' snapshot: %w", err)
		}

		if !nodeGroup.NeedStandby {
			if nodeGroup.CurrentStandby != nil {
				setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, standbyStatusField, nil)
			}
			continue
		}

		actualStandby := 0
		for standbyPod, err := range sdkobjectpatch.SnapshotIter[StandbyPodInfo](input.Snapshots.Get("standby_pods")) {
			if err != nil {
				return fmt.Errorf("cannot iterate over 'standby_pods' snapshot: %w", err)
			}

			if standbyPod.Group == nodeGroup.Name && standbyPod.IsReady {
				actualStandby++
			}
		}
		if nodeGroup.CurrentStandby == nil || *nodeGroup.CurrentStandby != actualStandby {
			setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, standbyStatusField, &actualStandby)
		}

		readyNodesCount := 0
		var (
			latestNodeTimestamp   *metav1.Time
			nodeAllocatableCPU    = resource.MustParse("4000m")
			nodeAllocatableMemory = resource.MustParse("8Gi")
		)
		for standbyNode, err := range sdkobjectpatch.SnapshotIter[StandbyNodeInfo](input.Snapshots.Get("nodes")) {
			if err != nil {
				return fmt.Errorf("cannot iterate over 'nodes' snapshot: %w", err)
			}

			if standbyNode.Group != nodeGroup.Name {
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

		if nodeGroup.ZonesCount == 0 {
			if zones, ok := input.Values.GetOk("nodeManager.internal.cloudProvider.zones"); ok {
				nodeGroup.ZonesCount = len(zones.Array())
			} else {
				nodeGroup.ZonesCount = 1
			}
		}
		maxInstances := nodeGroup.MaxPerZone * nodeGroup.ZonesCount

		desiredStandby := intOrPercent(nodeGroup.Standby, maxInstances)
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
		standbyRequestCPU := resource.NewScaledQuantity(nodeAllocatableCPU.ScaledValue(resource.Milli)/100*nodeGroup.OverprovisioningRate, resource.Milli)
		standbyRequestMemory := resource.NewScaledQuantity(nodeAllocatableMemory.ScaledValue(resource.Milli)/100*nodeGroup.OverprovisioningRate, resource.Milli)
		// Convert memory to Mi and format as a string. 1 Mi = 1024 * 1024 bytes.
		reserveMemoryInMi := standbyRequestMemory.Value() / (1024 * 1024)
		reserveMemoryMi := fmt.Sprintf("%dMi", reserveMemoryInMi)

		standbyNodeGroups = append(standbyNodeGroups, StandbyNodeGroupForValues{
			Name:          nodeGroup.Name,
			Standby:       desiredStandby,
			ReserveCPU:    standbyRequestCPU.String(),
			ReserveMemory: reserveMemoryMi,
			Taints:        nodeGroup.Taints,
		})
	}

	input.Values.Set("nodeManager.internal.standbyNodeGroups", standbyNodeGroups)
	return nil
}

var NumPercentRegex = regexp.MustCompile(`^([0-9]+)%$`)

func intOrPercent(val *intstr.IntOrString, maxValue int) int {
	matches := NumPercentRegex.FindStringSubmatch(val.StrVal)
	if len(matches) > 1 {
		percent, _ := strconv.Atoi(matches[1])
		return int(float64(maxValue) * float64(percent) / 100.0)
	}

	return val.IntValue()
}
