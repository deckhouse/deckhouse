/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	moduleConfig = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 1
  settings:
    speaker:
      nodeSelector:
        node-role.deckhouse.io/metallb: ""
      tolerations:
        - effect: NoExecute
          key: dedicated.deckhouse.io
          operator: Equal
          value: frontend
    addressPools:
      - name: nginx-loadbalancer-pool1
        protocol: layer2
        addresses:
          - 192.168.70.100-192.168.70.110
      - name: nginx-loadbalancer-pool2
        protocol: layer2
        addresses:
          - 192.168.71.100-192.168.72.110
`
	expectedMLBC = `
apiVersion: network.deckhouse.io/v1alpha1
kind: MetalLoadBalancerClass
metadata:
  name: default
spec:
  isDefault: true
  type: L2
  addressPool:
  - 192.168.70.100-192.168.70.110
  - 192.168.71.100-192.168.72.110
  nodeSelector:
    node-role.deckhouse.io/metallb: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
`
)

var _ = Describe("Metallb hooks :: migrate MC to MetalLoadBalancerClass ::", func() {
	f := HookExecutionConfigInit(`{"metallb":{"internal":{}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "MetalLoadBalancerClass", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleConfig))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Cluster with Metallb ModuleConfig", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleConfig))
			f.RunHook()
		})

		It("Created a new resource based on ModuleConfig", func() {
			Expect(f).To(ExecuteSuccessfully())

			MLBC := f.KubernetesResource("MetalLoadBalancerClass", "", "default")
			Expect(MLBC.ToYaml()).To(MatchYAML(expectedMLBC))
		})
	})
})
