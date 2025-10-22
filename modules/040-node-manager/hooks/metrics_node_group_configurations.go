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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Count user defined NodeGroupConfigurations, aggregate them by NodeGroups and export as metric

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "configurations",
			ExecuteHookOnSynchronization: ptr.To(true),
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "NodeGroupConfiguration",
			FilterFunc:                   filterNGConfigurations,
		},
	},
}, handleNodeGroupConfigurations)

func filterNGConfigurations(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ngs, ok, err := unstructured.NestedStringSlice(obj.Object, "spec", "nodeGroups")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec.nodeGroups from NodeGroupConfiguration %s: %v", obj.GetName(), err)
	}

	if !ok {
		ngs = []string{"*"}
	}

	return nodeGroupConfigurationMetric{
		Name:       obj.GetName(),
		NodeGroups: ngs,
	}, nil
}

func handleNodeGroupConfigurations(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("configurations")

	input.MetricsCollector.Expire("node_group_configurations")

	if len(snaps) == 0 {
		return nil
	}
	countByNodeGroup := make(map[string]uint)
	for ngc, err := range sdkobjectpatch.SnapshotIter[nodeGroupConfigurationMetric](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'configurations' snapshots: %w", err)
		}

		for _, ng := range ngc.NodeGroups {
			countByNodeGroup[ng]++
		}
	}

	for ng, count := range countByNodeGroup {
		input.MetricsCollector.Set("d8_node_group_configurations_total", float64(count), map[string]string{"node_group": ng}, metrics.WithGroup("node_group_configurations"))
	}

	return nil
}

type nodeGroupConfigurationMetric struct {
	Name       string   `json:"name"`
	NodeGroups []string `json:"node_groups"`
}
