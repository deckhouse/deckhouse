package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: node-manager :: helm template :: standby node", func() {
	f := SetupHelmConfig(``)

	Context("Two NGs with standby", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.standbyNodeGroups", `[{name: standby-absolute, standby: 2, reserveCPU: "5500m", reserveMemory: "983Mi", taints: [{effect: NoExecute, key: ship-class, value: frigate}]}, {name: standby-percent, standby: 12, reserveCPU: "3400m", reserveMemory: 10Mi, taints: [{operator: Exists}]}]`)
			f.HelmRender()
		})

		It("should render correctly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			da := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "standby-standby-absolute")
			Expect(da.Exists()).To(BeTrue())
			Expect(da.Field("spec.replicas").String()).To(Equal("2"))
			Expect(da.Field("spec.template.spec.priorityClassName").String()).To(Equal("standby"))
			Expect(da.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("5500m"))
			Expect(da.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("983Mi"))
			Expect(da.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: ship-class
  value: frigate
  effect: NoExecute
`))

			dp := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "standby-standby-percent")
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.replicas").String()).To(Equal("12"))
			Expect(dp.Field("spec.template.spec.priorityClassName").String()).To(Equal("standby"))
			Expect(dp.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("3400m"))
			Expect(dp.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("10Mi"))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- operator: Exists
`))
		})
	})
})
