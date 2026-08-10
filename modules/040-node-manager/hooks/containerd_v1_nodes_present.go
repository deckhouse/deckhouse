/*
Copyright 2026 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	containerdV1NodesPresentValuesKey = "nodeManager:containerdV1NodesPresent"
	containerdRuntimePrefix           = "containerd://"
)

// set nodeManager:containerdV1NodesPresent=true if at least one node is running containerd v1.x
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "containerd_v1_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: filterNodeContainerRuntimeVersion,
		},
	},
}, handleContainerdV1NodesPresent)

func filterNodeContainerRuntimeVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, err
	}

	return node.Status.NodeInfo.ContainerRuntimeVersion, nil
}

func handleContainerdV1NodesPresent(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("containerd_v1_nodes")

	hasContainerdV1Nodes := false
	for runtimeVersion, err := range sdkobjectpatch.SnapshotIter[string](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'containerd_v1_nodes' snapshot: %w", err)
		}

		if isContainerdV1(runtimeVersion) {
			hasContainerdV1Nodes = true
			break
		}
	}

	requirements.SaveValue(containerdV1NodesPresentValuesKey, hasContainerdV1Nodes)

	return nil
}

// isContainerdV1 reports whether the node's container runtime is containerd v1.x,
// e.g. "containerd://1.7.13" (true) vs "containerd://2.0.1" (false).
func isContainerdV1(runtimeVersion string) bool {
	version, ok := strings.CutPrefix(runtimeVersion, containerdRuntimePrefix)
	if !ok {
		return false
	}

	major, _, _ := strings.Cut(version, ".")

	return major == "1"
}
