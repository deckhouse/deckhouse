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

// The publication endpoint of the CURRENT implementation, under the operator's own certificate.
//
// It renders on every managed cluster with a cache, and names the copied secret exactly as the
// previous implementation's ingress does — `helm_lib_module_https_secret_name` resolves to
// `registry-ingress-tls-customcertificate` under `https.mode: CustomCertificate`. The copy itself
// used to be gated on the previous implementation's `ingress_enabled`, which no managed cluster
// sets: the handover deletes that state machine's secret and nothing in this implementation writes
// the flag. So the endpoint named a secret that did not exist.
//
// Nginx answers such an ingress with its own self-signed certificate rather than refusing to serve,
// which is the worst shape for this to be wrong in: this is the write endpoint of an air-gapped
// registry, reachable from outside, and the client that has to trust it is `d8 mirror push`.
const v2WithCustomCertificate = v2Enabled + `
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: registry :: helm template :: custom-certificate for the v2 endpoint", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2WithCustomCertificate)
		f.HelmRender()
	})

	It("copies the certificate the publication endpoint names", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		ingress := f.KubernetesResource("Ingress", "d8-system", "registry-push")
		Expect(ingress.Exists()).To(BeTrue())
		named := ingress.Field("spec.tls.0.secretName").String()
		Expect(named).To(Equal("registry-ingress-tls-customcertificate"))

		copied := f.KubernetesResource("Secret", "d8-system", named)
		Expect(copied.Exists()).To(BeTrue(),
			"the endpoint names this secret, and nothing else creates it")
		Expect(copied.Field("data").String()).
			To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
	})
})

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
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
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
