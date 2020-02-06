package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: convert_internal_subnet_cidr_to_regex ::", func() {
	f := HookExecutionConfigInit(`{"cloudProviderOpenstack":{"internal":{}}}`, `{}`)

	Context("BeforeHelm — cloudProviderOpenstack.internalSubnet isn't set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Hook must not fail and cloudProviderOpenstack.internal.internalSubnetRegex must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderOpenstack.internal.internalSubnetRegex").Exists()).To(BeFalse())
		})
	})

	Context("BeforeHelm — cloudProviderOpenstack.internalSubnet is '10.10.10.0/25'", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderOpenstack.internalSubnet", "10.10.10.0/25")
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It(`Hook must not fail and cloudProviderOpenstack.internal.internalSubnetRegex must be '10(\.10){2}\.(12[0-7]|1[01][0-9]|[1-9]?[0-9])'`, func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderOpenstack.internal.internalSubnetRegex").String()).To(Equal(`10(\.10){2}\.(12[0-7]|1[01][0-9]|[1-9]?[0-9])`))
		})
	})

})
