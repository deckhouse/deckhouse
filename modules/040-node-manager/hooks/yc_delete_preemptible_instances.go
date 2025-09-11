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
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	// Preemptible instances are forcibly stopped by Yandex.Cloud after 24 hours
	// https://cloud.yandex.com/en-ru/docs/compute/concepts/preemptible-vm
	hookExecutionSchedule = 15 * time.Minute
	// we'll delete Machines that are almost ready to be terminated by the cloud provider
	durationThresholdForDeletion = 24*time.Hour - 4*time.Hour

	// we won't delete any Machines if it would violate overall Node readiness of a given NodeGroup
	nodeGroupReadinessRatio = 0.9
)

type Node struct {
	Name              string
	NodeGroup         string
	CreationTimestamp metav1.Time
}

type Machine struct {
	Name             string
	Terminating      bool
	MachineClassKind string
	MachineClassName string

	NodeCreationTimestamp metav1.Time
	NodeGroup             string
}

type NodeGroupStatus struct {
	Name  string
	Nodes int64
	Ready int64
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
		return nil, fmt.Errorf("cannot access \"spec.cloudInstances.classReference.kind\" in a NodeGroup %s: %s", obj.GetName(), err)
	}

	if !icExists || (icKind != "YandexInstanceClass") {
		return nil, nil
	}

	nodeCountRaw, nodeCountExists, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "status", "nodes")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.nodes\" in a NodeGroup %s: %s", obj.GetName(), err)
	}
	readyNodeCountRaw, readyNodeCountExists, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "status", "ready")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.ready\" in a NodeGroup %s: %s", obj.GetName(), err)
	}

	if !nodeCountExists || !readyNodeCountExists {
		return nil, nil
	}

	var nodeCount, readyNodeCount int64
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		nodeCount = int64(nodeCountRaw.(float64))
		readyNodeCount = int64(readyNodeCountRaw.(float64))
	} else {
		nodeCount = nodeCountRaw.(int64)
		readyNodeCount = readyNodeCountRaw.(int64)
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
		return obj.GetName(), nil
	}

	return nil, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	AllowFailure: true,
	// this hook relies on information set by update_node_group_status hook
	Queue: "/modules/node-manager/update_ngs_statuses",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name: "every-15",
			// string formatting is ugly, but serves a purpose of referencing an important constant
			Crontab: fmt.Sprintf("0/%.0f * * * *", hookExecutionSchedule.Minutes()),
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

func deleteMachines(_ context.Context, input *go_hook.HookInput) error {
	var (
		timeNow                        = time.Now().UTC()
		machines                       = make([]*Machine, 0)
		preemptibleMachineClassesSet   = set.Set{}
		nodeNameToNodeMap              = make(map[string]*Node)
		nodeGroupNameToNodeGroupStatus = make(map[string]*NodeGroupStatus)
	)

	for mc, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get("mcs")) {
		if err != nil {
			return fmt.Errorf("failed to assert to string: failed to iterate over 'mcs' snapshot: %w", err)
		}

		preemptibleMachineClassesSet.Add(mc)
	}

	if preemptibleMachineClassesSet.Size() == 0 {
		return nil
	}

	for node, err := range sdkobjectpatch.SnapshotIter[Node](input.Snapshots.Get("nodes")) {
		if err != nil {
			return fmt.Errorf("failed to assert to Node: failed to iterate over 'nodes' snapshot: %w", err)
		}

		nodeNameToNodeMap[node.Name] = &node
	}

	for ngStatus, err := range sdkobjectpatch.SnapshotIter[NodeGroupStatus](input.Snapshots.Get("nodegroupstatuses")) {
		if err != nil {
			return fmt.Errorf("failed to assert to NodeGroupStatus: failed to iterate over 'nodegroupstatuses' snapshot: %w", err)
		}

		nodeGroupNameToNodeGroupStatus[ngStatus.Name] = &ngStatus
	}

	for machine, err := range sdkobjectpatch.SnapshotIter[Machine](input.Snapshots.Get("machines")) {
		if err != nil {
			return fmt.Errorf("failed to assert to Machine: failed to iterate over 'machines' snapshot: %w", err)
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
			machine.NodeCreationTimestamp = node.CreationTimestamp
			machine.NodeGroup = node.NodeGroup
		} else {
			continue
		}

		// skip young Machines
		if machine.NodeCreationTimestamp.Time.Add(durationThresholdForDeletion).After(timeNow) {
			continue
		}

		// skip Machines in NodeGroups that violate NodeGroup readiness ratio
		ngStatus, ok := nodeGroupNameToNodeGroupStatus[machine.NodeGroup]
		if !ok {
			continue
		}
		if (float64(ngStatus.Ready) / float64(ngStatus.Nodes)) < nodeGroupReadinessRatio {
			continue
		}

		machines = append(machines, &machine)
	}

	if len(machines) == 0 {
		return nil
	}

	for _, m := range getMachinesToDelete(machines) {
		input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", m)
	}

	return nil
}

func getMachinesToDelete(machines []*Machine) []string {
	sort.Slice(machines, func(i, j int) bool {
		return machines[i].NodeCreationTimestamp.Before(&machines[j].NodeCreationTimestamp)
	})

	// take 10% of old Machines
	batch := len(machines) / 10
	if batch == 0 {
		batch = 1
	}

	machinesToDelete := make([]string, 0, batch)
	for _, currentMachine := range machines {
		if len(machinesToDelete) < batch {
			machinesToDelete = append(machinesToDelete, currentMachine.Name)
		} else {
			break
		}
	}

	return machinesToDelete
}
