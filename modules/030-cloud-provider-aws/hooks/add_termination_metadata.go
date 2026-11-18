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
	awsSpotITNTaintKey                 = "aws-node-termination-handler/spot-itn"
	awsRebalanceRecommendationTaintKey = "aws-node-termination-handler/rebalance-recommendation"

	terminationLabelKey   = "node.deckhouse.io/termination-in-progress"
	drainingAnnotationKey = "update.node.deckhouse.io/draining"

	awsTerminationSource = "aws-node-termination-handler"
)

var awsTerminationTaintKeys = map[string]struct{}{
	awsSpotITNTaintKey:                 {},
	awsRebalanceRecommendationTaintKey: {},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-aws/termination-metadata",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_with_termination_taint",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			FilterFunc:                   awsTerminationTaintFilter,
		},
	},
}, addTerminationMetadata)

type nodeWithTerminationTaint struct {
	Name                string
	HasTerminationTaint bool
	HasTerminationLabel bool
}

func awsTerminationTaintFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	result := nodeWithTerminationTaint{
		Name: node.Name,
	}

	for _, taint := range node.Spec.Taints {
		if _, ok := awsTerminationTaintKeys[taint.Key]; ok {
			result.HasTerminationTaint = true
			break
		}
	}

	if val, ok := node.Labels[terminationLabelKey]; ok && val == "true" {
		result.HasTerminationLabel = true
	}

	if result.HasTerminationTaint {
		return result, nil
	}

	return nil, nil
}

func addTerminationMetadata(_ context.Context, input *go_hook.HookInput) error {
	nodesSnapshot := input.Snapshots.Get("nodes_with_termination_taint")

	for node, err := range sdkobjectpatch.SnapshotIter[nodeWithTerminationTaint](nodesSnapshot) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_with_termination_taint' snapshots: %w", err)
		}

		if node.HasTerminationLabel {
			continue
		}

		input.Logger.Info(
			"Adding termination metadata for AWS termination-tainted node",
			"node", node.Name,
		)

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					terminationLabelKey: "true",
				},
				"annotations": map[string]interface{}{
					drainingAnnotationKey: awsTerminationSource,
				},
			},
		}
		input.PatchCollector.PatchWithMerge(patch, "v1", "Node", "", node.Name)
	}

	return nil
}
