package template_tests

import (
	"testing"

	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeCaAuthProxy: tagstring
      kubeRbacProxy: tagstring
    prometheus:
      grafana: tagstring
      grafanaDashboardProvisioner: tagstring
      prometheus: tagstring
      trickster: tagstring
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
  alertmanagers: {}
  prometheusAPIClientTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  prometheusScraperTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
`

var _ = Describe("Module :: prometheus :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("prometheus", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-monitoring", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

})
