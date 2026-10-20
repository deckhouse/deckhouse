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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler", "openvpn"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterDomain: cluster.local
`
const customCertificatePresent = `
tunnelNetwork: 172.25.175.0/24
https:
  mode: CustomCertificate
auth: {}
internal:
  migrated: true
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  auth: {}
`

var _ = Describe("Module :: openvpn :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("openvpn", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-openvpn", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})

	})

	Context("ingress-nginx module v1.3.0 or newer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.3.0")
			f.ValuesSetFromYaml("openvpn", customCertificatePresent)
			f.HelmRender()
		})

		It("uses the annotations that replace configuration snippets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-openvpn", "admin")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").Exists()).To(BeFalse())
		})
	})

	DescribeTable("ingress-nginx module version unknown or older than v1.3.0",
		func(setVersion func()) {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			setVersion()
			f.ValuesSetFromYaml("openvpn", customCertificatePresent)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-openvpn", "admin")
			snippet := ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()
			Expect(snippet).To(ContainSubstring("proxy_ssl_certificate /etc/nginx/ssl/client.crt;"))
			Expect(snippet).To(ContainSubstring("Strict-Transport-Security"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/proxy-ssl-use-controller-certificate").Exists()).To(BeFalse())
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").Exists()).To(BeFalse())
		},
		Entry("version is not discovered", func() {}),
		Entry("version is empty", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "") }),
		Entry("version is not a semver", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "broken") }),
		Entry("version is v1.2.0", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.2.0") }),
	)

	Context("With Gateway API enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("openvpn", customCertificatePresent)
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			f.HelmRender()
		})

		// CustomCertificate has no per-mechanism validation, so Gateway API reuses the Ingress secret.
		It("Should reuse the ingress secret instead of creating a redundant copy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-openvpn", "ingress-tls-customcertificate").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-openvpn", "httproute-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

})
