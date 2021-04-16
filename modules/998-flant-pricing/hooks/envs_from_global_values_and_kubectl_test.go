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
  enabledModules: ["deckhouse", "cloud-provider-openstack", "terraform-manager"]
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
  enabledModules: ["deckhouse", "cloud-provider-openstack", "terraform-manager"]
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
			a.BindingContexts.Set(a.GenerateBeforeHelmContext())
			a.RunHook()
		})

		It("Should work properly", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(a.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(a.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Hybrid`))
			Expect(a.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
			Expect(a.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`true`))
		})
	})

	b := HookExecutionConfigInit(initValuesStatic, `{}`)

	Context("Static cluster", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should work properly", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`none`))
			Expect(b.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(b.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Static`))
			Expect(b.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
			Expect(b.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`false`))
		})
	})

	c := HookExecutionConfigInit(initValuesCloudClusterWithStaticNodes, `{}`)

	Context("Cloud cluster with static nodes", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.GenerateBeforeHelmContext())
			c.RunHook()
		})

		It("Should work properly", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(c.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(c.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Hybrid`))
			Expect(c.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
			Expect(c.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`true`))
		})
	})

	d := HookExecutionConfigInit(initValuesKops, `{}`)

	Context("Kops cluster", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.GenerateBeforeHelmContext())
			d.RunHook()
		})

		It("Should work properly", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`none`))
			Expect(d.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(d.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Cloud`))
			Expect(d.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`true`))
			Expect(d.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`false`))
		})
	})

	e := HookExecutionConfigInit(initValuesCloudClusterWithStaticNodes, `{}`)

	Context("Cloud cluster with static nodes and clusterType override", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.GenerateBeforeHelmContext())
			e.ValuesSet("flantPricing.clusterType", "Cloud")
			e.RunHook()
		})

		It("Should work properly", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(e.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(e.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Cloud`))
			Expect(e.ValuesGet("flantPricing.internal.kops").String()).To(Equal(`false`))
			Expect(e.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`true`))
		})
	})
})
