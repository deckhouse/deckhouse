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

const globalValuesBootstrapped = `
clusterIsBootstrapped: true
enabledModules: ["vertical-pod-autoscaler", "documentation"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterDomain: cluster.local
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

const documentationValuesBootstrapped = `
https:
  mode: CustomCertificate
auth: {}
internal:
  deployDexAuthenticator: false
  auth:
    password: testpassword
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: documentation :: helm template :: network policy", func() {
	f := SetupHelmConfig(``)

	Context("Bootstrapped cluster with public domain", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("documentation", documentationValuesBootstrapped)
			f.HelmRender()
		})

		It("Renders a NetworkPolicy that locks down the builder port", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			np := f.KubernetesResource("NetworkPolicy", "d8-system", "documentation")
			Expect(np.Exists()).To(BeTrue())

			Expect(np.Field("spec.podSelector.matchLabels.app").String()).To(Equal("documentation"))
			Expect(np.Field("spec.policyTypes").String()).To(Equal(`["Ingress"]`))

			ingress := np.Field("spec.ingress").Array()
			Expect(ingress).To(HaveLen(3))

			// :8081 — only from the Deckhouse controller in d8-system.
			Expect(ingress[0].Get("ports.0.port").Int()).To(Equal(int64(8081)))
			Expect(ingress[0].Get("from.0.namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name").String()).To(Equal("d8-system"))
			Expect(ingress[0].Get("from.0.podSelector.matchLabels.app").String()).To(Equal("deckhouse"))

			// :8443 — kube-rbac-proxy, reachable without a source restriction.
			Expect(ingress[1].Get("ports.0.port").Int()).To(Equal(int64(8443)))
			Expect(ingress[1].Get("from").Exists()).To(BeFalse())

			// :9090 — metrics scraped from d8-monitoring.
			Expect(ingress[2].Get("ports.0.port").Int()).To(Equal(int64(9090)))
			Expect(ingress[2].Get("from.0.namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name").String()).To(Equal("d8-monitoring"))
		})
	})

	Context("Non-bootstrapped cluster", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("documentation", customCertificatePresent)
			f.HelmRender()
		})

		It("Does not render the NetworkPolicy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("NetworkPolicy", "d8-system", "documentation").Exists()).To(BeFalse())
		})
	})
})
