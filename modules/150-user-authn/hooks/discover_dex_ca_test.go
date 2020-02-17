package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{"controlPlaneConfigurator":{}},"controlPlaneConfigurator":{"enabled":true}}}`, "")

	Context("With FromIngressSecret option and empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
		})

		Context("Adding secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-user-authn
data:
  tls.crt: dGVzdA==`))
				f.RunHook()
			})

			It("Should add ca for oidc provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
			})
		})
	})

	Context("With FromIngressSecret option and secret", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-user-authn
data:
  ca.crt: dGVzdA==`))
			f.RunHook()
		})

		It("Should add ca for oidc provider", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
		})
	})

	Context("With DoNotNeed option", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "DoNotNeed")
			f.RunHook()
		})
		It("Should add no ca for oidc provider", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal(""))
		})
	})

	Context("With Custom option and ca in config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "Custom")
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCustomCA", "testca")

			f.RunHook()
		})
		It("Should add no ca for oidc provide from config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
		})
	})
})
