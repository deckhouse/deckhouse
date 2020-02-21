package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: migrate user passwords ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "User", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("With config values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ConfigValuesSetFromYaml("userAuthn.users", []byte(`
admin@example.com: randomPass
user+name@example.com: randomPass
256@flant.com: password
name.surename@example.com: testcom
`))
			f.RunHook()
		})
		It("Should create User objects and delete them from config values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ConfigValuesGet("userAuthn.users").Exists()).ToNot(BeTrue())
			Expect(f.KubernetesGlobalResource("User", "admin-example-com").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("User", "user-name-example-com").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("User", "256-flant-com").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("User", "name-surename-example-com").Exists()).To(BeTrue())
		})
	})
})
