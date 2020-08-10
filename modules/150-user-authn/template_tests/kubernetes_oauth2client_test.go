package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: kubernetes oauth2client", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")
	})

	Context("Without dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `[]`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})

	Context("With dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  allowAccessToKubernetes: "true"
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.HelmRender()
		})
		It("Should deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeTrue())
		})
	})

	Context("With dex authenticator without access to Kubernetes API", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})
})
