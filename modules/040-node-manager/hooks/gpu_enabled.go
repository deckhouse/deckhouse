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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	gpuEnabledLabel   = "node.deckhouse.io/gpu"
	devicePluginLabel = "nvidia.com/device-plugin.config"
	ngLabel           = "node.deckhouse.io/group"
)

// This hook discovers nodegroup names for dynamic probes in upmeter
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
			},
		},
	},
	setGPULabel,
)

type gpuSpecNG struct {
	Name    string
	Sharing string
}

// filterGPUSpec returns the name of a nodegroup to consider or emptystring if it should be skipped
func filterGPUSpec(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return "", err
	}

	// Filter only GPU node groups
	if nodeGroup.Spec.GPU.Sharing == "" {
		return "", nil
	}

	return nodeGroup, nil
}

// collectDynamicProbeConfig sets names of objects to internal values
func setGPULabel(input *go_hook.HookInput) error {
	// Input
	ngs := input.Snapshots["nodegroups"]
	nodes := input.Snapshots["nodes"]

	for _, ng := range ngs {
		ngName := ng.(gpuSpecNG).Name
		sharing := ng.(gpuSpecNG).Sharing
		input.Logger.Info("Processing nodegroup %s", ngName)

		for _, node := range nodes {
			nodeName := node.(*v1.Node).Name
			node := node.(*v1.Node)
			if node.Labels[gpuEnabledLabel] == ngName {
				_, isLabeled := node.Labels[gpuEnabledLabel]
				if isLabeled {
					continue
				}
				input.Logger.Info("Labeling %s node with %s=%v label", nodeName, devicePluginLabel, sharing)
				metadata := map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							gpuEnabledLabel:   "",
							devicePluginLabel: sharing,
						},
					},
				}
				input.PatchCollector.PatchWithMerge(metadata, "v1", "Node", "", nodeName)
			}
		}
	}
	return nil
}
