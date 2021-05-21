package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: generate_password ", func() {

	f := HookExecutionConfigInit(`{"prometheus":{"internal":{}, "auth": {}}}`, `{}`)
	Context("without external auth", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.auth.password").String()).ShouldNot(BeEmpty())
		})
	})

	Context("with extisting password", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ValuesSet("prometheus.auth.password", "zxczxczxc")
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.auth.password").String()).Should(BeEquivalentTo("zxczxczxc"))
		})
	})

	Context("with external auth", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.ValuesSet("prometheus.auth.externalAuthentication", "ok")
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.auth.password").String()).Should(BeEmpty())
			Expect(f.ConfigValuesGet("prometheus.auth").Exists()).Should(BeFalse())
		})
	})
})
