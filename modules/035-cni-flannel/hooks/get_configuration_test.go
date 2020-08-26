package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func TestHooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hooks Suite")
}

var _ = Describe("Modules :: cniFlannel :: hooks :: get_configuration ::", func() {
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
  flannel: ICAgIHsKICAgICAgInBvZE5ldHdvcmtNb2RlIjogInZ4bGFuIgogICAgfQ== # {"podNetworkMode":"vxlan"}"
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
	})

	Context("d8-cni-configuration", func() {

		It("Must be executed successfully", func() {

			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "host-gw")
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})

		})
	})

	Context("d8-cni-configuration", func() {

		It("Must be executed successfully", func() {

			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(stateWithoutFlannelConfig))
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})

		})
	})
})
