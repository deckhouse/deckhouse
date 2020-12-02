package template_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: publish api", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")

		hec.ValuesSet("userAuthn.publishAPI.enable", true)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should deploy publish api and kubeconfig generator", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			certificate := hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls")
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("Issuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("kubernetes-api"))
			Expect(certificate.Field("spec.acme.config.0.http01.ingressClass").String()).To(Equal("nginx"))
		})
	})

	Context("With publish API global mode", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			hec.ValuesSet("userAuthn.publishAPI.ingressClass", "my-ingress-class")
			hec.ValuesSet("userAuthn.publishAPI.https.global.kubeconfigGeneratorMasterCA", "simplecastring")
			hec.HelmRender()
		})
		It("Should use cluster issuer", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			certificate := hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls")
			fmt.Println(certificate.Field("spec").String())
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(certificate.Field("spec.acme.config.0.http01.ingressClass").String()).To(Equal("my-ingress-class"))
		})
	})

	Context("With publish API global mode and route53 issuer", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			hec.ValuesSet("userAuthn.publishAPI.https.global.kubeconfigGeneratorMasterCA", "simplecastring")
			hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "route53")
			hec.HelmRender()
		})
		It("Should use cluster issuer and dns challenge", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			certificate := hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls")
			fmt.Println(certificate.Field("spec").String())
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("route53"))
			Expect(certificate.Field("spec.acme.config.0.dns01.provider").String()).To(Equal("route53"))
		})
	})

	Context("With crowd provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.crowdProxyCert", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.crowdProxyKey", "dGVzdA==")
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: crowdNexID
  name: crowdNextName
  type: Crowd
  crowd:
    enableBasicAuth: true
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com`)
			hec.ValuesSetFromYaml("userAuthn.publishAPI.whitelistSourceRanges", `
- 1.1.1.1
- 192.168.0.0/24
`)
			hec.HelmRender()
		})
		It("Should deploy basic auth proxy deployment and ingress", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "crowd-basic-auth-proxy").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "crowd-basic-auth-proxy").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "kubernetes-api").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()).To(
				Equal("if ($http_authorization ~ \"^(.*)Basic(.*)$\") {\n  rewrite ^(.*)$ /basic-auth$1;\n}\n"))
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "kubernetes-api").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(
				Equal("1.1.1.1,192.168.0.0/24"))
		})
	})
})
