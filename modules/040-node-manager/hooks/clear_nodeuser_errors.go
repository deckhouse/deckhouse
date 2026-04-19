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

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
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
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
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

func discoverNodeUsersForClear(_ context.Context, input *go_hook.HookInput) error {
	nodeUserSnap := input.Snapshots.Get(nodeUserForClearSnapName)
	if len(nodeUserSnap) == 0 {
		return nil
	}

	nodes := golibset.NewFromSnapshot(input.Snapshots.Get(nodeForClearSnapName))
	for nuForClear, err := range sdkobjectpatch.SnapshotIter[nodeUsersForClear](nodeUserSnap) {
		if err != nil {
			return fmt.Errorf("failed to iterate over node_users_for_clear snapshot: %w", err)
		}

		input.Logger.Debug("clearErrors", slog.Any("NodeUsers", nuForClear), slog.Any("Nodes", nodes))
		if incorrectNodes := hasIncorrectNodeUserErrors(nuForClear.StatusErrors, nodes); len(
			incorrectNodes,
		) > 0 {
			input.Logger.Debug("clearErrors", slog.Any("incorrectNodes", incorrectNodes))
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

// TODO (core): fix this linter
//
//nolint:unparam
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

	input.Logger.Debug("clearErrors", slog.Any("patch", patch))
	input.PatchCollector.PatchWithMerge(
		patch,
		"deckhouse.io/v1",
		"NodeUser",
		"",
		nodeUserName,
		object_patch.WithSubresource("/status"),
	)
	return nil
}
