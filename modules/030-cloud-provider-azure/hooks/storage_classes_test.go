package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-azure :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderAzure:
  internal: {}
  storageClass:
    provision:
    - name: managed-ultra-ssd
      diskIOPSReadWrite: 600
      diskMBpsReadWrite: 150
    exclude:
    - sc\d+
    - bar
    default: other-bar
`
		initValuesExcludeAllString = `
cloudProviderAzure:
  internal: {}
  storageClass:
    exclude:
    - ".*"
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Should discover storageClasses with default set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderAzure.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "managed-standard-ssd",
    "type": "StandardSSD_LRS"
  },
  {
    "name": "managed-standard",
    "type": "Standard_LRS"
  },
  {
    "name": "managed-premium",
    "type": "Premium_LRS"
  },
  {
    "diskIOPSReadWrite": 600,
    "diskMBpsReadWrite": 150,
    "name": "managed-ultra-ssd",
    "type": "UltraSSD_LRS"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderAzure.internal.defaultStorageClass").String()).To(Equal(`other-bar`))
		})

	})

	fb := HookExecutionConfigInit(initValuesExcludeAllString, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			fb.BindingContexts.Set(BeforeHelmContext)
			fb.RunHook()
		})

		It("Should discover no storageClasses with no default is set", func() {
			Expect(fb).To(ExecuteSuccessfully())
			Expect(fb.ValuesGet("cloudProviderAzure.internal.storageClasses").String()).To(MatchJSON(`[]`))
			Expect(fb.ValuesGet("cloudProviderAzure.internal.defaultStorageClass").Exists()).To(BeFalse())
		})

	})

})
