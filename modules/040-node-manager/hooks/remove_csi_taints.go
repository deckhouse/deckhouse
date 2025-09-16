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
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},

	Queue: "/modules/node-manager/remove_csi_taints",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "csinodes",
			WaitForSynchronization: ptr.To(false),
			ApiVersion:             "storage.k8s.io/v1",
			Kind:                   "CSINode",
			FilterFunc:             csiFilterCSINode, //  jqFilter: '{"name": .metadata.name}'
		},
		{
			Name:                         "nodes",
			WaitForSynchronization:       ptr.To(false),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   csiFilterNode, // '{"needPatch": ([(.spec.taints // [])[] | select(.key == "node.deckhouse.io/csi-not-bootstrapped")] | length > 0), "name": .metadata.name}',
		},
	},
}, handleRemoveCSI)

func csiFilterCSINode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var csiNode storagev1.CSINode

	err := sdk.FromUnstructured(obj, &csiNode)
	if err != nil {
		return nil, err
	}

	if len(csiNode.Spec.Drivers) == 0 {
		return "", nil
	}

	return csiNode.Name, nil
}
func csiFilterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var needPatch bool
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.deckhouse.io/csi-not-bootstrapped" {
			needPatch = true
			break
		}
	}

	return removeCSINode{
		Name:      node.Name,
		NeedPatch: needPatch,
	}, nil
}

type removeCSINode struct {
	Name      string
	NeedPatch bool
}

func handleRemoveCSI(_ context.Context, input *go_hook.HookInput) error {
	nodes := make(map[string]bool)
	snaps := input.Snapshots.Get("nodes")
	for node, err := range sdkobjectpatch.SnapshotIter[removeCSINode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
		}

		nodes[node.Name] = node.NeedPatch
	}

	snaps = input.Snapshots.Get("csinodes")
	for csiName, err := range sdkobjectpatch.SnapshotIter[string](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
		}

		needPatch, ok := nodes[csiName]
		if !ok {
			continue
		}
		if !needPatch {
			continue
		}

		input.PatchCollector.PatchWithMutatingFunc(removeCSIFilterNode, "v1", "Node", "", csiName)
	}

	return nil
}

func removeCSIFilterNode(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var node *v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	taints := make([]v1.Taint, 0)

	for _, taint := range node.Spec.Taints {
		if taint.Key != "node.deckhouse.io/csi-not-bootstrapped" {
			taints = append(taints, taint)
		}
	}

	node.Spec.Taints = taints

	return sdk.ToUnstructured(node)
}
