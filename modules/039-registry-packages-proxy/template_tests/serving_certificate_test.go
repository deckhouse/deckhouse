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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const globalValuesWithCAs = `
clusterIsBootstrapped: true
enabledModules: ["vertical-pod-autoscaler", "registry-packages-proxy", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
  ingressClass: nginx
internal:
  modules:
    kubeRBACProxyCA:
      cert: PLATFORM-CA-PEM
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterUUID: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
  kubernetesCA: |
    -----BEGIN CERTIFICATE-----
    MIIBCLUSTERCAFIRSTLINE
    MIIBCLUSTERCASECONDLINE
    -----END CERTIFICATE-----
`

// clusterCAPEM is the value above as it must appear inside the rendered bundle:
// a multi-line PEM keeps its line breaks and gains no stray indentation.
const clusterCAPEM = `-----BEGIN CERTIFICATE-----
MIIBCLUSTERCAFIRSTLINE
MIIBCLUSTERCASECONDLINE
-----END CERTIFICATE-----`

// customCertificateData is unrelated to the serving certificate, but the module
// requires it to render the public ingress in CustomCertificate mode.
const servingCertificatePresent = `
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  proxyAddresses: ["192.168.0.1"]
  proxyCert:
    ca: CACACA
    crt: CRTCRTCRT
    key: KEYKEYKEY
`

const servingCertificateAbsent = `
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

const kubeRBACProxyArgs = `spec.template.spec.containers.#(name=="kube-rbac-proxy").args`

var _ = Describe("Module :: registry-packages-proxy :: helm template :: serving certificate", func() {
	f := SetupHelmConfig(``)

	Context("Certificate issued by the module", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesWithCAs)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", servingCertificatePresent)
			f.HelmRender()
		})

		It("stores the certificate in a TLS secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "registry-packages-proxy-tls")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("type").String()).To(Equal("kubernetes.io/tls"))
			Expect(secret.Field(`data.ca\.crt`).String()).To(Equal("Q0FDQUNB"))
			Expect(secret.Field(`data.tls\.crt`).String()).To(Equal("Q1JUQ1JUQ1JU"))
			Expect(secret.Field(`data.tls\.key`).String()).To(Equal("S0VZS0VZS0VZ"))
		})

		It("trusts both client issuers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			clientCA := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "registry-packages-proxy-client-ca")
			Expect(clientCA.Exists()).To(BeTrue())

			platformCA := f.ValuesGet("global.internal.modules.kubeRBACProxyCA.cert").String()
			Expect(platformCA).ToNot(BeEmpty())
			Expect(clientCA.Field(`data.ca\.crt`).String()).To(ContainSubstring(platformCA))
			Expect(clientCA.Field(`data.ca\.crt`).String()).To(ContainSubstring(clusterCAPEM))
		})

		It("serves that certificate on the proxy port", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "registry-packages-proxy")
			args := deployment.Field(kubeRBACProxyArgs).String()
			Expect(args).To(ContainSubstring("--tls-cert-file=/etc/registry-packages-proxy/tls/tls.crt"))
			Expect(args).To(ContainSubstring("--tls-private-key-file=/etc/registry-packages-proxy/tls/tls.key"))
			Expect(args).To(ContainSubstring("--client-ca-file=/etc/registry-packages-proxy/client-ca/ca.crt"))
		})

		It("mounts the secret and the client CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			volumes := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "registry-packages-proxy").
				Field("spec.template.spec.volumes").String()
			Expect(volumes).To(ContainSubstring(`"secretName":"registry-packages-proxy-tls"`))
			Expect(volumes).To(ContainSubstring(`"name":"registry-packages-proxy-client-ca"`))
		})
	})

	Context("Certificate not issued yet", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesWithCAs)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", servingCertificateAbsent)
			f.HelmRender()
		})

		It("renders the module without the TLS secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "registry-packages-proxy-tls").Exists()).
				To(BeFalse())
		})
	})
})
