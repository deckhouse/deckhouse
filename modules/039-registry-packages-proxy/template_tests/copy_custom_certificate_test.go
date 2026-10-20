/*
Copyright 2026 Flant JSC

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

const globalValuesBootstrapped = `
clusterIsBootstrapped: true
enabledModules: ["vertical-pod-autoscaler", "registry-packages-proxy", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
  ingressClass: nginx
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterUUID: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
`

const globalValuesNotBootstrapped = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "registry-packages-proxy", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
  ingressClass: nginx
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterUUID: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
`

const customCertificatePresent = `
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: registry-packages-proxy :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Cluster bootstrapped, https.mode = CustomCertificate", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		It("renders the ingress-tls-customcertificate secret in d8-cloud-instance-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			created := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "ingress-tls-customcertificate")
			Expect(created.Exists()).To(BeTrue())
			Expect(created.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})
	})

	Context("Cluster bootstrapped, ingress-nginx module v1.3.0 or newer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.3.0")
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		It("uses the annotation that replaces the configuration snippet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-cloud-instance-manager", "registry-packages-proxy")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").String()).To(Equal("true"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").Exists()).To(BeFalse())
		})
	})

	DescribeTable("Cluster bootstrapped, ingress-nginx module version unknown or older than v1.3.0",
		func(setVersion func()) {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			setVersion()
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingress := f.KubernetesResource("Ingress", "d8-cloud-instance-manager", "registry-packages-proxy")
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()).To(ContainSubstring("Strict-Transport-Security"))
			Expect(ingress.Field("metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/ingress-nginx-hsts").Exists()).To(BeFalse())
		},
		Entry("version is not discovered", func() {}),
		Entry("version is empty", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "") }),
		Entry("version is not a semver", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "broken") }),
		Entry("version is v1.2.0", func() { f.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.2.0") }),
	)

	Context("Cluster not bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesNotBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		It("does not render the secret because the public ingress is not deployed yet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			created := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "ingress-tls-customcertificate")
			Expect(created.Exists()).To(BeFalse())
		})
	})

	Context("With Gateway API enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.name", "shared-gateway")
			f.ValuesSet("global.discovery.gatewayAPIDefaultGateway.namespace", "d8-alb")
			f.HelmRender()
		})

		// CustomCertificate has no per-mechanism validation, so Gateway API reuses the Ingress secret.
		It("Should reuse the ingress secret instead of creating a redundant copy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "ingress-tls-customcertificate").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "registry-packages-proxy-httproute-tls-customcertificate").Exists()).To(BeFalse())
		})
	})
})
