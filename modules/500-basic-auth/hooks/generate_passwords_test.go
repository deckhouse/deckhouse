package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/hook-testing/library"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	valuesString = `{}`
	configValuesString = `{}`
	bindingContext     = `[]`
)

var _ = Describe("", func() {
	SetupHookExecutionConfig(valuesString, configValuesString, bindingContext)

	Context("without basicAuth.locations in values", func() {
		Hook("", func(hookResult *HookExecutionResult) {
			Expect(hookResult).To(ExecuteSuccessfully())
			Expect(hookResult).To(ConfigValuesKeyEquals("basicAuth.locations.0.location", "/"))
			Expect(hookResult).To(ConfigValuesHasKey("basicAuth.locations.0.users.admin"))
		})
	})

	Context("with basicAuth.locations in values", func() {
		BeforeEach(func() {
			HookConfig.ValuesSet("basicAuth.locations.0.location", "/")
			HookConfig.ValuesSet("basicAuth.locations.0.users.admin", "test123")
		})

		Hook("", func(hookResult *HookExecutionResult) {
			Expect(hookResult).To(ExecuteSuccessfully())
			Expect(hookResult).ToNot(ConfigValuesHasKey("basicAuth.locations"))
		})
	})
})
