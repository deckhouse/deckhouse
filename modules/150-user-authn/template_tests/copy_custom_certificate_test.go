package template_tests

import (
	. "github.com/deckhouse/deckhouse/testing/helm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module :: userAuthn :: helm template :: custom-certificate", func() {
	const globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "user-authn", "cert-manager"]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeCaAuthProxy: tagstring
      kubeRbacProxy: tagstring
    userAuthn:
      busybox: tagstring
      cfssl: tagstring
      crowdBasicAuthProxy: tagstring
      dex: tagstring
      dexAuthenticator: tagstring
      dexAuthenticatorRedis: tagstring
      kubeconfigGenerator: tagstring
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
  kubernetesDexClientAppSecret: plainstring
  kubernetesCA: plainstring
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("userAuthn", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeFalse())
		})

	})

	Context("Default with PublishAPI", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("userAuthn", customCertificatePresent)
			f.ValuesSet("userAuthn.publishAPI.enable", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

})
