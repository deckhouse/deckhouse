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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	serviceWithDeprecatedAnnotation = `---
apiVersion: v1
kind: Service
metadata:
  name: test
  namespace: test
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
`

	serviceWithoutDeprecatedAnnotation = `---
apiVersion: v1
kind: Service
metadata:
  name: test
  namespace: test
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
`

	serviceWithDeprecatedAnnotationAndWithSpecField = `---
apiVersion: v1
kind: Service
metadata:
  name: test
  namespace: test
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
spec:
  publishNotReadyAddresses: true
`
)

var _ = Describe("kube-dns hooks :: legacy_metrics ::", func() {
	f := HookExecutionConfigInit("", "")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("No metrics should be present in an empty custer", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal("expire"))
		})

		Context("Service created with the deprecated annotation", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(serviceWithDeprecatedAnnotation))
				f.RunHook()
			})

			It("Metric should be generated", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(2))
				Expect(m[0].Action).Should(Equal("expire"))
				Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
					Name:   legacyServiceAnnotationMetricName,
					Group:  legacyServiceAnnotationGroup,
					Action: "set",
					Value:  pointer.Float64Ptr(1.0),
					Labels: map[string]string{
						"service_name":      "test",
						"service_namespace": "test",
					},
				}))
			})

			Context("Service updated without the deprecated annotation", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(serviceWithoutDeprecatedAnnotation))
					f.RunHook()
				})

				It("Metric should be removed", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(1))
					Expect(m[0].Action).Should(Equal("expire"))
				})
			})

			Context("Service updated with the proper spec field", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(serviceWithDeprecatedAnnotationAndWithSpecField))
					f.RunHook()
				})

				It("Metric should be removed", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(1))
					Expect(m[0].Action).Should(Equal("expire"))
				})
			})
		})
	})
})
