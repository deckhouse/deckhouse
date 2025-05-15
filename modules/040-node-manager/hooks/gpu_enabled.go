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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	gpuEnabledLabel   = "node.deckhouse.io/gpu"
	devicePluginLabel = "node.deckhouse.io/device-gpu.config"
	ngLabel           = "node.deckhouse.io/group"
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
		},
	},
	dependency.WithExternalDependencies(setGPULabel))

type nodeGroupInfo struct {
	Name       string
	GpuSharing string
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

	return nodeGroupInfo{
		Name:       nodeGroup.Name,
		GpuSharing: nodeGroup.Spec.GPU.Sharing,
	}, nil
}

func setGPULabel(input *go_hook.HookInput, dc dependency.Container) error {
	ngs := input.Snapshots["nodegroups"]

	for _, ng := range ngs {
		var nodes *v1.NodeList
		ngName := ng.(nodeGroupInfo).Name
		gpuSharing := ng.(nodeGroupInfo).GpuSharing
		if gpuSharing == "" {
			continue
		}
		input.Logger.Info("Processing GPU nodegroup %s", ngName)

		kubeClient := dc.MustGetK8sClient()

		nodes, _ = kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "node.deckhouse.io/group=" + ngName,
		})

		for _, node := range nodes.Items {
			if _, ok := node.Labels[gpuEnabledLabel]; ok {
				if sharingType, ok := node.Labels[devicePluginLabel]; ok {
					if sharingType == gpuSharing {
						continue
					}
				}
			}

			input.Logger.Info("Labeling %s node with %s=%v label", node.Name, devicePluginLabel, gpuSharing)
			metadata := map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						gpuEnabledLabel:   "",
						devicePluginLabel: gpuSharing,
					},
				},
			}

			input.PatchCollector.PatchWithMerge(metadata, "v1", "Node", "", node.Name)
		}
	}
	return nil
}
