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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	d8v1alpha1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1"
)

const (
	nodeMetricName     = "d8_node_status"
	instanceMetricName = "d8_instance_status"
	metricsGroup       = "node_instance_metrics"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/node_instance_metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: nodeMetricsNodeFilter,
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
		},
		{
			Name:       "instances",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Instance",
			FilterFunc: nodeMetricsinstanceFilter,
		},
	},
}, handleNodeInstanceMetrics)

func nodeMetricsNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	isReady := "NotReady"

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			isReady = "Ready"
			break
		}
	}

	return nodeStatus{
		Name:   node.Name,
		Status: isReady,
	}, nil
}

func nodeMetricsinstanceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var instance d8v1alpha1.Instance
	err := sdk.FromUnstructured(obj, &instance)
	if err != nil {
		return nil, err
	}
	return instanceStatus{
		Name:    instance.Name,
		Status:  string(instance.Status.CurrentStatus.Phase),
		NodeRef: instance.Status.NodeRef.Name,
	}, nil
}

type nodeStatus struct {
	Name   string
	Status string
}

type instanceStatus struct {
	Name    string
	Status  string
	NodeRef string
}

func handleNodeInstanceMetrics(input *go_hook.HookInput) error {
	nodeSnapshots := input.Snapshots["nodes"]
	instanceSnapshots := input.Snapshots["instances"]

	input.MetricsCollector.Expire(metricsGroup)

	options := []metrics.Option{
		metrics.WithGroup(metricsGroup),
	}

	for _, snap := range nodeSnapshots {
		node := snap.(nodeStatus)
		labels := map[string]string{"node_name": node.Name, "status": node.Status}
		input.MetricsCollector.Set(nodeMetricName, 1, labels, options...)
	}

	for _, snap := range instanceSnapshots {
		instance := snap.(instanceStatus)
		labels := map[string]string{"instance_name": instance.Name, "status": instance.Status, "node_name": instance.NodeRef}
		input.MetricsCollector.Set(instanceMetricName, 1, labels, options...)
	}

	return nil
}
