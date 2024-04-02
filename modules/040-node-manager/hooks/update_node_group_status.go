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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimtypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
	capiv1beta1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/conditions"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/shared"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

// cache for event messages to avoid event spamming
// it's much harder to increment counter for existing event
var ngStatusCache = make(map[string]string)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/node-manager/update_ngs_statuses",
	Kubernetes: []go_hook.KubernetesConfig{
		// snapshot: "configuration_checksums_secret"
		// api: "v1",
		// kind: "Secret",
		// ns: "d8-cloud-instance-manager"
		// name: "configuration-checksums"
		shared.ConfigurationChecksumHookConfig(),
		{
			Name:                   "ngs",
			Kind:                   "NodeGroup",
			ApiVersion:             "deckhouse.io/v1",
			WaitForSynchronization: pointer.Bool(false),
			FilterFunc:             updStatusFilterNodeGroup,
		},
		{
			Name:                   "zones_count",
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "v1",
			Kind:                   "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-node-manager-cloud-provider"},
			},
			FilterFunc: updStatusFilterCpSecrets,
		},
		{
			Name:                   "mds",
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "machine.sapcloud.io/v1alpha1",
			Kind:                   "MachineDeployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: updStatusFilterMD,
		},
		{
			Name:                   "instances",
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "machine.sapcloud.io/v1alpha1",
			Kind:                   "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: updStatusFilterMachine,
		},
		{
			Name:                   "capi_instances",
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "cluster.x-k8s.io/v1beta1",
			Kind:                   "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: updStatusFilterCapiMachine,
		},
		{
			Name:                   "nodes",
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "v1",
			Kind:                   "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: updStatusFilterNode,
		},
	},
}, handleUpdateNGStatus)

func updStatusFilterMD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var md v1alpha1.MachineDeployment

	err := sdk.FromUnstructured(obj, &md)
	if err != nil {
		return nil, err
	}

	var frozen bool
	for _, c := range md.Status.Conditions {
		if c.Type == v1alpha1.MachineDeploymentFrozen {
			frozen = c.Status == v1alpha1.ConditionTrue
			break
		}
	}

	return statusMachineDeployment{
		Name:                md.Name,
		Replicas:            md.Spec.Replicas,
		IsFrozen:            frozen,
		NodeGroup:           md.Labels["node-group"],
		LastMachineFailures: md.Status.FailedMachines,
	}, nil
}

func updStatusFilterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}
	cloudInstanceGroup := node.Labels["node.deckhouse.io/group"]
	configurationChecksum := node.Annotations["node.deckhouse.io/configuration-checksum"]

	var isReady bool
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	return statusNode{
		Name:               node.Name,
		CloudInstanceGroup: cloudInstanceGroup,
		IsReady:            isReady,
		Checksum:           configurationChecksum,
		NodeForConditions:  conditions.NodeToConditionsNode(&node),
	}, nil
}

func updStatusFilterNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	var minPerZone, maxPerZone int32
	if ng.Spec.CloudInstances.MinPerZone != nil {
		minPerZone = *ng.Spec.CloudInstances.MinPerZone
	}

	if ng.Spec.CloudInstances.MaxPerZone != nil {
		maxPerZone = *ng.Spec.CloudInstances.MaxPerZone
	}

	zonesNum := len(ng.Spec.CloudInstances.Zones)

	return statusNodeGroup{
		Name:       ng.Name,
		NodeType:   ng.Spec.NodeType,
		MinPerZone: minPerZone,
		MaxPerZone: maxPerZone,
		ZonesNum:   int32(zonesNum),
		Error:      ng.Status.Error,

		UID:        ng.UID,
		Conditions: ng.Status.Conditions,
	}, nil
}

func updStatusFilterMachine(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var machine v1alpha1.Machine

	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	nodeGroup := machine.Spec.NodeTemplateSpec.Labels["node.deckhouse.io/group"]

	return nodeGroup, nil
}

func updStatusFilterCapiMachine(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	machine := capiv1beta1.Machine{}
	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	nodeGroup := machine.Labels["node-group"]

	return nodeGroup, nil
}

// returns count of zones for current cluster
func updStatusFilterCpSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	var res []string

	zonesDataBytes := sec.Data["zones"]

	err = json.Unmarshal(zonesDataBytes, &res)
	if err != nil {
		return nil, err
	}

	return int32(len(res)), nil
}

func handleUpdateNGStatus(input *go_hook.HookInput) error {
	var defaultZonesNum int32

	snap := input.Snapshots["zones_count"]
	if len(snap) > 0 {
		defaultZonesNum = snap[0].(int32)
	}

	// machine deployments snapshot
	snap = input.Snapshots["mds"]
	mdMap := make(map[string][]statusMachineDeployment)
	for _, res := range snap {
		md := res.(statusMachineDeployment)

		// group by nodeGroup
		if v, ok := mdMap[md.NodeGroup]; ok {
			v = append(v, md)
			mdMap[md.NodeGroup] = v
		} else {
			mdMap[md.NodeGroup] = []statusMachineDeployment{md}
		}

		// set metric for MachineDeployment
		labels := map[string]string{
			"node_group": md.NodeGroup,
			"name":       md.Name,
		}

		input.MetricsCollector.Set("machine_deployment_node_group_info", 1, labels)
	}

	// count instances of each node group
	instances := make(map[string]int32)
	snap = input.Snapshots["instances"]
	for _, res := range snap {
		instanceNodeGroup := res.(string)
		if count, ok := instances[instanceNodeGroup]; ok {
			count++
			instances[instanceNodeGroup] = count
		} else {
			instances[instanceNodeGroup] = 1
		}
	}
	snap = input.Snapshots["capi_instances"]
	for _, res := range snap {
		instanceNodeGroup := res.(string)
		if count, ok := instances[instanceNodeGroup]; ok {
			count++
			instances[instanceNodeGroup] = count
		} else {
			instances[instanceNodeGroup] = 1
		}
	}

	// store configuration checksums for each node group
	checksums := make(map[string]string)
	snap = input.Snapshots["configuration_checksums_secret"]
	if len(snap) > 0 {
		for k, v := range snap[0].(shared.ConfigurationChecksum) {
			checksums[k] = v
		}
	}

	snap = input.Snapshots["nodes"]
	nodes := make([]statusNode, 0, len(snap))
	for _, sn := range snap {
		node := sn.(statusNode)
		nodes = append(nodes, node)
	}

	// iterate over all node groups and calculate desired and current status
	snap = input.Snapshots["ngs"]
	for _, res := range snap {
		nodeGroup := res.(statusNodeGroup)

		ngName := nodeGroup.Name

		// calculate nodes and their status
		var nodesNum, readyNodesNum, uptodateNodesCount int32
		nodesForCalcConditions := make([]*conditions.Node, 0, len(nodes))

		for _, node := range nodes {
			if node.CloudInstanceGroup == ngName {
				nodesForCalcConditions = append(nodesForCalcConditions, node.NodeForConditions)
				nodesNum++
				if node.IsReady {
					readyNodesNum++
				}
			}

			ngChecksum := checksums[ngName]

			if node.Checksum == ngChecksum {
				uptodateNodesCount++
			}
		}

		// calculate zonecount
		zonesCount := nodeGroup.ZonesNum
		if zonesCount == 0 {
			zonesCount = defaultZonesNum
		}

		minPerZone := nodeGroup.MinPerZone * zonesCount
		maxPerZone := nodeGroup.MaxPerZone * zonesCount

		var desiredMax int32
		var lastMachineFailures []*v1alpha1.MachineSummary

		mds := mdMap[ngName]
		hasFrozenMd := false
		for _, md := range mds {
			desiredMax += md.Replicas
			lastMachineFailures = append(lastMachineFailures, md.LastMachineFailures...)
			if !hasFrozenMd {
				hasFrozenMd = md.IsFrozen
			}
		}

		if minPerZone > desiredMax {
			desiredMax = minPerZone
		}

		var failureReason string
		if len(lastMachineFailures) > 0 {
			sort.Slice(lastMachineFailures, func(i, j int) bool {
				imts := lastMachineFailures[i].LastOperation.LastUpdateTime
				jmts := lastMachineFailures[j].LastOperation.LastUpdateTime

				return imts.Before(&jmts)
			})
			failureReason = lastMachineFailures[len(lastMachineFailures)-1].LastOperation.Description
		}
		statusMsg := fmt.Sprintf("%s %s", nodeGroup.Error, failureReason)
		statusMsg = strings.TrimSpace(statusMsg)
		if len(statusMsg) > 0 {
			// truncate to maximum allowed message limit
			// https://github.com/kubernetes/kubernetes/blob/3e442b74f717ab2f43897d7af50de6114486e459/pkg/apis/core/validation/events.go#L38
			if len(statusMsg) > 1024 {
				statusMsg = statusMsg[:1024]
			}
			prev := ngStatusCache[nodeGroup.Name]
			// skip events with the same in-row message
			if prev != statusMsg {
				err := createEvent(input, nodeGroup, statusMsg)
				if err != nil {
					return err
				}
				ngStatusCache[nodeGroup.Name] = statusMsg
			}
			// rewrite status message for NG description field
			statusMsg = "Machine creation failed. Check events for details."
		}

		instancesCount := instances[ngName]

		ngForConditions := conditions.NodeGroup{
			Type:      nodeGroup.NodeType,
			Desired:   desiredMax,
			Instances: instancesCount,

			HasFrozenMachineDeployment: hasFrozenMd,
		}
		errors := make([]string, 0, 2)
		if len(nodeGroup.Error) > 0 {
			errors = append(errors, nodeGroup.Error)
		}
		if len(failureReason) > 0 {
			errors = append(errors, failureReason)
		}
		newConditions := conditions.CalculateNodeGroupConditions(
			ngForConditions,
			nodesForCalcConditions,
			nodeGroup.Conditions,
			errors,
			int(minPerZone),
		)

		patch := buildUpdateStatusPatch(
			nodesNum, readyNodesNum, uptodateNodesCount,
			minPerZone, maxPerZone,
			desiredMax, instancesCount,
			nodeGroup.NodeType, statusMsg,
			lastMachineFailures, newConditions,
		)

		patchNodeGroupStatus(input.PatchCollector, ngName, patch)
		// set CR processed status
		input.PatchCollector.Filter(set_cr_statuses.SetProcessedStatus(applyNodeGroupCrdFilter), "deckhouse.io/v1", "nodegroup", "", ngName, object_patch.WithSubresource("/status"), object_patch.IgnoreHookError())
	}

	return nil
}

func createEvent(input *go_hook.HookInput, nodeGroup statusNodeGroup, msg string) error {
	eventType := corev1.EventTypeWarning
	reason := "MachineFailed"

	if msg == "Started Machine creation process" {
		eventType = corev1.EventTypeNormal
		reason = "MachineCreating"
	}
	now := time.Now()

	event := buildEventV1(nodeGroup, eventType, reason, msg, now)

	input.PatchCollector.Create(event)
	return nil
}

func buildEventV1(nodeGroup statusNodeGroup, eventType, reason, msg string, now time.Time) *eventsv1.Event {
	return &eventsv1.Event{
		TypeMeta: v1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			// Namespace field has to be filled - event will not be created without it
			// and we have to set 'default' value here for linking this event with a NodeGroup object, which is global
			// if we set 'd8-cloud-instance-manager' here for example, `Events` field on `kubectl describe ng $X` will be empty
			Namespace:    "default",
			GenerateName: "ng-" + nodeGroup.Name + "-",
		},
		Regarding: corev1.ObjectReference{
			Kind:       "NodeGroup",
			Name:       nodeGroup.Name,
			UID:        nodeGroup.UID,
			APIVersion: "deckhouse.io/v1",
		},
		Reason:              reason,
		Note:                msg,
		Type:                eventType,
		EventTime:           v1.MicroTime{Time: now},
		Action:              "Binding",
		ReportingInstance:   "deckhouse",
		ReportingController: "deckhouse",
	}
}

type statusNodeGroup struct {
	Name       string
	NodeType   ngv1.NodeType
	MinPerZone int32
	MaxPerZone int32
	ZonesNum   int32
	Error      string

	Conditions []ngv1.NodeGroupCondition

	// for event generation
	UID apimtypes.UID
}

type statusNode struct {
	NodeForConditions  *conditions.Node
	Name               string
	CloudInstanceGroup string
	IsReady            bool
	Checksum           string
}

type statusMachineDeployment struct {
	Name                string
	IsFrozen            bool
	Replicas            int32
	NodeGroup           string
	LastMachineFailures []*v1alpha1.MachineSummary
}
