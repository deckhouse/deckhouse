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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "all_nodes",
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			WaitForSynchronization:       ptr.To(true),
			FilterFunc:                   applyNodeFilter,
		},
	},
}, handleSetNodesCount)

type nodeInfo struct {
	Name string
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return nodeInfo{
		Name: node.Name,
	}, nil
}

func handleSetNodesCount(_ context.Context, input *go_hook.HookInput) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[nodeInfo](input.Snapshots, "all_nodes")
	if err != nil {
		return fmt.Errorf("failed to unmarshal all_nodes snapshot: %w", err)
	}

	nodesCount := len(nodes)

	input.Values.Set("controlPlaneManager.internal.nodesCount", nodesCount)

	return nil
}
