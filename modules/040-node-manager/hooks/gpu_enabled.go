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

var removeMigLabel = map[string]interface{}{
	"metadata": map[string]interface{}{
		"labels": map[string]interface{}{
			migConfigLabel: nil,
		},
	},
}

func setGPULabel(input *go_hook.HookInput) error {
	ngs := input.Snapshots["nodegroups"]
	nodes := input.Snapshots["nodes"]

	for _, ng := range ngs {
		ngName := ng.(nodeGroupInfo).Name
		gpuSharing := ng.(nodeGroupInfo).GpuSharing
		if gpuSharing == "" {
			continue
		}
		input.Logger.Info("Processing GPU nodegroup %s", ngName)

		for _, node := range nodes {
			if _, ok := node.(NodeInfo).Labels[ngLabel]; ok {
				if node.(NodeInfo).Labels[ngLabel] != ngName {
					continue
				}
			}

			labels := map[string]interface{}{
				gpuEnabledLabel:   "",
				devicePluginLabel: gpuSharing,
			}

			if ng.(nodeGroupInfo).MIGConfig != nil {
				labels[migConfigLabel] = ng.(nodeGroupInfo).MIGConfig
			} else {
				// remove MIG label if it's set and it's not a MIG node
				if _, ok := node.(NodeInfo).Labels[migConfigLabel]; ok {
					input.PatchCollector.PatchWithMerge(removeMigLabel, "v1", "Node", "", node.(NodeInfo).Name)
				}
			}

			if _, ok := node.(NodeInfo).Labels[gpuEnabledLabel]; ok {
				if sharingType, ok := node.(NodeInfo).Labels[devicePluginLabel]; ok {
					if sharingType == gpuSharing {
						continue
					}
				}
			}

			input.Logger.Info("Labeling %s node with %s=%v label", node.(NodeInfo).Name, devicePluginLabel, gpuSharing)

			metadata := map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels,
				},
			}

			input.PatchCollector.PatchWithMerge(metadata, "v1", "Node", "", node.(NodeInfo).Name)
		}
	}
	return nil
}
