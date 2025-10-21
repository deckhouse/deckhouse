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
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "nodes",
			WaitForSynchronization: ptr.To(false),
			ApiVersion:             "v1",
			Kind:                   "Node",
			FilterFunc:             setProviderIDNodeFilter,
		},
	},
}, handleSetProviderID)

func setProviderIDNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var needPatch bool

	var hasUninitializedTaint bool
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.cloudprovider.kubernetes.io/uninitialized" {
			hasUninitializedTaint = true
			break
		}
	}

	if !hasUninitializedTaint && node.Spec.ProviderID == "" && node.Labels["node.deckhouse.io/type"] == "Static" {
		needPatch = true
	}

	return providerIDNode{
		Name:      node.Name,
		NeedPatch: needPatch,
	}, nil
}

type providerIDNode struct {
	Name      string
	NeedPatch bool
}

func handleSetProviderID(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("nodes")
	for node, err := range sdkobjectpatch.SnapshotIter[providerIDNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'node' snapshots: %w", err)
		}

		if !node.NeedPatch {
			continue
		}

		input.PatchCollector.PatchWithMerge(staticPatch, "v1", "Node", "", node.Name)
	}

	return nil
}

var (
	staticPatch = map[string]interface{}{
		"spec": map[string]string{
			"providerID": "static://",
		},
	}
)
