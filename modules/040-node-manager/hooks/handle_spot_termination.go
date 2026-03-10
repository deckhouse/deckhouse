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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	spotTerminationTaintKey       = "aws-node-termination-handler/spot-itn"
	spotDrainingAnnotationKey     = "update.node.deckhouse.io/draining"
	spotTerminationDrainingSource = "spot-termination"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/spot-termination",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_with_spot_taint",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			FilterFunc:                   spotTaintFilter,
		},
	},
}, handleSpotTermination)

type spotTaintedNode struct {
	Name                 string
	HasSpotTaint         bool
	HasDrainingAnnotation bool
}

func spotTaintFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result := spotTaintedNode{
		Name:                 node.Name,
		HasSpotTaint:         false,
		HasDrainingAnnotation: false,
	}

	// Check if node has spot termination taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == spotTerminationTaintKey {
			result.HasSpotTaint = true
			break
		}
	}

	// Check if node already has draining annotation
	if _, ok := node.Annotations[spotDrainingAnnotationKey]; ok {
		result.HasDrainingAnnotation = true
	}

	// Only return nodes that have spot taint
	if !result.HasSpotTaint {
		return nil, nil
	}

	return result, nil
}

func handleSpotTermination(_ context.Context, input *go_hook.HookInput) error {
	nodes := input.Snapshots.Get("nodes_with_spot_taint")

	for node, err := range sdkobjectpatch.SnapshotIter[spotTaintedNode](nodes) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_with_spot_taint' snapshots: %w", err)
		}

		// If node has spot taint but doesn't have draining annotation, add it
		if node.HasSpotTaint && !node.HasDrainingAnnotation {
			input.Logger.Info("Adding draining annotation for spot-terminated node", "node", node.Name)

			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						spotDrainingAnnotationKey: spotTerminationDrainingSource,
					},
				},
			}

			input.PatchCollector.PatchWithMerge(patch, "v1", "Node", "", node.Name)
		}
	}

	return nil
}
