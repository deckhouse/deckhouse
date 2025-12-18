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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: metrics_storage_retention ::", func() {

	f := HookExecutionConfigInit(`{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }, "retentionDays": 14, "longtermRetentionDays": 300}}`, `{}`)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(f).To(ExecuteSuccessfully())

			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "prometheus_disk_hook",
				Action: operation.ActionExpireMetrics,
			}))

			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_prometheus_storage_retention_days",
				Group:  "prometheus_disk_hook",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(14.0),
				Labels: map[string]string{
					"prometheus": "main",
				},
			}))

			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_prometheus_storage_retention_days",
				Group:  "prometheus_disk_hook",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(300.0),
				Labels: map[string]string{
					"prometheus": "longterm",
				},
			}))

		})
	})

})
