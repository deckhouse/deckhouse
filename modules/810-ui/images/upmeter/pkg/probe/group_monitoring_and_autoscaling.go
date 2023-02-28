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

package probe

import (
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/node"
	"d8.io/upmeter/pkg/probe/checker"
)

func initMonitoringAndAutoscaling(access kubernetes.Access, nodeLister node.Lister, preflight checker.Doer) []runnerConfig {
	const (
		groupMonitoringAndAutoscaling = "monitoring-and-autoscaling"
		controlPlaneTimeout           = 5 * time.Second
	)

	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	return []runnerConfig{
		{
			group:  groupMonitoringAndAutoscaling,
			probe:  "prometheus",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "prometheus=main",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "prometheus",
			check:  "api",
			period: 10 * time.Second,
			config: checker.PrometheusApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query?query=vector(1)",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "trickster",
			check:  "pod",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "app=trickster",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "trickster",
			check:  "api",
			period: 10 * time.Second,
			config: checker.PrometheusApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://trickster.d8-monitoring:443/trickster/main/api/v1/query?query=vector(1)",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "prometheus-metrics-adapter",
			check:  "pod",
			period: 5 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "app=prometheus-metrics-adapter",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "prometheus-metrics-adapter",
			check:  "api",
			period: 5 * time.Second,
			config: checker.MetricsAdapterApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://kubernetes.default/apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-updater",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "kube-system",
				LabelSelector:    "app=vpa-updater",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-recommender",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "kube-system",
				LabelSelector:    "app=vpa-recommender",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-admission-controller",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "kube-system",
				LabelSelector:    "app=vpa-admission-controller",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "metrics-sources",
			check:  "node-exporter",
			period: 10 * time.Second,
			config: checker.DaemonSetPodsReady{
				Access:             access,
				NodeLister:         nodeLister,
				Namespace:          "d8-monitoring",
				Name:               "node-exporter",
				RequestTimeout:     5 * time.Second,
				PodCreationTimeout: time.Minute,
				PodDeletionTimeout: 5 * time.Second,
				PreflightChecker:   controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "metrics-sources",
			check:  "kube-state-metrics",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:           access,
				Timeout:          5 * time.Second,
				Namespace:        "d8-monitoring",
				LabelSelector:    "app=kube-state-metrics",
				PreflightChecker: controlPlanePinger,
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "key-metrics-present",
			check:  "kube-state-metrics",
			period: 15 * time.Second,
			config: checker.MetricPresentInPrometheus{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query",
				Metric:   "kube_state_metrics_list_total",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "key-metrics-present",
			check:  "node-exporter",
			period: 15 * time.Second,
			config: checker.MetricPresentInPrometheus{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query",
				Metric:   "node_exporter_build_info",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "key-metrics-present",
			check:  "kubelet",
			period: 15 * time.Second,
			config: checker.MetricPresentInPrometheus{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query",
				Metric:   "kubelet_node_name",
			},
		}, {
			group:  groupMonitoringAndAutoscaling,
			probe:  "key-metrics-present",
			check:  "kubelet",
			period: 15 * time.Second,
			config: checker.MetricPresentInPrometheus{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query",
				Metric:   "kubelet_node_name",
			},
		},
	}
}
