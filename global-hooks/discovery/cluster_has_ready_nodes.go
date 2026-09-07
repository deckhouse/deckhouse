// Copyright 2026 Flant JSC
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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const clusterHasReadyNodesPath = "global.discovery.clusterHasReadyNodes"

const readyNodesSnapName = "ready_nodes"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       readyNodesSnapName,
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeReadyFilter,
		},
	},
}, setClusterHasReadyNodes)

func applyNodeReadyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return false, fmt.Errorf("from unstructured: %w", err)
	}

	for _, c := range node.Status.Conditions {
		if c.Type == v1core.NodeReady {
			return c.Status == v1core.ConditionTrue, nil
		}
	}

	return false, nil
}

func setClusterHasReadyNodes(_ context.Context, input *go_hook.HookInput) error {
	ready, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, readyNodesSnapName)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", readyNodesSnapName, err)
	}

	for _, isReady := range ready {
		if isReady {
			input.Values.Set(clusterHasReadyNodesPath, true)
			return nil
		}
	}

	input.Values.Set(clusterHasReadyNodesPath, false)

	return nil
}
