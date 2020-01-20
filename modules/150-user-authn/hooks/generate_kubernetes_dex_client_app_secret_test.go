package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate kubernetes dex client app secret ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	var clientAppSecret string

	Context("Before helm", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
		})

		Context("With another before helm", func() {
			BeforeEach(func() {
				clientAppSecret = f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()

				f.BindingContexts.Set(BeforeHelmContext)
				f.RunHook()
			})

			It("Do not change the values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
				Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(Equal(clientAppSecret))
			})
		})
	})
})
