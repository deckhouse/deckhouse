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
	deprecatedEbpfSchedulingLabelKey = "monitoring-kubernetes.deckhouse.io/ebpf-supported"
)

type NodeWithLabel struct {
	Name string
}

func getNodeNameWithSupportedDistro(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	if _, ok := node.Labels[deprecatedEbpfSchedulingLabelKey]; ok {
		return &NodeWithLabel{Name: node.Name}, nil
	}

	return nil, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: getNodeNameWithSupportedDistro,
		},
	},
}, unlabelNodes)

func unlabelNodes(_ context.Context, input *go_hook.HookInput) error {
	snapshot := input.Snapshots.Get("nodes")
	for labeledNodeRaw, err := range sdkobjectpatch.SnapshotIter[NodeWithLabel](snapshot) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshot: %w", err)
		}

		input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var node v1.Node
			err := sdk.FromUnstructured(obj, &node)
			if err != nil {
				return nil, err
			}

			delete(node.Labels, deprecatedEbpfSchedulingLabelKey)

			return sdk.ToUnstructured(&node)
		}, "v1", "Node", "", labeledNodeRaw.Name)
	}

	return nil
}
