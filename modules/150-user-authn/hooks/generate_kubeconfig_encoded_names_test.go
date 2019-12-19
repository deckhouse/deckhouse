package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("User Authn hooks :: generate kubeconfig encoded names ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{}}}`, "")

	Context("Without kubeconfig in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
		})
	})

	Context("With kubeconfig in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ConfigValuesSetFromYaml("userAuthn.kubeconfigGenerator", []byte(`[
{"id": "kubeconfig-one", "masterURI": "127.0.0.1", "description": "test"},
{"id": "kubeconfig-two", "masterURI": "test.example.com", "description": "test2"}
]`))
			f.RunHook()
		})

		It("Should add encoded kubeconfig names", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.kubeconfigEncodedNames").String()).To(MatchJSON(`["nn2wezldn5xgm2lhfvtwk3tfojqxi33sfuymx4u44scceizf", "nn2wezldn5xgm2lhfvtwk3tfojqxi33sfuy4x4u44scceizf"]`))
		})
	})
})
