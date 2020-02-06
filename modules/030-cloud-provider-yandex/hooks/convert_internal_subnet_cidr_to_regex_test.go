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

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: convert_internal_subnet_cidr_to_regex ::", func() {
	f := HookExecutionConfigInit(`{"cloudProviderYandex":{"internal":{}}}`, `{}`)

	Context("BeforeHelm — cloudProviderYandex.internalSubnet isn't set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Hook must not fail and cloudProviderYandex.internal.internalSubnetRegex must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.internal.internalSubnetRegex").Exists()).To(BeFalse())
		})
	})

	Context("BeforeHelm — cloudProviderYandex.internalSubnet is '10.10.10.0/25'", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderYandex.internalSubnet", "10.10.10.0/25")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It(`Hook must not fail and cloudProviderYandex.internal.internalSubnetRegex must be '10(\.10){2}\.(12[0-7]|1[01][0-9]|[1-9]?[0-9])'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.internal.internalSubnetRegex").String()).To(Equal(`10(\.10){2}\.(12[0-7]|1[01][0-9]|[1-9]?[0-9])`))
		})
	})

})
