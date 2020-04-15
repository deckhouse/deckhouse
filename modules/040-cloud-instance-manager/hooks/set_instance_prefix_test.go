package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: set_instance_prefix ::", func() {
	f := HookExecutionConfigInit(`
global:
  clusterConfiguration:
    spec:
      cloud: {}
cloudInstanceManager:
  internal: {}
`, `{}`)

	Context("BeforeHelm — cloudInstanceManager.instancePrefix isn't set", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.spec.cloud.prefix", "global")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Hook must not fail and cloudInstanceManager.internal.instancePrefix is 'global'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.instancePrefix").String()).To(Equal("global"))
		})
	})

	Context("BeforeHelm — cloudInstanceManager.instancePrefix is 'kube'", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudInstanceManager.instancePrefix", "kube")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It(`Hook must not fail and cloudInstanceManager.internal.instancePrefix must be 'kube'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.instancePrefix").String()).To(Equal("kube"))
		})
	})

})
