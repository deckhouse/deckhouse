package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: cloud-provider-aws :: hooks :: storage_classes ::", func() {
	const (
		initValuesString = `
cloudProviderAws:
  internal: {}
  storageClass:
    provision:
    - iopsPerGB: 5
      name: iops-foo
      type: io1
    exclude:
    - sc\d+
    - bar
    default: other-bar
`
		initValuesExcludeAllString = `
cloudProviderAws:
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
			Expect(f.ValuesGet("cloudProviderAws.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "gp2",
	"type": "gp2"
  },
  {
	"iopsPerGB": 5,
	"name": "iops-foo",
	"type": "io1"
  },
  {
	"name": "st1",
	"type": "st1"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderAws.internal.defaultStorageClass").String()).To(Equal(`other-bar`))
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
			Expect(fb.ValuesGet("cloudProviderAws.internal.storageClasses").String()).To(MatchJSON(`[]`))
			Expect(fb.ValuesGet("cloudProviderAws.internal.defaultStorageClass").Exists()).To(BeFalse())
		})

	})

})
