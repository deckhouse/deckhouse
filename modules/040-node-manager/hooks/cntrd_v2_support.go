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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	containerdV2SupportLabel = "node.deckhouse.io/containerd-v2-unsupported"
	cntrdV2GroupName         = "nodes_cntrd_v2"
)

var cntrdV2UnsupportedMetricName = fmt.Sprintf("d8_%s_unsupported", cntrdV2GroupName)

// set nodes_cntrdv2_unsupported=1 if node has label node.deckhouse.io/containerd-v2-unsupported
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "sync",
			Crontab: "*/3 * * * *",
		},
	},
	Queue: "/modules/node-manager/nodes_cntrdv2_unsupported_metric",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes_cntrdv2_unsupported",
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
}, handlecntrdV2SupportMetrics)

type cgroupV2SupportNode struct {
	Name                string
	NodeGroup           string
	HasUnsupportedLabel bool
}

func filterNodeForCgroupV2Support(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	nodeGroup := node.Labels[nodeGroupLabel]
	_, hasLabel := node.Labels[containerdV2SupportLabel]

	return cgroupV2SupportNode{
		Name:                node.Name,
		NodeGroup:           nodeGroup,
		HasUnsupportedLabel: hasLabel,
	}, nil
}

func handlecntrdV2SupportMetrics(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("nodes_cntrdv2_unsupported")
	input.MetricsCollector.Expire(cntrdV2GroupName)
	options := []sdkpkg.MetricCollectorOption{
		metrics.WithGroup(cntrdV2GroupName),
	}
	for nodeInfo, err := range sdkobjectpatch.SnapshotIter[cgroupV2SupportNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_cntrdv2_unsupported snapshot': %w", err)
		}

		metricValue := 1.0
		if nodeInfo.HasUnsupportedLabel {
			labels := map[string]string{
				"node":       nodeInfo.Name,
				"node_group": nodeInfo.NodeGroup,
			}
			input.MetricsCollector.Set(cntrdV2UnsupportedMetricName, metricValue, labels, options...)
		}
	}

	return nil
}
