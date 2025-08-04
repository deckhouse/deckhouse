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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	BashibleFirstRunFinishedLabel = "node.deckhouse.io/bashible-first-run-finished"
	BashibleUninitializedTaintKey = "node.deckhouse.io/bashible-uninitialized"
)

type NodesInfo struct {
	Name   string
	Labels map[string]string
	Taints []v1.Taint
}

func nodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	return NodesInfo{
		Name:   node.Name,
		Labels: node.Labels,
		Taints: node.Spec.Taints,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/remove_bashible_completed_labels_and_taints",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: nodeFilter,
		},
	},
}, cleanupBashibleArtifacts)

func cleanupBashibleArtifacts(_ context.Context, input *go_hook.HookInput) error {
	snapshots := input.Snapshots.Get("nodes")
	if len(snapshots) == 0 {
		return nil
	}

	for node, err := range sdkobjectpatch.SnapshotIter[NodesInfo](snapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes snapshots': %v", err)
		}

		hasTaint := false
		for _, taint := range node.Taints {
			if taint.Key == BashibleUninitializedTaintKey {
				hasTaint = true
				break
			}
		}
		hasLabel := false
		_, labelExists := node.Labels[BashibleFirstRunFinishedLabel]
		if labelExists {
			hasLabel = true
		}

		if hasLabel {
			input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				nodeObj := &v1.Node{}
				err := sdk.FromUnstructured(obj, nodeObj)
				if err != nil {
					return nil, err
				}

				// Remove the label if hasLabel is true
				delete(nodeObj.Labels, BashibleFirstRunFinishedLabel)

				// Remove the taint if hasTaint is true
				if hasTaint {
					taints := make([]v1.Taint, 0)
					for _, t := range nodeObj.Spec.Taints {
						if t.Key != BashibleUninitializedTaintKey {
							taints = append(taints, t)
						}
					}
					if len(taints) == 0 {
						nodeObj.Spec.Taints = nil
					} else {
						nodeObj.Spec.Taints = taints
					}
				}
				return sdk.ToUnstructured(nodeObj)
			}, "v1", "Node", "", node.Name)
		}
	}
	return nil
}
