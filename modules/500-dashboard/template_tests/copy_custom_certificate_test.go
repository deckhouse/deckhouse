package template_tests

import (
	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module :: dashboard :: helm template :: custom-certificate", func() {
	const globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "dashboard"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeCaAuthProxy: tagstring
      kubeRbacProxy: tagstring
    dashboard:
      dashboard: tagstring
      metricsScraper: tagstring
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`
	const customCertificatePresent = `
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("dashboard", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

})
