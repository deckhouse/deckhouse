/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template_tests

import (
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
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")

		hec.ValuesSet("userAuthn.publishAPI.enable", true)
		hec.ValuesSet("userAuthn.publishAPI.addKubeconfigGeneratorEntry", true)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should deploy publish api and kubeconfig generator", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			certificate := hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls-selfsigned")
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("Issuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("kubernetes-api"))
			Expect(hec.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("With discovered dex cluster ip", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.discoveredDexClusterIP", "10.10.10.10")
			hec.HelmRender()
		})

		It("Should add dex to hosts aliases", func() {
			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())
			kgDeployment := hec.KubernetesResource("Deployment", "d8-user-authn", "kubeconfig-generator")

			Expect(len(kgDeployment.Field("spec.template.spec.hostAliases").Array())).To(Equal(1))
			Expect(kgDeployment.Field("spec.template.spec.hostAliases.0.ip").String()).To(Equal("10.10.10.10"))

			Expect(len(kgDeployment.Field("spec.template.spec.hostAliases.0.hostnames").Array())).To(Equal(1))
			Expect(kgDeployment.Field("spec.template.spec.hostAliases.0.hostnames.0").String()).To(Equal("dex.example.com"))
		})
	})

	Context("With global mode CustomCertificate", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			hec.ValuesSet("global.modules.https.mode", "CustomCertificate")
			hec.HelmRender()
		})

		It("Should deploy secret certificate", func() {
			Expect(hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls-selfsigned").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Certificate", "d8-user-authn", "kubernetes-tls").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate").Exists()).To(BeTrue())
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
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(hec.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
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
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("route53"))
			Expect(hec.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("With crowd provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.crowdProxyCert", "dGVzdA==")
			hec.ValuesSet("userAuthn.internal.crowdProxyKey", "dGVzdA==")
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: crowdNexID
  displayName: crowdNextName
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
