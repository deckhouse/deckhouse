package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderYandex:
  internal: {}
  storageClass:
    exclude:
    - .*-hdd
    - bar
    default: baz
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
			Expect(f.ValuesGet("cloudProviderYandex.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "network-ssd",
	"type": "network-ssd"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderYandex.internal.defaultStorageClass").String()).To(Equal(`baz`))
		})

	})

})
