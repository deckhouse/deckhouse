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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: DexClient", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
	})

	Context("With DexClient in values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexClientCRDs", `
- id: dex-client-grafana@test-grafana
  encodedID: "m5zgcztbnzq4x4u44scceizf"
  name: grafana
  namespace: test-grafana
  spec:
    id: grafana
    redirectURIs:
    - "https://grafana.example.com/callback"
    secret: "123456789"
  legacyID: dex-client-grafana:test-grafana
  legacyEncodedID: "m5zgcztbnzq4x4u44scceizfxxx"
  clientSecret: test
- id: dex-client-opendistro@test
  name: "opendistro"
  namespace: test
  spec:
    redirectURIs:
    - "https://opendistro.example.com/callback"
    allowedGroups:
    - aaa
    - ccc
    allowedEmails:
    - bb@aaa.com
  clientSecret: test
  encodedID: "n5ygk3tenfzxi4tpzpzjzzeeeirsk"
  legacyID: dex-client-opendistro:test
  legacyEncodedID: "m5zgcztbnzq4x4u44scceizfyyy"
`)
			hec.HelmRender()
		})
		It("Should create OAuth2Client objects", func() {
			clientGrafana := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizf")
			Expect(clientGrafana.Exists()).To(BeTrue())
			Expect(clientGrafana.Field("id").String()).To(Equal("dex-client-grafana@test-grafana"))
			Expect(clientGrafana.Field("secret").String()).To(Equal("test"))
			Expect(clientGrafana.Field("redirectURIs").String()).To(MatchJSON(`["https://grafana.example.com/callback"]`))
			Expect(hec.KubernetesResource("Secret", "test-grafana", "dex-client-grafana").Exists()).To(BeTrue())

			clientGrafanaLegacy := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizfxxx")
			Expect(clientGrafanaLegacy.Exists()).To(BeTrue())
			Expect(clientGrafanaLegacy.Field("id").String()).To(Equal("dex-client-grafana:test-grafana"))
			Expect(clientGrafanaLegacy.Field("secret").String()).To(Equal("test"))
			Expect(clientGrafanaLegacy.Field("redirectURIs").String()).To(MatchJSON(`["https://grafana.example.com/callback"]`))
			Expect(hec.KubernetesResource("Secret", "test-grafana", "dex-client-grafana").Exists()).To(BeTrue())

			clientOpendistro := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "n5ygk3tenfzxi4tpzpzjzzeeeirsk")
			Expect(clientOpendistro.Exists()).To(BeTrue())
			Expect(clientOpendistro.Field("id").String()).To(Equal("dex-client-opendistro@test"))
			Expect(clientOpendistro.Field("secret").String()).To(Equal("test"))
			Expect(clientOpendistro.Field("redirectURIs").String()).To(MatchJSON(`["https://opendistro.example.com/callback"]`))
			Expect(clientOpendistro.Field("allowedEmails").String()).To(MatchJSON(`["bb@aaa.com"]`))
			Expect(clientOpendistro.Field("allowedGroups").String()).To(MatchJSON(`["aaa", "ccc"]`))
			Expect(hec.KubernetesResource("Secret", "test", "dex-client-opendistro").Exists()).To(BeTrue())

			clientOpendistroLegacy := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizfyyy")
			Expect(clientOpendistroLegacy.Exists()).To(BeTrue())
			Expect(clientOpendistroLegacy.Field("id").String()).To(Equal("dex-client-opendistro:test"))
			Expect(clientOpendistroLegacy.Field("secret").String()).To(Equal("test"))
			Expect(clientOpendistroLegacy.Field("redirectURIs").String()).To(MatchJSON(`["https://opendistro.example.com/callback"]`))
			Expect(clientOpendistroLegacy.Field("allowedEmails").String()).To(MatchJSON(`["bb@aaa.com"]`))
			Expect(clientOpendistroLegacy.Field("allowedGroups").String()).To(MatchJSON(`["aaa","ccc"]`))
			Expect(hec.KubernetesResource("Secret", "test", "dex-client-opendistro").Exists()).To(BeTrue())
		})
	})
})
