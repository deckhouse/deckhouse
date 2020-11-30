package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: cloud-provider-gcp :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderGcp:
  internal: {}
  storageClass:
    exclude:
    - .*standard.*
    - bar
    default: pd-ssd-replicated
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
			Expect(f.ValuesGet("cloudProviderGcp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "pd-ssd-not-replicated",
	"replicationType": "none",
	"type": "pd-ssd"
  },
  {
	"name": "pd-ssd-replicated",
	"replicationType": "regional-pd",
	"type": "pd-ssd"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderGcp.internal.defaultStorageClass").String()).To(Equal(`pd-ssd-replicated`))
		})

	})

})
