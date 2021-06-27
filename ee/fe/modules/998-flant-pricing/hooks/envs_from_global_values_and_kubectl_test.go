/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery/fake"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
  internal:
    nodeStats:
      staticNodesCount: 0
`
		initValuesStatic = `
global:
  enabledModules: ["deckhouse"]
  clusterConfiguration:
    clusterType: Static
flantPricing:
  internal:
    nodeStats:
      staticNodesCount: 0
`
		initValuesCloudClusterWithStaticNodes = `
global:
  enabledModules: ["deckhouse", "cloud-provider-openstack", "terraform-manager"]
  clusterConfiguration:
    clusterType: Cloud
flantPricing:
  internal:
    nodeStats:
      staticNodesCount: 1
`
		initValuesCloudClusterWithoutStaticNodeCount = `
global:
  enabledModules: ["deckhouse", "cloud-provider-openstack", "terraform-manager"]
  clusterConfiguration:
    clusterType: Cloud
flantPricing:
  internal: {}
`
	)

	BeforeEach(func() {
		dependency.TestDC.K8sClient.Discovery().(*fake.FakeDiscovery).FakedServerVersion = &version.Info{
			Major:      "1",
			Minor:      "16",
			GitVersion: "v1.16.5-rc.0",
		}
	})

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
			Expect(c.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`true`))
		})
	})

	e := HookExecutionConfigInit(initValuesHybridWithCloudProvider, `{}`)

	Context("Hybrid cluster and cloud-provider-openstack is enabled with clusterType override", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.GenerateBeforeHelmContext())
			e.ValuesSet("flantPricing.clusterType", "Cloud")
			e.RunHook()
		})

		It("Should work properly", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("flantPricing.internal.cloudProvider").String()).To(Equal(`openstack`))
			Expect(e.ValuesGet("flantPricing.internal.controlPlaneVersion").String()).To(Equal(`1.16`))
			Expect(e.ValuesGet("flantPricing.internal.clusterType").String()).To(Equal(`Cloud`))
			Expect(e.ValuesGet("flantPricing.internal.terraformManagerEnabled").String()).To(Equal(`true`))
		})
	})

	d := HookExecutionConfigInit(initValuesCloudClusterWithoutStaticNodeCount, `{}`)

	Context("Cluster without `internal.nodeStats.staticNodesCount` value", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.GenerateBeforeHelmContext())
			d.RunHook()
		})

		It("Should exit with error", func() {
			Expect(d).NotTo(ExecuteSuccessfully())
		})
	})
})
