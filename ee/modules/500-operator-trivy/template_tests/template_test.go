/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "operator-trivy"]
modulesImages:
  registry:
    base: registry.deckhouse.io/deckhouse/fe
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterDomain: cluster.local
`

	scanJobsValues = `
tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
nodeSelector:
  test-label: test-value
`
)

var _ = Describe("Module :: operator-trivy :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	checkTivyOperatorCM := func(f *Config, tolerations, nodeSelector types.GomegaMatcher) {
		cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator")
		Expect(cm.Exists()).To(BeTrue())

		cmdData := cm.Field(`data`).Map()

		Expect(cmdData["scanJob.tolerations"].String()).To(tolerations)
		Expect(cmdData["scanJob.nodeSelector"].String()).To(nodeSelector)

	}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	Context("Default", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has proper tolerations and nodeSelector", func() {
			checkTivyOperatorCM(f, Equal(""), Equal(""))
		})
	})

	Context("Operator trivy with custom tolerations and nodeSelector", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", scanJobsValues)
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has proper tolerations and nodeSelector", func() {
			tolerations := `[{"effect":"NoSchedule","key":"key1","operator":"Equal","value":"value1"}]`
			nodeSelector := `{"test-label":"test-value"}`
			checkTivyOperatorCM(f, MatchJSON(tolerations), MatchJSON(nodeSelector))
		})
	})

})
