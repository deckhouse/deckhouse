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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func unlabelNodes(input *go_hook.HookInput) error {
	snapshot := input.Snapshots["nodes"]
	for _, labeledNodeRaw := range snapshot {
		if labeledNodeRaw == nil {
			continue
		}
		labeledNode := labeledNodeRaw.(*NodeWithLabel)

		input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var node v1.Node
			err := sdk.FromUnstructured(obj, &node)
			if err != nil {
				return nil, err
			}

			delete(node.Labels, deprecatedEbpfSchedulingLabelKey)

			return sdk.ToUnstructured(&node)
		}, "v1", "Node", "", labeledNode.Name)
	}

	return nil
}
