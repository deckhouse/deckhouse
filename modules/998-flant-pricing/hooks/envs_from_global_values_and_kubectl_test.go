package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: flant-pricing :: hooks :: envs_from_global_values_and_kubectl ", func() {
	const (
		initValuesHybridWithCloudProvider = `
global:
  enabledModules: ["deckhouse", "cloud-provider-openstack"]
  clusterConfiguration:
    clusterType: Static
flantPricing:
  internal: {}
`
		initValuesStatic = `
global:
  enabledModules: ["deckhouse"]
  clusterConfiguration:
    clusterType: Static
flantPricing:
  internal: {}
`
		initValuesCloudClusterWithStaticNodes = `
global:
  enabledModules: ["deckhouse", "cloud-provider-openstack"]
  clusterConfiguration:
    clusterType: Static
  discovery:
    nodeCountByType:
      static: 5
flantPricing:
  internal: {}
`
		initValuesKops = `
global:
  enabledModules: ["deckhouse"]
  discovery:
    kubernetesVersion: 1.14.1
flantPricing:
  internal: {}
`
		)

	a := HookExecutionConfigInit(initValuesHybridWithCloudProvider, `{}`)

	Context("Hybrid cluster and cloud-provider-openstack is enabled", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(BeforeHelmContext)
			a.RunHook()
		})

		It("Should work properly", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(a.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(a.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Hybrid`))
			Expect(a.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
		})
	})

	b := HookExecutionConfigInit(initValuesStatic, `{}`)

	Context("Static cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(BeforeHelmContext)
			b.RunHook()
		})

		It("Should work properly", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`none`))
			Expect(b.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(b.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Static`))
			Expect(b.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
		})
	})

	c := HookExecutionConfigInit(initValuesCloudClusterWithStaticNodes, `{}`)

	Context("Cloud cluster with static nodes", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(BeforeHelmContext)
			c.RunHook()
		})

		It("Should work properly", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(c.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(c.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Hybrid`))
			Expect(c.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
		})
	})

	d := HookExecutionConfigInit(initValuesKops, `{}`)

	Context("Kops cluster", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(BeforeHelmContext)
			d.RunHook()
		})

		It("Should work properly", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`none`))
			Expect(d.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(d.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Cloud`))
			Expect(d.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`true`))
		})
	})
})
