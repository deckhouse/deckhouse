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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "documentation"]
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

var _ = Describe("Module :: documentation :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("documentation", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})

	})

	Context("With Gateway API enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("documentation", customCertificatePresent)
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			f.HelmRender()
		})

		// CustomCertificate has no per-mechanism validation, so Gateway API reuses the Ingress secret.
		It("Should reuse the ingress secret instead of creating a redundant copy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", "ingress-tls-customcertificate").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-system", "httproute-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("ingress-nginx module v1.3.0 or newer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("documentation", customCertificatePresent)
			f.ApplyOpenAPIDefaults()
			f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.3.0")
			f.HelmRender()
		})

		It("uses the annotations that replace configuration snippets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-system", "documentation")
			Expect(ingress.Exists()).To(BeTrue())
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").Exists()).To(BeFalse())
		})
	})

	Context("ingress-nginx module version not discovered", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("documentation", customCertificatePresent)
			f.ApplyOpenAPIDefaults()
			f.HelmRender()
		})

		It("keeps the configuration snippets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-system", "documentation")
			Expect(ingress.Exists()).To(BeTrue())
			snippet := ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()
			Expect(snippet).To(ContainSubstring("proxy_ssl_certificate /etc/nginx/ssl/client.crt;"))
			Expect(snippet).To(ContainSubstring("Strict-Transport-Security"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").Exists()).To(BeFalse())
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").Exists()).To(BeFalse())
		})
	})
})
