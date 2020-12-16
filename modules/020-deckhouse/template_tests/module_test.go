package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
project: my_project
clusterName: my_cluster
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "operator-prometheus-crd"]
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30s
  d8SpecificNodeCountByRole:
    system: 1
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
modules:
  placement: {}
`

	moduleValuesForMasterNode = `
    bundle: Default
    logLevel: Info
`

	moduleValuesForDeckhouseNode = `
    bundle: Default
    logLevel: Info
    nodeSelector: 'node-role.kubernetes.io/deckhouse: ""'
    tolerations:
    - key: testkey
      operator: Exists
`
)

var _ = Describe("Module :: deckhouse :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/master: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
  - operator: Exists
`))
		})
	})

	Context("Cluster with deckhouse on system node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/deckhouse: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: testkey
  operator: Exists
`))
		})
	})

})
