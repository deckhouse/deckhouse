package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
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
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover storageClasses with default set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderGcp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "pd-balanced-not-replicated",
	"replicationType": "none",
	"type": "pd-balanced"
  },
  {
	"name": "pd-balanced-replicated",
	"replicationType": "regional-pd",
	"type": "pd-balanced"
  },
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
