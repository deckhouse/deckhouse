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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-custom :: hooks :: metrics ::", func() {
	const (
		servicesResources = `
---
apiVersion: v1
kind: Service
metadata:
  name: my-service-1
spec:
  selector:
    app: MyApp1
  ports:
  - protocol: TCP
    port: 80
---
apiVersion: v1
kind: Service
metadata:
  name: my-service-2
  labels:
    foo: bar
spec:
  selector:
    app: MyApp2
  ports:
  - protocol: TCP
    port: 80
---
apiVersion: v1
kind: Service
metadata:
  name: my-service-3
  labels:
    prometheus-custom-target: myapp3
spec:
  selector:
    app: MyApp3
  ports:
  - protocol: TCP
    port: 80
---
apiVersion: v1
kind: Service
metadata:
  name: my-service-4
  labels:
    foo: bar
    bazz: wolrd
    prometheus-custom-target: myapp4
spec:
  selector:
    app: MyApp4
  ports:
  - protocol: TCP
    port: 80
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(4))

			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_service_monitor_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_pod_monitor_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_prometheus_rules_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_old_prometheus_custom_targets_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
		})
	})

	Context("Cluster containing some services", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(servicesResources))
			f.RunHook()
		})

		It("Hook must not fail, should get metric with 2 old Services with prometheus-custom-target label", func() {
			Expect(f).To(ExecuteSuccessfully())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(4))

			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_service_monitor_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_pod_monitor_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_unknown_prometheus_rules_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(0.0),
			}))
			Expect(ops[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_monitoring_custom_old_prometheus_custom_targets_total",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(2.0),
			}))
		})
	})

})
