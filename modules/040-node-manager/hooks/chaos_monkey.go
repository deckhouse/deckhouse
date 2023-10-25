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
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/node-manager/chaos_monkey",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ngs",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   chaosFilterNodeGroup,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   chaosFilterNode,
		},
		{
			Name:                         "machines",
			ApiVersion:                   "machine.sapcloud.io/v1alpha1",
			Kind:                         "Machine",
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   chaosFilterMachine,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "monkey",
			Crontab: "* * * * *",
		},
	},
}, handleChaosMonkey)

func handleChaosMonkey(input *go_hook.HookInput) error {
	random := time.Now().UnixNano()
	testRandomSeed := os.Getenv("D8_TEST_RANDOM_SEED")
	if testRandomSeed != "" {
		res, _ := strconv.ParseInt(testRandomSeed, 10, 64)
		random = res
	}
	randomizer := rand.New(rand.NewSource(random))

	nodeGroups, machines, nodes, err := prepareChaosData(input)
	if err != nil {
		input.LogEntry.Infof(err.Error()) // just info message, already have a victim
		return nil
	}

	// preparation complete, main hook logic goes here
	for _, ng := range nodeGroups {
		if ng.ChaosMode != "DrainAndDelete" {
			continue
		}

		chaosPeriod, err := time.ParseDuration(ng.ChaosPeriod)
		if err != nil {
			input.LogEntry.Warnf("chaos period (%s) for NodeGroup:%s is invalid", ng.ChaosPeriod, ng.Name)
			continue
		}

		run := randomizer.Uint32() % uint32(chaosPeriod.Milliseconds()/1000/60)

		if run != 0 {
			continue
		}

		nodeGroupNodes := nodes[ng.Name]
		if len(nodeGroupNodes) == 0 {
			continue
		}

		victimNode := nodeGroupNodes[randomizer.Intn(len(nodeGroupNodes))]

		victimMachine, ok := machines[victimNode.Name]
		if !ok {
			continue
		}

		input.PatchCollector.MergePatch(victimAnnotationPatch, "machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", victimMachine.Name)

		input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", victimMachine.Name, object_patch.InBackground())
	}

	return nil
}

func prepareChaosData(input *go_hook.HookInput) ([]chaosNodeGroup, map[string]chaosMachine, map[string][]chaosNode, error) {
	snap := input.Snapshots["machines"]
	machines := make(map[string]chaosMachine, len(snap)) // map by node name
	for _, sn := range snap {
		machine := sn.(chaosMachine)
		if machine.IsAlreadyMonkeyVictim {
			return nil, nil, nil, fmt.Errorf("machine %s is already marked as chaos monkey victim. Exiting", machine.Name) // If there are nodes in deleting state then do nothing
		}
		machines[machine.Node] = machine
	}

	// collect NodeGroup with Enabled chaos monkey
	snap = input.Snapshots["ngs"]
	nodeGroups := make([]chaosNodeGroup, 0)
	for _, sn := range snap {
		ng := sn.(chaosNodeGroup)
		// if chaos mode is empty - it's disabled
		if ng.ChaosMode == "" || !ng.IsReadyForChaos {
			continue
		}
		nodeGroups = append(nodeGroups, ng)
	}

	// map nodes by NodeGroup
	nodes := make(map[string][]chaosNode)
	snap = input.Snapshots["nodes"]
	for _, sn := range snap {
		node := sn.(chaosNode)
		if v, ok := nodes[node.NodeGroup]; ok {
			v = append(v, node)
			nodes[node.NodeGroup] = v
		} else {
			nodes[node.NodeGroup] = []chaosNode{node}
		}
	}

	return nodeGroups, machines, nodes, nil
}

func chaosFilterMachine(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var machine v1alpha1.Machine

	err := sdk.FromUnstructured(obj, &machine)
	if err != nil {
		return nil, err
	}

	isMonkeyVictim := false
	if _, ok := machine.Labels["node.deckhouse.io/chaos-monkey-victim"]; ok {
		isMonkeyVictim = true
	}

	return chaosMachine{
		Name:                  machine.Name,
		Node:                  machine.Labels["node"],
		IsAlreadyMonkeyVictim: isMonkeyVictim,
	}, nil
}

func chaosFilterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	return chaosNode{
		Name:      node.Name,
		NodeGroup: node.Labels["node.deckhouse.io/group"],
	}, nil
}

func chaosFilterNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	isReadyForChaos := false
	if ng.Spec.NodeType == ngv1.NodeTypeCloudEphemeral {
		if ng.Status.Desired > 1 && ng.Status.Desired == ng.Status.Ready {
			isReadyForChaos = true
		}
	} else {
		if ng.Status.Nodes > 1 && ng.Status.Nodes == ng.Status.Ready {
			isReadyForChaos = true
		}
	}

	period := ng.Spec.Chaos.Period
	if period == "" {
		period = "6h"
	}

	return chaosNodeGroup{
		Name:            ng.Name,
		ChaosMode:       ng.Spec.Chaos.Mode,
		ChaosPeriod:     period,
		IsReadyForChaos: isReadyForChaos,
	}, nil
}

type chaosNodeGroup struct {
	Name            string
	ChaosMode       string
	ChaosPeriod     string // default 6h
	IsReadyForChaos bool
}

type chaosMachine struct {
	Name                  string
	Node                  string
	IsAlreadyMonkeyVictim bool
}

type chaosNode struct {
	Name      string
	NodeGroup string
}

var (
	victimAnnotationPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"node.deckhouse.io/chaos-monkey-victim": "",
			},
		},
	}
)
