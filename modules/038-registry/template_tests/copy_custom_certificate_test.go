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
cache:
  enabled: true
  publish: true
internal:
  cache:
    enabled: true
    upstream: { scheme: HTTPS, host: registry.example.com }
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  pki:
    httpSecret: HS
    ca: {cert: CA, key: CAK}
    token: {cert: TC, key: TK}
    agent: {cert: AGC, key: AGK}
    distribution: {cert: DC, key: DK}
    auth: {cert: AC, key: AK}
    users:
      - {name: ro, password: rp, passwordHash: rh, role: ReadOnly}
      - {name: rw, password: wp, passwordHash: wh, role: ReadWrite}
`

const customCertificateModeIngressDisable = `
https:
  mode: CustomCertificate
cache:
  enabled: true
  publish: false
internal:
  cache:
    enabled: true
    upstream: { scheme: HTTPS, host: registry.example.com }
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  pki:
    httpSecret: HS
    ca: {cert: CA, key: CAK}
    token: {cert: TC, key: TK}
    agent: {cert: AGC, key: AGK}
    distribution: {cert: DC, key: DK}
    auth: {cert: AC, key: AK}
    users:
      - {name: ro, password: rp, passwordHash: rh, role: ReadOnly}
      - {name: rw, password: wp, passwordHash: wh, role: ReadWrite}
`

var _ = Describe("Module :: registry :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Ingress enable (new arch, phase=New)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", customCertificateModeIngressEnable)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("Non-empty customcertificate if ingress enbale", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})

	})

	Context("Ingress disable (new arch, phase=New)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", customCertificateModeIngressDisable)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("Empty customcertificate if ingress disable", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeFalse())
		})

	})

	Context("Legacy phase — custom-certificate renders via legacy gate", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", `
https:
  mode: CustomCertificate
cache:
  enabled: false
internal:
  orchestrator:
    hash: "testhash"
    state:
      mode: Local
      target_mode: Local
      ingress_enabled: true
      registry_service: node-services
      node_services:
        run: false
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  pki:
    httpSecret: HS
    ca: {cert: CA, key: CAK}
    token: {cert: TC, key: TK}
    agent: {cert: AGC, key: AGK}
    distribution: {cert: DC, key: DK}
    auth: {cert: AC, key: AK}
    users:
      - {name: ro, password: rp, passwordHash: rh, role: ReadOnly}
      - {name: rw, password: wp, passwordHash: wh, role: ReadWrite}
`)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})

		It("renders the legacy custom-certificate secret when ingress_enabled is true in Legacy phase", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-system", "registry-ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})

		It("renders the legacy registry-push Service when ingress_enabled is true in Legacy phase", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			svc := f.KubernetesResource("Service", "d8-system", "registry-push")
			Expect(svc.Exists()).To(BeTrue())
		})

		It("renders the legacy Ingress backed by registry-push in Legacy phase", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ing := f.KubernetesResource("Ingress", "d8-system", "registry")
			Expect(ing.Exists()).To(BeTrue())
			Expect(ing.Field("spec.rules.0.http.paths.0.backend.service.name").String()).To(Equal("registry-push"))
		})
	})

})
