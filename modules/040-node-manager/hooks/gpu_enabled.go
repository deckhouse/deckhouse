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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	gpuEnabledLabel   = "node.deckhouse.io/gpu"
	devicePluginLabel = "node.deckhouse.io/device-gpu.config"
	ngLabel           = "node.deckhouse.io/group"
	migConfigLabel    = "nvidia.com/mig.config"
	migDisabled       = "all-disabled"
)

// This hook discovers nodegroup GPU sharing type and labels nodes
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue:       "/modules/node-manager",
		OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "nodegroups",
				ApiVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
				FilterFunc: filterGPUSpec,
			},
			{
				Name:       "nodes",
				ApiVersion: "v1",
				Kind:       "Node",
				FilterFunc: nodeFilterFunc,
			},
		},
	},
	setGPULabel)

type nodeGroupInfo struct {
	Name       string
	GpuSharing string
	MIGConfig  *string
}

type NodeInfo struct {
	Name   string
	Labels map[string]string
}

func filterGPUSpec(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return "", err
	}

	ngi := nodeGroupInfo{
		Name:       nodeGroup.Name,
		GpuSharing: nodeGroup.Spec.GPU.Sharing,
	}

	if nodeGroup.Spec.GPU.Mig != nil && nodeGroup.Spec.GPU.Mig.PartedConfig != nil {
		ngi.MIGConfig = nodeGroup.Spec.GPU.Mig.PartedConfig
	}

	return ngi, nil
}

func nodeFilterFunc(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return "", err
	}

	return NodeInfo{
		Name:   node.Name,
		Labels: node.Labels,
	}, nil
}

func setGPULabel(_ context.Context, input *go_hook.HookInput) error {
	ngs := input.Snapshots.Get("nodegroups")
	nodes := input.Snapshots.Get("nodes")

	for _, ngSnapshot := range ngs {
		var ng nodeGroupInfo
		err := ngSnapshot.UnmarshalTo(&ng)
		if err != nil {
			return err
		}
		if ng.GpuSharing == "" {
			continue
		}
		input.Logger.Info("Processing GPU nodegroup %s", ng.Name)

		for _, nodeSnapshot := range nodes {
			var node NodeInfo
			err := nodeSnapshot.UnmarshalTo(&node)
			if err != nil {
				return err
			}
			if _, ok := node.Labels[ngLabel]; ok {
				if node.Labels[ngLabel] != ng.Name {
					continue
				}
			}

			labels := map[string]interface{}{}

			if ng.MIGConfig != nil {
				labels[migConfigLabel] = ng.MIGConfig
			} else {
				// remove MIG label if it's set and it's not a MIG node
				if _, ok := node.Labels[migConfigLabel]; ok {
					labels[migConfigLabel] = migDisabled
				}
			}

			if _, ok := node.Labels[gpuEnabledLabel]; ok {
				if sharingType, ok := node.Labels[devicePluginLabel]; ok {
					if sharingType != ng.GpuSharing {
						labels[devicePluginLabel] = ng.GpuSharing
					}
				}
			} else {
				labels[gpuEnabledLabel] = ""
				labels[devicePluginLabel] = ng.GpuSharing
			}

			input.Logger.Info("Labeling %s node with %s=%v label", node.Name, devicePluginLabel, ng.GpuSharing)

			metadata := map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels,
				},
			}

			input.PatchCollector.PatchWithMerge(metadata, "v1", "Node", "", node.Name)
		}
	}
	return nil
}
