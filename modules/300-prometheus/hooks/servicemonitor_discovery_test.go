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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks ::  service_monitors_discovery ::", func() {
	f := HookExecutionConfigInit(
		`{"prometheus":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)
	f.RegisterCRD("monitoring.coreos.com", "v1", "ServiceMonitor", true)

	Context("ServiceMonitor with only endpoints label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: test1
  namespace: d8-monitoring
  labels:
    prometheus: main
spec:
  endpoints:
    - port: self
      relabelings:
        - sourceLabels: [ __meta_kubernetes_endpoint_ready ]
          regex: "true"
          action: keep
`))
			f.RunHook()
		})

		It("Must generate an alert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Name).To(Equal("d8_prometheus_deprecated_servicemonitor"))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Value).To(Equal(ptr.To(1.0)))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Labels).To(Equal(map[string]string{"name": "test1", "namespace": "d8-monitoring"}))
		})
	})

	Context("ServiceMonitor with only endpoints prefix label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: test2
  namespace: testns
  labels:
    prometheus: main
spec:
  endpoints:
    - port: self
      relabelings:
        - sourceLabels: [ __meta_kubernetes_endpoints_labelpresent_foobar ]
          regex: "true"
          action: keep
`))
			f.RunHook()
		})

		It("Must generate an alert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Name).To(Equal("d8_prometheus_deprecated_servicemonitor"))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Value).To(Equal(ptr.To(1.0)))
			Expect(f.MetricsCollector.CollectedMetrics()[1].Labels).To(Equal(map[string]string{"name": "test2", "namespace": "testns"}))
		})
	})

	Context("ServiceMonitor with only endpointslice label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: test2
  namespace: testns
  labels:
    prometheus: main
spec:
  endpoints:
    - port: self
      relabelings:
        - sourceLabels: [ __meta_kubernetes_endpointslice_endpoint_conditions_ready ]
          regex: "true"
          action: keep
`))
			f.RunHook()
		})

		It("Must not generate an alert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})

	Context("ServiceMonitor with both labels", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: test3
  namespace: d8-monitoring
  labels:
    prometheus: main
spec:
  endpoints:
    - port: self
      relabelings:
        - sourceLabels: [ __meta_kubernetes_endpointslice_endpoint_conditions_ready ]
          regex: "true"
          action: keep
        - sourceLabels: [ __meta_kubernetes_endpoints_labelpresent_foobar, __meta_kubernetes_endpointslice_labelpresent_foobar ]
          regex: ".*true.*"
          action: keep
`))
			f.RunHook()
		})

		It("Must have no alerts", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))
		})
	})
})
