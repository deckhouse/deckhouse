/*
Copyright 2022 Flant JSC

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
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	preemtibleVMDeletionDuration = 24 * time.Hour
)

type Node struct {
	Name              string
	NodeGroup         string
	CreationTimestamp metav1.Time
}

type Machine struct {
	Name                  string
	nodeCreationTimestamp metav1.Time
	nodeGroup             string
	Terminating           bool
	MachineClassKind      string
	MachineClassName      string
}

type YandexMachineClass struct {
	Name string
}

type NodeGroupStatus struct {
	Name  string
	Nodes float64
	Ready float64
}

func applyMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var terminating bool
	if obj.GetDeletionTimestamp() != nil {
		terminating = true
	}

	classKind, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "class", "kind")
	if err != nil {
		return nil, fmt.Errorf("can't access class name of Machine %q: %s", obj.GetName(), err)
	}
	if len(classKind) == 0 {
		return nil, fmt.Errorf("spec.class.kind is empty in %q", obj.GetName())
	}

	className, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "class", "name")
	if err != nil {
		return nil, fmt.Errorf("can't access class name of Machine %q: %s", obj.GetName(), err)
	}
	if len(className) == 0 {
		return nil, fmt.Errorf("spec.class.name is empty in %q", obj.GetName())
	}

	return &Machine{
		Name:             obj.GetName(),
		Terminating:      terminating,
		MachineClassKind: classKind,
		MachineClassName: className,
	}, nil
}

func applyNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	icKind, icExists, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "cloudInstances", "classReference", "kind")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"spec.cloudInstances.classReference.kind\" in a NodeGroup %s", obj.GetName())
	}

	if !icExists || (icKind != "YandexInstanceClass") {
		return nil, nil
	}

	nodeCount, nodeCountExists, err := unstructured.NestedFloat64(obj.UnstructuredContent(), "status", "nodes")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.nodes\" in a NodeGroup %s", obj.GetName())
	}
	readyNodeCount, readyNodeCountExists, err := unstructured.NestedFloat64(obj.UnstructuredContent(), "status", "ready")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.ready\" in a NodeGroup %s", obj.GetName())
	}

	if !nodeCountExists || !readyNodeCountExists {
		return nil, nil
	}

	if (nodeCount < 0) || (readyNodeCount < 0) {
		return nil, nil
	}

	return &NodeGroupStatus{
		Name:  obj.GetName(),
		Nodes: nodeCount,
		Ready: readyNodeCount,
	}, nil
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()

	ng, ok := labels["node.deckhouse.io/group"]
	if !ok {
		return nil, nil
	}

	return &Node{
		Name:              obj.GetName(),
		NodeGroup:         ng,
		CreationTimestamp: obj.GetCreationTimestamp(),
	}, nil
}

func isPreemptibleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	preemptible, ok, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "schedulingPolicy", "preemptible")
	if err != nil {
		return nil, fmt.Errorf("can't access field \"spec.schedulingPolicy.preemptible\" of YandexMachineClass %q: %s", obj.GetName(), err)
	}

	if ok && preemptible {
		return &YandexMachineClass{
			Name: obj.GetName(),
		}, nil
	}

	return nil, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	AllowFailure: true,
	Queue:        "/modules/cloud-provider-yandex/preemtibly-delete-preemtible-instances",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "every-15",
			Crontab: "0/15 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "mcs",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "machine.sapcloud.io/v1alpha1",
			Kind:                "YandexMachineClass",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: isPreemptibleFilter,
		},
		{
			Name:                "machines",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "machine.sapcloud.io/v1alpha1",
			Kind:                "Machine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: applyMachineFilter,
		},
		{
			Name:                "nodes",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "v1",
			Kind:                "Node",
			FilterFunc:          applyNodeFilter,
		},
		{
			Name:                "nodegroupstatuses",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "NodeGroup",
			FilterFunc:          applyNodeGroupFilter,
		},
	},
}, deleteMachines)

func deleteMachines(input *go_hook.HookInput) error {
	var (
		timeNow                        = time.Now().UTC()
		machines                       []*Machine
		preemptibleMachineClassesSet   = set.Set{}
		nodeNameToNodeMap              = make(map[string]*Node)
		nodeGroupNameToNodeGroupStatus = make(map[string]*NodeGroupStatus)
	)

	for _, mcRaw := range input.Snapshots["mcs"] {
		if mcRaw == nil {
			continue
		}

		ic, ok := mcRaw.(*YandexMachineClass)
		if !ok {
			return fmt.Errorf("failed to assert to *YandexMachineClass")
		}

		preemptibleMachineClassesSet.Add(ic.Name)
	}

	if preemptibleMachineClassesSet.Size() == 0 {
		return nil
	}

	for _, nodeRaw := range input.Snapshots["nodes"] {
		if nodeRaw == nil {
			continue
		}

		node, ok := nodeRaw.(*Node)
		if !ok {
			return fmt.Errorf("failed to assert to *Node")
		}

		nodeNameToNodeMap[node.Name] = node
	}

	for _, ngStatusRaw := range input.Snapshots["nodegroupstatuses"] {
		if ngStatusRaw == nil {
			continue
		}

		ngStatus, ok := ngStatusRaw.(*NodeGroupStatus)
		if !ok {
			return fmt.Errorf("failed to assert to *NodeGroupStatus")
		}

		nodeGroupNameToNodeGroupStatus[ngStatus.Name] = ngStatus
	}

	for _, machineRaw := range input.Snapshots["machines"] {
		machine, ok := machineRaw.(*Machine)
		if !ok {
			return fmt.Errorf("failed to assert to *Machine")
		}

		if machine.Terminating {
			continue
		}

		if machine.MachineClassKind != "YandexMachineClass" {
			continue
		}

		if !preemptibleMachineClassesSet.Has(machine.MachineClassName) {
			continue
		}

		if node, ok := nodeNameToNodeMap[machine.Name]; ok {
			machine.nodeCreationTimestamp = node.CreationTimestamp
			machine.nodeGroup = node.NodeGroup
		} else {
			continue
		}

		machines = append(machines, machine)
	}

	if len(machines) == 0 {
		return nil
	}

	for _, m := range getMachinesToDelete(timeNow, machines, nodeGroupNameToNodeGroupStatus) {
		input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", m)
	}

	return nil
}

// delete all after 23h mark
// afterwards delete in 15 minutes increments, no more than batch size
func getMachinesToDelete(timeNow time.Time, machines []*Machine, ngToNgStatusMap map[string]*NodeGroupStatus) (machinesToDelete []string) {
	const (
		// 12 * 0.25 = 3 hours
		durationIterations = 12
		slidingStep        = 15 * time.Minute
		nodeReadinessRatio = 0.9
	)
	var (
		currentSlidingDuration = preemtibleVMDeletionDuration - time.Hour
		cursor                 int
	)

	sort.Slice(machines, func(i, j int) bool {
		return machines[i].nodeCreationTimestamp.Before(&machines[j].nodeCreationTimestamp)
	})

	batch := len(machines) / durationIterations
	if batch == 0 {
		batch = 1
	}

	for t := 0; t < durationIterations; t++ {
		currentSlidingDuration -= slidingStep

		for cursor < len(machines) {
			if len(machinesToDelete) >= batch {
				break
			}

			currentMachine := machines[cursor]

			if expires(timeNow, currentMachine.nodeCreationTimestamp.Time, currentSlidingDuration) {
				ngStatus, ok := ngToNgStatusMap[currentMachine.nodeGroup]
				if !ok {
					continue
				}

				if (ngStatus.Ready / ngStatus.Nodes) < nodeReadinessRatio {
					break
				}

				machinesToDelete = append(machinesToDelete, currentMachine.Name)
				cursor++
			} else {
				break
			}
		}
	}

	return
}

func expires(now, timestamp time.Time, expirationDuration time.Duration) bool {
	return timestamp.Add(expirationDuration).Before(now)
}
