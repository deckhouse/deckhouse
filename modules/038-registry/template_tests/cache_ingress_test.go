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

const cacheIngressGlobal = `
clusterIsBootstrapped: true
enabledModules: ["cert-manager", "registry"]
modules:
  publicDomainTemplate: "%s.example.com"
  https:
    mode: CertManager
    certManager:
      clusterIssuerName: letsencrypt
  placement: {}
discovery:
  clusterMasterCount: 3
  d8SpecificNodeCountByRole:
    master: 3
internal:
  modules:
    kubeRBACProxyCA:
      cert: KRBAC_CA_PEM
      key: KRBAC_CA_KEY
`

const cachePublishValues = `
upstream:
  host: registry.example.com
  scheme: HTTPS
cache:
  enabled: true
  publish: true
  storageSize: 50Gi
internal:
  cache:
    enabled: true
    upstream: { scheme: HTTPS, host: registry.example.com }
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

var _ = Describe("Module :: registry :: helm template :: cache ingress mTLS", func() {
	f := SetupHelmConfig(``)

	Context("publish enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", cachePublishValues)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("adds realip.clientcert to the distribution config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dist := f.KubernetesResource("Secret", "d8-system", "registry-cache-config").Field("stringData.distribution-config\\.yaml").String()
			Expect(dist).To(ContainSubstring("realip:"))
			Expect(dist).To(ContainSubstring("ca: /pki/ingress-client-ca.crt"))
		})

		It("renders the ingress-client-ca in the cache PKI secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			pki := f.KubernetesResource("Secret", "d8-system", "registry-cache-pki")
			// The test harness force-sets global.internal.modules.kubeRBACProxyCA.cert="test"
			// (testing/helm/init.go) after fixtures, and the cache PKI secret derives
			// ingress-client-ca.crt from it; assert the wired harness value.
			Expect(pki.Field(`data.ingress-client-ca\.crt`).String()).To(Equal(b64("test")))
		})

		It("renders the Ingress backed by the leader Service", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ing := f.KubernetesResource("Ingress", "d8-system", "registry")
			Expect(ing.Exists()).To(BeTrue())
			Expect(ing.Field("spec.rules.0.http.paths.0.backend.service.name").String()).To(Equal("registry-cache-leader"))
			Expect(ing.Field("spec.rules.0.http.paths.0.backend.service.port.number").String()).To(Equal("5001"))
			Expect(ing.Field(`metadata.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol`).String()).To(Equal("HTTPS"))
		})

		It("renders the CertManager Certificate for the ingress TLS", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Certificate", "d8-system", "registry-ingress").Exists()).To(BeTrue())
		})

		It("does not render the old registry-push node-services Service", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-push").Exists()).To(BeFalse())
		})
	})

	Context("publish disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", `
cache:
  enabled: true
  publish: false
internal:
  cache: { enabled: true }
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
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("omits realip + ingress-client-ca when publish is off", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dist := f.KubernetesResource("Secret", "d8-system", "registry-cache-config").Field("stringData.distribution-config\\.yaml").String()
			Expect(dist).ShouldNot(ContainSubstring("realip:"))
			pki := f.KubernetesResource("Secret", "d8-system", "registry-cache-pki")
			Expect(pki.Field(`data.ingress-client-ca\.crt`).Exists()).To(BeFalse())
		})

		It("renders no ingress when publish is off", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ingress", "d8-system", "registry").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Certificate", "d8-system", "registry-ingress").Exists()).To(BeFalse())
		})
	})

	Context("phase Legacy — new (cache-leader) ingress must be absent", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cacheIngressGlobal)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", cachePublishValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("does not render the registry-cache-leader-backed Ingress in legacy mode", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ing := f.KubernetesResource("Ingress", "d8-system", "registry")
			// Legacy mode has no orchestrator.state.ingress_enabled set, so no Ingress at all.
			// The key assertion: if an Ingress does exist it must NOT point at registry-cache-leader.
			if ing.Exists() {
				Expect(ing.Field("spec.rules.0.http.paths.0.backend.service.name").String()).NotTo(Equal("registry-cache-leader"))
			}
		})
	})
})
