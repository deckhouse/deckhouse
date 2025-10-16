/*
Copyright 2025 Flant JSC

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

const globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "registry", "cert-manager"]
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
const customCertificateModeIngressEnable = `
https:
  mode: CustomCertificate
internal:
  orchestrator:
    hash: "123"
    state:
      ingress_enabled: true
      conditions: []
      mode: "Local"
      target_mode: "Local"
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

const customCertificateModeIngressDisable = `
https:
  mode: CustomCertificate
internal:
  orchestrator: {}
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: registry :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Ingress enable", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", customCertificateModeIngressEnable)
			f.HelmRender()
		})

		It("Non-empty customcertificate if ingress enbale", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

	Context("Ingress disable", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", customCertificateModeIngressDisable)
			f.HelmRender()
		})

		It("Empty customcertificate if ingress disable", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeFalse())
		})

	})

})
