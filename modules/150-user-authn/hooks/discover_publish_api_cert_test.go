package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}, "https": { "mode": "CertManager"}}}`, "")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("After adding secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdA==
`))
				f.RunHook()
			})

			It("Should add internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
				Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("test"))
			})

		})
	})

})
