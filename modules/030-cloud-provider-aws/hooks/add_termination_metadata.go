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
	// AWS-specific taint key added by AWS Node Termination Handler
	awsSpotTaintKey = "aws-node-termination-handler/spot-itn"

	// Deckhouse standard labels and annotations
	terminationLabelKey   = "node.deckhouse.io/termination-in-progress"
	drainingAnnotationKey = "update.node.deckhouse.io/draining"

	// AWS-specific values
	awsTerminationSource = "aws-node-termination-handler"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-aws/termination-metadata",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_with_spot_taint",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			FilterFunc:                   awsSpotTaintFilter,
		},
	},
}, addTerminationMetadata)

type nodeWithSpotTaint struct {
	Name                  string
	HasSpotTaint          bool
	HasTerminationLabel   bool
	HasDrainingAnnotation bool
}

func awsSpotTaintFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result := nodeWithSpotTaint{
		Name:                  node.Name,
		HasSpotTaint:          false,
		HasTerminationLabel:   false,
		HasDrainingAnnotation: false,
	}

	// Check if node has AWS spot termination taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == awsSpotTaintKey {
			result.HasSpotTaint = true
			break
		}
	}

	// Check if node already has termination label
	if val, ok := node.Labels[terminationLabelKey]; ok && val == "true" {
		result.HasTerminationLabel = true
	}

	// Check if node already has draining annotation
	if _, ok := node.Annotations[drainingAnnotationKey]; ok {
		result.HasDrainingAnnotation = true
	}

	// Only return nodes that have spot taint
	// We'll check label and annotation presence in the handler
	if result.HasSpotTaint {
		return result, nil
	}

	return nil, nil
}

func addTerminationMetadata(_ context.Context, input *go_hook.HookInput) error {
	nodesSnapshot := input.Snapshots.Get("nodes_with_spot_taint")

	for node, err := range sdkobjectpatch.SnapshotIter[nodeWithSpotTaint](nodesSnapshot) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_with_spot_taint' snapshots: %w", err)
		}

		input.Logger.Info(
			"Adding termination metadata for AWS spot-tainted node",
			"node", node.Name,
		)

		metadata := make(map[string]interface{})

		// Add label if not present
		if !node.HasTerminationLabel {
			metadata["labels"] = map[string]interface{}{
				terminationLabelKey: "true",
			}
		}

		// Add annotation if not present
		if !node.HasDrainingAnnotation {
			metadata["annotations"] = map[string]interface{}{
				drainingAnnotationKey: awsTerminationSource,
			}
		}

		if len(metadata) > 0 {
			patch := map[string]interface{}{
				"metadata": metadata,
			}
			input.PatchCollector.PatchWithMerge(patch, "v1", "Node", "", node.Name)
		}
	}

	return nil
}
