package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: publish api", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.clusterVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.nodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.publishAPI.enable", true)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should deploy publish api and kubeconfig generator", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "kubernetes-api").Field("metadata.annotations").Map()).To(HaveKey("certmanager.k8s.io/issuer"))
		})
	})

	Context("With publish API global mode", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			hec.ValuesSet("userAuthn.publishAPI.https.global.kubeconfigGeneratorMasterCA", "simplecastring")
			hec.HelmRender()
		})
		It("Should use cluster issuer", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "kubernetes-api").Field("metadata.annotations").Map()).To(HaveKey("certmanager.k8s.io/cluster-issuer"))
		})
	})

	Context("With crowd provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.crowdProxyCert", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.crowdProxyKey", "dGVzdA==")
			hec.ValuesSetFromYaml("userAuthn.providers", `
- id: crowdNexID
  name: crowdNextName
  type: Crowd
  crowd:
    enableBasicAuth: true
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com`)
			hec.HelmRender()
		})
		It("Should deploy basic auth proxy deployment and ingress", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "crowd-basic-auth-proxy").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "crowd-basic-auth-proxy").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Ingress", "d8-user-authn", "kubernetes-api").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()).To(
				Equal("if ($http_authorization ~ \"^(.*)Basic(.*)$\") {\n  rewrite ^(.*)$ /basic-auth$1;\n}\n"))
		})
	})
})
