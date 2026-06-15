/*
Copyright 2025 Flant JSC

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
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	terminationLabelKey      = "node.deckhouse.io/termination-in-progress"
	spotDrainedAnnotationKey = "update.node.deckhouse.io/drained"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/spot-instance-deletion",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_drained_for_spot_termination",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					terminationLabelKey: "true",
				},
			},
			FilterFunc: spotDrainedFilter,
		},
	},
}, handleSpotInstanceDeletion)

type spotDrainedNode struct {
	Name                 string
	HasTerminationLabel  bool
	HasDrainedAnnotation bool
}

func spotDrainedFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result := spotDrainedNode{
		Name:                 node.Name,
		HasTerminationLabel:  false,
		HasDrainedAnnotation: false,
	}

	// Check if node has termination label (already filtered by LabelSelector, but check for safety)
	if val, ok := node.Labels[terminationLabelKey]; ok && val == "true" {
		result.HasTerminationLabel = true
	}

	// Check if node has drained annotation (any source)
	if _, ok := node.Annotations[spotDrainedAnnotationKey]; ok {
		result.HasDrainedAnnotation = true
	}

	// Only return nodes that have both label and drained annotation
	if result.HasTerminationLabel && result.HasDrainedAnnotation {
		return result, nil
	}

	return nil, nil
}

func handleSpotInstanceDeletion(_ context.Context, input *go_hook.HookInput) error {
	nodes := input.Snapshots.Get("nodes_drained_for_spot_termination")

	for node, err := range sdkobjectpatch.SnapshotIter[spotDrainedNode](nodes) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_drained_for_spot_termination' snapshots: %w", err)
		}

		input.Logger.Info("Deleting Instance for drained spot-terminated node", "node", node.Name)

		// Delete the corresponding Instance object
		// This will trigger machine-controller-manager to delete the machine and VM
		input.PatchCollector.DeleteInBackground("deckhouse.io/v1alpha1", "Instance", "", node.Name)
	}

	return nil
}
