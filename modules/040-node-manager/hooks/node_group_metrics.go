/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	nodeGroupMetricsGroup        = "node_group"
	nodeGroupMetricReadyName     = "d8_node_group_ready"
	nodeGroupMetricNodesName     = "d8_node_group_nodes"
	nodeGroupMetricInstancesName = "d8_node_group_instances"
	nodeGroupMetricDesiredName   = "d8_node_group_desired"
	nodeGroupMetricMinName       = "d8_node_group_min"
	nodeGroupMetricMaxName       = "d8_node_group_max"
	nodeGroupMetricUpToDateName  = "d8_node_group_up_to_date"
	nodeGroupMetricStandbyName   = "d8_node_group_standby"
	nodeGroupMetricHasErrorsName = "d8_node_group_has_errors"
)

type nodeGroupStatus struct {
	Name      string
	Ready     float64
	Nodes     float64
	Instances float64
	Desired   float64
	Min       float64
	Max       float64
	UpToDate  float64
	Standby   float64
	HasErrors float64
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/node_group_metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group_status",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: filterNodeGroupStatus,
		},
	},
}, handleNodeGroupStatus)

func filterNodeGroupStatus(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return nil, err
	}

	var hasErrors float64

	for _, condition := range nodeGroup.Status.Conditions {
		if condition.Type == ngv1.NodeGroupConditionTypeError && condition.Status == ngv1.ConditionTrue {
			hasErrors = 1
			break
		}
	}

	return nodeGroupStatus{
		Name:      nodeGroup.Name,
		Ready:     float64(nodeGroup.Status.Ready),
		Nodes:     float64(nodeGroup.Status.Nodes),
		Instances: float64(nodeGroup.Status.Instances),
		Desired:   float64(nodeGroup.Status.Desired),
		Min:       float64(nodeGroup.Status.Min),
		Max:       float64(nodeGroup.Status.Max),
		UpToDate:  float64(nodeGroup.Status.UpToDate),
		Standby:   float64(nodeGroup.Status.Standby),
		HasErrors: hasErrors,
	}, nil
}

func handleNodeGroupStatus(input *go_hook.HookInput) error {
	nodeGroupStatusSnapshots := input.Snapshots["node_group_status"]

	input.MetricsCollector.Expire(nodeGroupMetricsGroup)

	options := []metrics.Option{
		metrics.WithGroup(nodeGroupMetricsGroup),
	}

	for _, nodeGroupStatusSnapshot := range nodeGroupStatusSnapshots {
		nodeGroupStatus := nodeGroupStatusSnapshot.(nodeGroupStatus)

		labels := map[string]string{"node_group_name": nodeGroupStatus.Name}

		input.MetricsCollector.Set(nodeGroupMetricReadyName, nodeGroupStatus.Ready, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricNodesName, nodeGroupStatus.Nodes, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricInstancesName, nodeGroupStatus.Instances, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricDesiredName, nodeGroupStatus.Desired, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricMinName, nodeGroupStatus.Min, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricMaxName, nodeGroupStatus.Max, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricUpToDateName, nodeGroupStatus.UpToDate, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricStandbyName, nodeGroupStatus.Standby, labels, options...)

		input.MetricsCollector.Set(nodeGroupMetricHasErrorsName, nodeGroupStatus.HasErrors, labels, options...)
	}

	return nil
}
