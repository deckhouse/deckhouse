/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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

	scanJobsToleratrionValues = `
scanJobs:
  tolerations:
    - key: "key1"
      operator: "Equal"
      value: "value1"
      effect: "NoSchedule"
`
)

var _ = Describe("Module :: operator-trivy :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

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
	})

	Context("Scan jobs with tolerations", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", scanJobsToleratrionValues)
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has scan jobs tolerations", func() {
			otcm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator")
			Expect(otcm.Exists()).To(BeTrue())

			cmdData := otcm.Field(`data`).Map()
			Expect(cmdData["scanJob.tolerations"]).To(MatchJSON(`[{"effect":"NoSchedule","key":"key1","operator":"Equal","value":"value1"}]`))
		})
	})

})
