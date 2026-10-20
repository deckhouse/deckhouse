/*
Copyright 2024 Flant JSC

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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "deckhouse-tools"]
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
const globalValuesWithDefaultGateway = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "deckhouse-tools"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  gatewayAPIGateway:
    name: default
    namespace: d8-alb
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

var _ = Describe("Module :: deckhouse-tools :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("deckhouseTools", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "tools-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})

	})

	Context("With default gateway", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesWithDefaultGateway)
			f.ValuesSetFromYaml("deckhouseTools", customCertificatePresent)
			f.HelmRender()
		})

		// CustomCertificate has no per-mechanism validation, so Gateway API reuses the Ingress secret.
		It("Should reuse the ingress secret instead of creating a redundant copy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "tools-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))

			Expect(f.KubernetesResource("Secret", "d8-system", "tools-httproute-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	// render draws the tools Ingress for an ingress-nginx module version ("" leaves it undiscovered),
	// with basic or external authentication.
	render := func(version string, externalAuth bool) object_store.KubeObject {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.clusterIsBootstrapped", true)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		if version != "" {
			f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", version)
		}
		f.ValuesSetFromYaml("deckhouseTools", customCertificatePresent)
		f.ValuesSet("deckhouseTools.internal.auth.password", "test")
		if externalAuth {
			f.ValuesSetFromYaml("deckhouseTools.auth.externalAuthentication",
				`{authURL: "https://auth.example.com/auth", authSignInURL: "https://auth.example.com/sign-in"}`)
		}
		f.ApplyOpenAPIDefaults()
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		ingress := f.KubernetesResource("Ingress", "d8-system", "deckhouse-tools")
		Expect(ingress.Exists()).To(BeTrue())
		return ingress
	}

	Context("ingress-nginx module v1.3.0 or newer", func() {
		It("uses the annotations that replace configuration snippets with basic authentication", func() {
			ingress := render("1.3.0", false)
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/auth-type").String()).To(Equal("basic"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").Exists()).To(BeFalse())
		})

		It("uses the annotations that replace configuration snippets with external authentication", func() {
			ingress := render("1.3.0", true)
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/auth-url").String()).To(Equal("https://auth.example.com/auth"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").Exists()).To(BeFalse(),
				"external authentication never presented the client certificate, so the replacement does not either")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").Exists()).To(BeFalse())
		})
	})

	Context("ingress-nginx module version not discovered", func() {
		It("keeps the configuration snippets with basic authentication", func() {
			ingress := render("", false)
			snippet := ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()
			Expect(snippet).To(ContainSubstring("proxy_ssl_certificate /etc/nginx/ssl/client.crt;"))
			Expect(snippet).To(ContainSubstring("Strict-Transport-Security"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").Exists()).To(BeFalse())
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").Exists()).To(BeFalse())
		})

		It("keeps the configuration snippets with external authentication", func() {
			ingress := render("", true)
			snippet := ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()
			Expect(snippet).NotTo(ContainSubstring("proxy_ssl_certificate"))
			Expect(snippet).To(ContainSubstring("Strict-Transport-Security"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").Exists()).To(BeFalse())
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").Exists()).To(BeFalse())
		})
	})
})
