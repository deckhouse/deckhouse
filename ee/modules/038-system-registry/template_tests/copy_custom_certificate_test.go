/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "system-registry", "cert-manager"]
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
const customCertificateModeWithDetachedMode = `
mode: Detached
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

const customCertificateModeWithNonDetachedMode = `
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: system-registry :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Detached mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("systemRegistry", customCertificateModeWithDetachedMode)
			f.HelmRender()
		})

		It("Non-empty customcertificate for detached mode", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "embedded-registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

	Context("Non-detached mode", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("systemRegistry", customCertificateModeWithNonDetachedMode)
			f.HelmRender()
		})

		It("Empty customcertificate for non-detached mode", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "embedded-registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeFalse())
		})

	})

})
