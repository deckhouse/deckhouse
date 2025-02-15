/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Metallb hooks :: check requirements for upgrade ::", func() {
	f := HookExecutionConfigInit(`{}`, `{"global":{"discovery":{}}}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster have a ModuleConfig version 3", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 3
  settings:
    addressPools:
    - addresses:
      - 192.168.219.100-192.168.219.100
      name: frontend-pool
      protocol: layer2
`))
			f.RunHook()
		})
		It("Should not proceed as ModuleConfig version is >= 2 and add alert", func() {
			Expect(f).To(ExecuteSuccessfully())
			mc := f.KubernetesResource("ModuleConfig", "", "metallb")
			Expect(mc.Exists()).To(BeTrue(), "ModuleConfig resource should exist")
			Expect(mc.Field("spec.version").Int()).To(BeNumerically(">=", 2))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_obsolete_layer2_pools_are_used" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					Expect(metric.Labels["name"]).To(Equal("frontend-pool"))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find "+
				"'d8_metallb_obsolete_layer2_pools_are_used' metric")
		})
	})

	Context("There is Service with deprecated annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 1
---
apiVersion: v1
kind: Service
metadata:
  name: nginx1
  namespace: nginx1-ns
  annotations:
    metallb.universe.tf/address-pool: "aaa"
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

		It("The 'd8_metallb_update_mc_version_required' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_update_mc_version_required" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find "+
				"'d8_metallb_update_mc_version_required' metric")
		})

		It("The 'd8_metallb_not_supported_service_annotations_detected' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_supported_service_annotations_detected" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					Expect(metric.Labels["name"]).To(Equal("nginx1"))
					Expect(metric.Labels["namespace"]).To(Equal("nginx1-ns"))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find "+
				"'d8_metallb_not_supported_service_annotations_detected' metric")
		})
	})
})
