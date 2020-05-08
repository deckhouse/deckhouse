package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: set_instance_prefix ::", func() {
	f := HookExecutionConfigInit(`
global:
  clusterConfiguration:
    spec:
      cloud: {}
nodeManager:
  internal: {}
`, `{}`)

	Context("BeforeHelm — nodeManager.instancePrefix isn't set", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.cloud.prefix", "global")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Hook must not fail and nodeManager.internal.instancePrefix is 'global'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.instancePrefix").String()).To(Equal("global"))
		})
	})

	Context("BeforeHelm — nodeManager.instancePrefix is 'kube'", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.instancePrefix", "kube")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It(`Hook must not fail and nodeManager.internal.instancePrefix must be 'kube'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.instancePrefix").String()).To(Equal("kube"))
		})
	})

})
