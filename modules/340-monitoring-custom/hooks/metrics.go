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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service_monitors",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "ServiceMonitor",
			FilterFunc: applyNameFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus": "main",
				},
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpNotIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				LabelSelector: &v1.LabelSelector{
					MatchExpressions: []v1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: v1.LabelSelectorOpIn,
							Values: []string{
								"deckhouse",
							},
						},
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: v1.LabelSelectorOpNotIn,
							Values: []string{
								"d8-observability",
							},
						},
					},
				},
			},
		},
		{
			Name:       "pod_monitors",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "PodMonitor",
			FilterFunc: applyNameFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus": "main",
				},
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpNotIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				LabelSelector: &v1.LabelSelector{
					MatchExpressions: []v1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: v1.LabelSelectorOpIn,
							Values: []string{
								"deckhouse",
							},
						},
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: v1.LabelSelectorOpNotIn,
							Values: []string{
								"d8-observability",
							},
						},
					},
				},
			},
		},
		{
			Name:       "rules",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
			FilterFunc: applyNameFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus": "main",
					"component":  "rules",
				},
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpNotIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				LabelSelector: &v1.LabelSelector{
					MatchExpressions: []v1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: v1.LabelSelectorOpIn,
							Values: []string{
								"deckhouse",
							},
						},
						{
							Key:      "kubernetes.io/metadata.name",
							Operator: v1.LabelSelectorOpNotIn,
							Values: []string{
								"d8-observability",
							},
						},
					},
				},
			},
		},
		{
			Name:       "custom_services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyNameFilter,
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "prometheus-custom-target",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
		},
	},
}, exposeMetrics)

func exposeMetrics(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Set("d8_monitoring_custom_unknown_service_monitor_total", float64(len(input.Snapshots.Get("service_monitors"))), nil)
	input.MetricsCollector.Set("d8_monitoring_custom_unknown_pod_monitor_total", float64(len(input.Snapshots.Get("pod_monitors"))), nil)
	input.MetricsCollector.Set("d8_monitoring_custom_unknown_prometheus_rules_total", float64(len(input.Snapshots.Get("rules"))), nil)
	input.MetricsCollector.Set("d8_monitoring_custom_old_prometheus_custom_targets_total", float64(len(input.Snapshots.Get("custom_services"))), nil)

	return nil
}
