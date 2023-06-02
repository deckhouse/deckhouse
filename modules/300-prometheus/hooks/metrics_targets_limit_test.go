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
	"bytes"
	"io"
	"net/http"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Before(f *HookExecutionConfig, body string) {
	BeforeEach(func() {
		// Mock HTTP client to emulate prom targets.
		buf := bytes.NewBufferString(body)
		rc := io.NopCloser(buf)
		dependency.TestDC.HTTPClient.DoMock.
			Expect(&http.Request{}).
			Return(&http.Response{
				Header:     map[string][]string{"Content-Type": {"application/json"}},
				StatusCode: http.StatusOK,
				Body:       rc,
			}, nil)
		f.KubeStateSet(``)
		f.RunHook()
	})
}

var _ = Describe("Modules :: prometheus :: hooks :: metrics_targets_limit ::", func() {
	const (
		nolimit = `
{
  "status": "success",
  "data": {
    "activeTargets": [
      {
        "labels": {
          "instance": "kube-state-metrics.d8-monitoring.svc.cluster.local.:8080",
          "job": "kube-state-metrics",
          "scrape_endpoint": "main"
        },
        "scrapePool": "kube-state-metrics/main",
        "lastError": ""
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
        "labels": {
          "instance": "kube-state-metrics.d8-monitoring.svc.cluster.local.:8080",
          "job": "kube-state-metrics",
          "scrape_endpoint": "main"
        },
        "scrapePool": "kube-state-metrics/main",
        "lastError": ""
      },
      {
        "labels": {
          "instance": "10.128.0.93:9100",
          "job": "custom-test2",
          "namespace": "default",
          "pod": "test-limit-7956c4c647-px85v"
        },
        "scrapePool": "podMonitor/d8-monitoring/custom-pod/0",
        "lastError": "sample limit exceeded"
      }
    ]
  }
}`
	)

	f := HookExecutionConfigInit(``, ``)

	Context("No targets with limits", func() {
		Before(f, nolimit)

		It("Hook must execute successfully", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(f).To(ExecuteSuccessfully())

			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "prometheus_target_limits_hook",
				Action: "expire",
			}))
		})
	})

	Context("No targets with limits", func() {
		Before(f, limit)

		It("Hook must execute successfully", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(f).To(ExecuteSuccessfully())

			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "prometheus_target_limits_hook",
				Action: "expire",
			}))

			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_prometheus_target_limits_metrics",
				Group:  "prometheus_target_limits_hook",
				Action: "set",
				Value:  pointer.Float64Ptr(1),
				Labels: map[string]string{
					"pod":        "test-limit-7956c4c647-px85v",
					"scrapePool": "podMonitor/d8-monitoring/custom-pod/0",
					"instance":   "10.128.0.93:9100",
					"job":        "custom-test2",
					"namespace":  "default",
				},
			}))
		})
	})
})
