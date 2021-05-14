package probe

import (
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func MonitoringAndAutoscaling(access kubernetes.Access) []runnerConfig {
	const (
		groupName = "monitoring-and-autoscaling"
	)

	return []runnerConfig{
		{
			group:  groupName,
			probe:  "prometheus",
			check:  "pods",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "d8-monitoring",
				LabelSelector: "app=prometheus",
			},
		}, {
			group:  groupName,
			probe:  "prometheus",
			check:  "api",
			period: 10 * time.Second,
			config: checker.PrometheusApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://prometheus.d8-monitoring:9090/api/v1/query?query=vector(1)",
			},
		}, {
			group:  groupName,
			probe:  "trickster",
			check:  "pods",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "d8-monitoring",
				LabelSelector: "app=trickster",
			},
		}, {
			group:  groupName,
			probe:  "trickster",
			check:  "api",
			period: 10 * time.Second,
			config: checker.PrometheusApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://trickster.d8-monitoring:443/trickster/main/api/v1/query?query=vector(1)",
			},
		}, {
			group:  groupName,
			probe:  "prometheus-metrics-adapter",
			check:  "pods",
			period: 5 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "d8-monitoring",
				LabelSelector: "app=prometheus-metrics-adapter",
			},
		}, {
			group:  groupName,
			probe:  "prometheus-metrics-adapter",
			check:  "api",
			period: 5 * time.Second,
			config: checker.MetricsAdapterApiAvailable{
				Access:   access,
				Timeout:  5 * time.Second,
				Endpoint: "https://kubernetes.default/apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m",
			},
		}, {
			group:  groupName,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-updater",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "kube-system",
				LabelSelector: "app=vpa-updater",
			},
		}, {
			group:  groupName,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-recommender",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "kube-system",
				LabelSelector: "app=vpa-recommender",
			},
		}, {
			group:  groupName,
			probe:  "vertical-pod-autoscaler",
			check:  "vpa-admission-controller",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "kube-system",
				LabelSelector: "app=vpa-admission-controller",
			},
		}, {
			group:  groupName,
			probe:  "metrics-sources",
			check:  "node-exporter",
			period: 10 * time.Second,
			config: checker.DaemonSetPodsReady{
				Access:    access,
				Timeout:   5 * time.Second,
				Namespace: "d8-monitoring",
				Name:      "node-exporter",
			},
		}, {
			group:  groupName,
			probe:  "metrics-sources",
			check:  "kube-state-metrics",
			period: 10 * time.Second,
			config: checker.AtLeastOnePodReady{
				Access:        access,
				Timeout:       5 * time.Second,
				Namespace:     "d8-monitoring",
				LabelSelector: "app=kube-state-metrics",
			},
		}, {
			group:  groupName,
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
			group:  groupName,
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
			group:  groupName,
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
