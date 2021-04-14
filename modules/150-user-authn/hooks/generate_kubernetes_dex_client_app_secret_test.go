package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate kubernetes dex client app secret ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	var clientAppSecret string
	var testSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-dex-client-app-secret
  namespace: d8-user-authn
data:
  secret: QUJD # ABC
`
	Context("With secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testSecret))
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()).To(Equal("ABC"))
		})
	})

	Context("With empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").Exists()).To(BeTrue())
		})

		Context("With another run", func() {
			BeforeEach(func() {
				clientAppSecret = f.ValuesGet("userAuthn.internal.kubernetesDexClientAppSecret").String()

				f.BindingContexts.Set(f.KubeStateSet(testSecret))
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
