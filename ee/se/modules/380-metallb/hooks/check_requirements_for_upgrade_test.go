/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	config = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 1
  settings:
`
	l2Advertisement = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-1
  - pool-2
  nodeSelectors:
  - matchLabels:
      zone: a
`
	ipAddressPools = `
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
spec:
  addresses:
  - 11.11.11.11/32
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-2
  namespace: d8-metallb
spec:
  addresses:
  - 22.22.22.22/32
`
	ipAddressPools2 = `
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
spec:
  addresses:
  - 11.11.11.11/32
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-3
  namespace: d8-metallb
spec:
  addresses:
  - 33.33.33.33/32
`
	l2Advertisement2 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-b
  namespace: metallb
spec:
  ipAddressPools:
  - pool-1
  nodeSelectors:
  - matchLabels:
      zone: b
`
	l2Advertisement3 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-2
  nodeSelectors:
  - matchLabels:
      zone: a
`
	l2Advertisement4 = `
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-b
  namespace: d8-metallb
spec:
  ipAddressPools:
  - pool-1
  nodeSelectors:
  - matchExpressions: []
  - matchLabels:
      zone: b
`
	ipAddressPools3 = `
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
spec:
  addresses:
  - 11.11.11.11/32
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-3
  namespace: metallb
spec:
  addresses:
  - 33.33.33.33/32
`
	services = `
---
apiVersion: v1
kind: Service
metadata:
  name: nginx1
  namespace: nginx1
  annotations:
    metallb.universe.tf/address-pool: "aaa"
    metallb.universe.tf/ip-allocated-from-pool: "bbb"
spec:
  clusterIP: 1.2.3.4
  ports:
  - port: 7474
    protocol: TCP
    targetPort: 7474
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx1
  type: LoadBalancer
  loadBalancerClass: test
---
apiVersion: v1
kind: Service
metadata:
  name: nginx2
  namespace: nginx2
  annotations:
    metallb.universe.tf/address-pool: "aaa"
spec:
  clusterIP: 2.3.4.5
  ports:
  - port: 7474
    protocol: TCP
    targetPort: 7474
  externalTrafficPolicy: Local
  internalTrafficPolicy: Cluster
  selector:
    app: nginx2
  type: LoadBalancer
  loadBalancerClass: test
`
)

var _ = Describe("Metallb hooks :: check requirements for upgrade ::", func() {
	f := HookExecutionConfigInit(`{}`, `{"global":{"discovery":{}}}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	f.RegisterCRD("metallb.io", "v1beta1", "L2Advertisement", true)
	f.RegisterCRD("metallb.io", "v1beta1", "IPAddressPool", true)

	Context("Check correct ModuleConfig version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config))
			f.RunHook()
		})
		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			mc := f.KubernetesResource("ModuleConfig", "", "metallb")
			Expect(mc.Field("spec.version").String()).NotTo(Equal("2"))
		})
	})

	Context("Check correct configurations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + l2Advertisement + ipAddressPools))
			f.RunHook()
		})
		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("OK"))
		})
	})

	Context("Check AddressPoolsMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + l2Advertisement + ipAddressPools2))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_only_layer2_pools" {
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
		})
	})

	Context("Check L2AdvertisementNSMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + l2Advertisement2 + l2Advertisement3))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_l2advertisement_ns_mismatch" {
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
		})
	})

	Context("Check IpAddressPoolNSMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + ipAddressPools3))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_ipaddress_pool_ns_mismatch" {
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
		})
	})

	Context("Check NodeSelectorsMismatch error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + l2Advertisement3 + l2Advertisement4))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_l2advertisement_node_selectors_mismatch" {
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
		})
	})

	Context("Check OrphanedServices error", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(config + services))
			f.RunHook()
		})

		It("Check the set variable", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_orphaned_loadbalancer_detected" {
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
		})
	})
})
