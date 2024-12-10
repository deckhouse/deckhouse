/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Metallb hooks :: check requirements for upgrade ::", func() {
	f := HookExecutionConfigInit(`{}`, `{"global":{"discovery":{}}}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	f.RegisterCRD("metallb.io", "v1beta1", "L2Advertisement", true)
	f.RegisterCRD("metallb.io", "v1beta1", "IPAddressPool", true)

	Context("Cluster have correct configurations", func() {
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
`))
			f.RunHook()
		})
		It("ConfigurationStatus should be OK", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("OK"))
		})
	})

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
`))
			f.RunHook()
		})
		It("Should not proceed as ModuleConfig version is >= 2", func() {
			Expect(f).To(ExecuteSuccessfully())
			mc := f.KubernetesResource("ModuleConfig", "", "metallb")
			Expect(mc.Exists()).To(BeTrue(), "ModuleConfig resource should exist")
			Expect(mc.Field("spec.version").Int()).To(BeNumerically(">=", 2))
		})
	})

	Context("MC has pools with different types of pools", func() {
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
  settings:
    addressPools:
      - addresses:
        - 192.168.199.100-192.168.199.200
        name: frontend-pool
        protocol: layer2
      - addresses:
        - 192.168.198.100-192.168.198.200
        name: frontend-pool-bgp
        protocol: bgp
`))
			f.RunHook()
		})
		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_not_only_layer2_pools' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_only_layer2_pools" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_not_only_layer2_pools' metric")
		})
	})

	Context("MC has pools `layer2` pool and L2Advertisement", func() {
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
  settings:
    addressPools:
      - addresses:
        - 192.168.199.100-192.168.199.200
        name: frontend-pool
        protocol: bgp
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
`))
			f.RunHook()
		})
		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_not_only_layer2_pools' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_not_only_layer2_pools" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_not_only_layer2_pools' metric")
		})
	})

	Context("Two L2Advertisement is located different namespaces", func() {
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
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-b
  namespace: metallb
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-a
  namespace: d8-metallb
`))
			f.RunHook()
		})

		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_l2advertisement_ns_mismatch' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_l2advertisement_ns_mismatch" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					Expect(metric.Labels["name"]).To(Equal("zone-b"))
					Expect(metric.Labels["namespace"]).To(Equal("metallb"))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_l2advertisement_ns_mismatch' metric")
		})
	})

	Context("Two IPAddressPool is located different namespaces", func() {
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
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-1
  namespace: d8-metallb
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool-2
  namespace: metallb
`))
			f.RunHook()
		})

		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_ipaddress_pool_ns_mismatch' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_ipaddress_pool_ns_mismatch" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					Expect(metric.Labels["name"]).To(Equal("pool-2"))
					Expect(metric.Labels["namespace"]).To(Equal("metallb"))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_ipaddress_pool_ns_mismatch' metric")
		})
	})

	Context("There is an L2Advertisement with matchExpressions in the cluster", func() {
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
`))
			f.RunHook()
		})

		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_l2advertisement_node_selectors_mismatch' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_l2advertisement_node_selectors_mismatch" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					Expect(metric.Labels["name"]).To(Equal("zone-b"))
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_l2advertisement_node_selectors_mismatch' metric")
		})
	})

	Context("There are several Services and among them one Orphaned Services", func() {
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
  name: nginx0
  namespace: nginx0
spec:
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: nginx1
  namespace: nginx1
spec:
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: nginx2
  namespace: nginx2
spec:
  type: LoadBalancer
  loadBalancerClass: nginx2
---
apiVersion: v1
kind: Service
metadata:
  name: nginx3
  namespace: nginx3
  annotations:
    metallb.universe.tf/address-pool: "nginx3"
spec:
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: nginx4
  namespace: nginx4
  annotations:
    metallb.universe.tf/ip-allocated-from-pool: "nginx4"
spec:
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: nginx5
  namespace: nginx5
  annotations:
    metallb.universe.tf/address-pool: "nginx5"
    metallb.universe.tf/ip-allocated-from: "nginx5"
spec:
  type: LoadBalancer
  loadBalancerClass: nginx5
`))
			f.RunHook()
		})

		It("ConfigurationStatus is Misconfigured and "+
			"the 'd8_metallb_orphaned_loadbalancer_detected' metric is 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			configurationStatusRaw, exists := requirements.GetValue(metallbConfigurationStatusKey)
			Expect(exists).To(BeTrue())
			configurationStatus := configurationStatusRaw.(string)
			Expect(configurationStatus).To(Equal("Misconfigured"))

			metrics := f.MetricsCollector.CollectedMetrics()
			var orphanedServiceNames []string
			found := false
			for _, metric := range metrics {
				if metric.Name == "d8_metallb_orphaned_loadbalancer_detected" {
					found = true
					Expect(*metric.Value).To(Equal(float64(1)))
					orphanedServiceNames = append(orphanedServiceNames, metric.Labels["name"])
				}
			}
			Expect(found).To(BeTrue(), "Expected to find 'd8_metallb_orphaned_loadbalancer_detected' metric")

			sort.Strings(orphanedServiceNames)
			Expect(len(orphanedServiceNames)).To(Equal(1))
			Expect(orphanedServiceNames).To(Equal([]string{"nginx1"}))
		})
	})
})
