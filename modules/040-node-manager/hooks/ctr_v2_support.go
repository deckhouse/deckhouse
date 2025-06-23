/*
Copyright 2023 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	containerdV2SupportLabel = "node.deckhouse.io/containerd-v2-unsupported"
	cgroupV2MetricName       = "d8_node_cgroup_v2_support_status"
)

// set d8_node_cgroup_v2_support_status=1 if node has label node.deckhouse.io/containerd-v2-unsupported
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 60 * time.Second,
	},
	Queue: "/modules/node-manager/cgroupv2_support_metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes_with_group",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      nodeGroupLabel,
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: filterNodeForCgroupV2Support,
		},
	},
}, handleCgroupV2SupportMetrics)

type cgroupV2SupportNode struct {
	Name      string
	NodeGroup string
}

func filterNodeForCgroupV2Support(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	nodeGroup := node.Labels[nodeGroupLabel]

	return cgroupV2SupportNode{
		Name:      node.Name,
		NodeGroup: nodeGroup,
	}, nil
}

func handleCgroupV2SupportMetrics(input *go_hook.HookInput) error {
	snap := input.Snapshots["nodes_with_group"]

	for _, s := range snap {
		nodeInfo := s.(cgroupV2SupportNode)

		var metricValue float64 = 1.0

		labels := map[string]string{
			"node":       nodeInfo.Name,
			"node_group": nodeInfo.NodeGroup,
		}

		input.MetricsCollector.Set(cgroupV2MetricName, metricValue, labels)
	}

	return nil
}
