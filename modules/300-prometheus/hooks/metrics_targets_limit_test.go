/*
Copyright 2022 Flant JSC

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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: metrics_targets_limit ::", func() {
	const (
		nolimit = `
{
  "status": "success",
  "data": {
    "activeTargets": [
      {
        "discoveredLabels": {
          "__address__": "kube-state-metrics.d8-monitoring.svc.cluster.local.:8080",
          "__metrics_path__": "/main/metrics",
          "__scheme__": "https",
          "__scrape_interval__": "30s",
          "__scrape_timeout__": "20s",
          "job": "kube-state-metrics/main"
        },
        "labels": {
          "instance": "kube-state-metrics.d8-monitoring.svc.cluster.local.:8080",
          "job": "kube-state-metrics",
          "scrape_endpoint": "main"
        },
        "scrapePool": "kube-state-metrics/main",
        "scrapeUrl": "https://kube-state-metrics.d8-monitoring.svc.cluster.local.:8080/main/metrics",
        "globalUrl": "https://kube-state-metrics.d8-monitoring.svc.cluster.local.:8080/main/metrics",
        "lastError": "",
        "lastScrape": "2023-06-02T16:37:52.654045394Z",
        "lastScrapeDuration": 0.028524734,
        "health": "up",
        "scrapeInterval": "30s",
        "scrapeTimeout": "20s"
      }
    ]
  }
}`
		limit = `
{
  "status": "success",
  "data": {
    "activeTargets": [
      {
        "discoveredLabels": {
          "__address__": "10.128.0.93:9100",
          "__meta_kubernetes_namespace": "default",
          "__meta_kubernetes_pod_annotation_prometheus_deckhouse_io_sample_limit": "1",
          "__meta_kubernetes_pod_annotationpresent_prometheus_deckhouse_io_sample_limit": "true",
          "__meta_kubernetes_pod_container_id": "containerd://5cddb9ab75b6fa9e0f0b9d6d9f65fa7b6ff724a2fca8832ed515f5117f7ab79c",
          "__meta_kubernetes_pod_container_image": "quay.io/prometheus/node-exporter:latest",
          "__meta_kubernetes_pod_container_init": "false",
          "__meta_kubernetes_pod_container_name": "b",
          "__meta_kubernetes_pod_container_port_name": "http-metrics",
          "__meta_kubernetes_pod_container_port_number": "9100",
          "__meta_kubernetes_pod_container_port_protocol": "TCP",
          "__meta_kubernetes_pod_controller_kind": "ReplicaSet",
          "__meta_kubernetes_pod_controller_name": "test-limit-7956c4c647",
          "__meta_kubernetes_pod_host_ip": "10.241.32.24",
          "__meta_kubernetes_pod_ip": "10.128.0.93",
          "__meta_kubernetes_pod_label_app": "test2",
          "__meta_kubernetes_pod_label_pod_template_hash": "7956c4c647",
          "__meta_kubernetes_pod_label_prometheus_deckhouse_io_custom_target": "test2",
          "__meta_kubernetes_pod_labelpresent_app": "true",
          "__meta_kubernetes_pod_labelpresent_pod_template_hash": "true",
          "__meta_kubernetes_pod_labelpresent_prometheus_deckhouse_io_custom_target": "true",
          "__meta_kubernetes_pod_name": "test-limit-7956c4c647-px85v",
          "__meta_kubernetes_pod_node_name": "test-pr4795-master-0",
          "__meta_kubernetes_pod_phase": "Running",
          "__meta_kubernetes_pod_ready": "true",
          "__meta_kubernetes_pod_uid": "9e1dce3e-68a7-496a-bf77-fe2b825e0344",
          "__metrics_path__": "/metrics",
          "__scheme__": "http",
          "__scrape_interval__": "30s",
          "__scrape_timeout__": "10s",
          "job": "podMonitor/d8-monitoring/custom-pod/0"
        },
        "labels": {
          "instance": "10.128.0.93:9100",
          "job": "custom-test2",
          "namespace": "default",
          "pod": "test-limit-7956c4c647-px85v"
        },
        "scrapePool": "podMonitor/d8-monitoring/custom-pod/0",
        "scrapeUrl": "http://10.128.0.93:9100/metrics",
        "globalUrl": "http://10.128.0.93:9100/metrics",
        "lastError": "sample limit exceeded",
        "lastScrape": "2023-06-02T16:38:04.707402782Z",
        "lastScrapeDuration": 0.013133037,
        "health": "down",
        "scrapeInterval": "30s",
        "scrapeTimeout": "10s"
      }
    ]
  }
}`
	)

	f := HookExecutionConfigInit(``, ``)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		f.BindingContexts.Set(f.GenerateScheduleContext("0 * * * *"))

		It("Hook must execute successfully", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(f).To(ExecuteSuccessfully())

			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "prometheus_disk_hook",
				Action: "expire",
			}))

			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_prometheus_storage_retention_days",
				Group:  "prometheus_disk_hook",
				Action: "set",
				Value:  pointer.Float64Ptr(14.0),
				Labels: map[string]string{
					"prometheus": "main",
				},
			}))

			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_prometheus_storage_retention_days",
				Group:  "prometheus_disk_hook",
				Action: "set",
				Value:  pointer.Float64Ptr(300.0),
				Labels: map[string]string{
					"prometheus": "longterm",
				},
			}))

		})
	})

})
