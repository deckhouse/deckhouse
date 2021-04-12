package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: deckhouse_version ", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Unknown deckhouse version", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseVersion").String()).To(Equal("unknown"))
		})
	})
})
