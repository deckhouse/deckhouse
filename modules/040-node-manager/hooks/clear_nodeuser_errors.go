// Copyright 2024 Flant JSC
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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	golibset "github.com/deckhouse/deckhouse/go_lib/set"
	nodeuserv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	nodeForClearSnapName     = "nodes_for_clear"
	nodeUserForClearSnapName = "nodeuser_for_clear"
)

type nodeUsersForClear struct {
	Name         string
	StatusErrors map[string]string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         nodeForClearSnapName,
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: applyNodesForClearFilter,
		},
		{
			Name:                         nodeUserForClearSnapName,
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeUser",
			FilterFunc:                   applyNodeUsersForClearFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "clear_nodeuser_errors",
			Crontab: "*/30 * * * *",
		},
	},
}, discoverNodeUsersForClear)

func applyNodesForClearFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	return node.Name, nil
}

func applyNodeUsersForClearFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeUser nodeuserv1.NodeUser

	err := sdk.FromUnstructured(obj, &nodeUser)
	if err != nil {
		return nil, err
	}

	return nodeUsersForClear{
		Name:         nodeUser.Name,
		StatusErrors: nodeUser.Status.Errors,
	}, nil
}

func discoverNodeUsersForClear(input *go_hook.HookInput) error {
	nodeUserSnap := input.Snapshots[nodeUserForClearSnapName]
	if len(nodeUserSnap) == 0 {
		return nil
	}

	nodes := golibset.NewFromSnapshot(input.Snapshots[nodeForClearSnapName])

	for _, item := range nodeUserSnap {
		nuForClear := item.(nodeUsersForClear)
		input.LogEntry.Debugf("clearErrors--> NodeUsers: %v Nodes: %v", nuForClear, nodes)
		if incorrectNodes := hasIncorrectNodeUserErrors(nuForClear.StatusErrors, nodes); len(
			incorrectNodes,
		) > 0 {
			input.LogEntry.Debugf("clearErrors--> incorrectNodes: %v", incorrectNodes)
			err := clearNodeUserIncorrectErrors(nuForClear.Name, incorrectNodes, input)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func hasIncorrectNodeUserErrors(
	nodeUserStatusError map[string]string,
	nodes golibset.Set,
) []string {
	result := make([]string, 0)
	for k := range nodeUserStatusError {
		if !nodes.Has(k) {
			result = append(result, k)
		}
	}
	return result
}

func clearNodeUserIncorrectErrors(
	nodeUserName string,
	incorrectNodes []string,
	input *go_hook.HookInput,
) error {
	patch := map[string]map[string]map[string]interface{}{
		"status": {
			"errors": {},
		},
	}

	for _, node := range incorrectNodes {
		patch["status"]["errors"][node] = nil
	}

	input.LogEntry.Debugf("clearErrors--> patch: %v", patch)
	input.PatchCollector.MergePatch(
		patch,
		"deckhouse.io/v1",
		"NodeUser",
		"",
		nodeUserName,
		object_patch.WithSubresource("/status"),
	)
	return nil
}
