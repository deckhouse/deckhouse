package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{"controlPlaneConfigurator":{}}, "controlPlaneConfigurator":{"enabled":true}, "https": {"mode":"CertManager"}}}`, "")

	Context("With FromIngressSecret option and empty cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Adding secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-user-authn
data:
  tls.crt: dGVzdA==
`, 1))
				f.RunHook()
			})

			It("Should add ca for OIDC provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
			})
		})
	})

	Context("With DoNotNeed option", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "DoNotNeed")
			f.RunHook()
		})
		It("Should add no ca for OIDC provider", func() {
			Expect(f).To(ExecuteSuccessfully())
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
		It("Should add no ca for OIDC provide from config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
		})
	})
})
