package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cniFlannel :: hooks :: set_pod_network_mode ::", func() {
	f := HookExecutionConfigInit(`{"cniFlannel":{"internal":{}}}`, ``)

	state := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
  flannel: ICAgIHsKICAgICAgInBvZE5ldHdvcmtNb2RlIjogInZ4bGFuIgogICAgfQ== # {"podNetworkMode":"vxlan"}
`

	stateWithEmptyFlannelConfig := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
  flannel: e30= # {}
`

	stateWithoutFlannelConfig := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
`

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("d8-cni-configuration", func() {
		It("Must be executed successfully", func() {
			By("podNetworkMode must be vxlan", func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
			})
		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be vxlan, because secret has higher priority, than config", func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "host-gw")
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
			})

		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be host-gw", func() {
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "host-gw")
				f.BindingContexts.Set(BeforeHelmContext)
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})

		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(stateWithEmptyFlannelConfig))
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})
		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(stateWithoutFlannelConfig))
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})
		})
	})
})
