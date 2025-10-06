/*
Copyright 2024 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/reconcile_masters_node",
	Schedule: []go_hook.ScheduleConfig{
		{
			Crontab: "*/15 * * * *",
			Name:    "reconcicle-masters-node",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: reconcicleMastersFilterNode,
		},
	},
}, handleRecicleMastersNode)

func reconcicleMastersFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	Node := recicleMastersNode{
		Name: node.Name,
	}

	return Node, nil
}

type recicleMastersNode struct {
	Name string
}

func handleRecicleMastersNode(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("master_nodes")

	if len(snaps) == 0 {
		input.Logger.Debug("No master Nodes found in snapshot, skipping iteration")
		return nil
	}

	mastersName := make([]string, 0, len(snaps))
	for node, err := range sdkobjectpatch.SnapshotIter[recicleMastersNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'master_nodes' snapshots: %v", err)
		}

		if node.Name == "" {
			return fmt.Errorf("node_name should not be empty")
		}

		mastersName = append(mastersName, node.Name)
	}

	input.Values.Set("controlPlaneManager.internal.mastersNode", mastersName)

	return nil
}
