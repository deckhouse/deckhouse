package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: flant-pricing :: hooks :: envs_from_metrics", func() {
	const (
		initValuesTerraformManagerEnabled = `
global:
  enabledModules: ["deckhouse", "terraform-manager"]
  clusterConfiguration:
    clusterType: Static
flantPricing:
  internal: {}
`
		initValuesTerraformManagerDisabled = `
global:
  enabledModules: ["deckhouse"]
  clusterConfiguration:
    clusterType: Static
flantPricing:
  internal: {}
`
	)

	f := HookExecutionConfigInit(initValuesTerraformManagerEnabled, `{}`)

	Context("TerraformManager is enabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("flantPricing.internal values should be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantPricing.internal.deprecatedResourcesInHelmReleases").String()).To(Equal(`100`))
			Expect(f.ValuesGet("flantPricing.internal.convergeIsCompleted").String()).To(Equal(`false`))
		})
	})

	b := HookExecutionConfigInit(initValuesTerraformManagerDisabled, `{}`)

	Context("TerraformManager is disabled", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(OnStartupContext)
			b.RunHook()
		})

		It("flantPricing.internal values should be correct", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("flantPricing.internal.deprecatedResourcesInHelmReleases").String()).To(Equal(`100`))
			Expect(b.ValuesGet("flantPricing.internal.convergeIsCompleted").String()).To(Equal(`true`))
		})
	})
})
