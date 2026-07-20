/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Metallb hooks :: service alerts ::", func() {
	f := HookExecutionConfigInit(`{}`, `{"global":{"discovery":{}}}`)

	Context("There is Service with deprecated annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Service
metadata:
  name: nginx1
  namespace: nginx1-ns
  annotations:
    metallb.io/address-pool: "aaa"
spec:
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: nginx2
  namespace: nginx2-ns
spec:
  type: LoadBalancer
`))
			f.RunHook()
		})

		It("Should fire D8MetallbNotSupportedServiceAnnotationsDetected alert for nginx1", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_supported_service_annotations_detected" {
					if metric.Labels["name"] == "nginx1" {
						found = true
						Expect(*metric.Value).To(Equal(float64(1)))
						Expect(metric.Labels["annotation"]).To(Equal("metallb.io/address-pool"))
					}
				}
			}
			Expect(found).To(BeTrue(), "Expected to find metric for nginx1")
		})

		It("Should not fire alert for nginx2", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_supported_service_annotations_detected" {
					Expect(metric.Labels["name"]).ToNot(Equal("nginx2"))
				}
			}
		})
	})
})
