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
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")
	})

	Context("With DexClient in values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexClientCRDs", `
- id: dex-client-grafana:test-grafana
  name: grafana
  namespace: test-grafana
  spec:
    id: grafana
    redirectURIs:
    - "https://grafana.example.com/callback"
    secret: "123456789"
  encodedID: "m5zgcztbnzq4x4u44scceizf"
  clientSecret: test
- id: dex-client-opendistro:test
  name: "opendistro"
  namespace: test
  spec:
    redirectURIs:
    - "https://opendistro.example.com/callback"
  clientSecret: test
  encodedID: "n5ygk3tenfzxi4tpzpzjzzeeeirsk"
`)
			hec.HelmRender()
		})
		It("Should create OAuth2Client objects", func() {
			clientGrafana := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizf")
			Expect(clientGrafana.Exists()).To(BeTrue())
			Expect(clientGrafana.Field("id").String()).To(Equal("dex-client-grafana:test-grafana"))
			Expect(clientGrafana.Field("secret").String()).To(Equal("test"))
			Expect(clientGrafana.Field("redirectURIs").String()).To(MatchJSON(`["https://grafana.example.com/callback"]`))
			Expect(hec.KubernetesResource("Secret", "test-grafana", "dex-client-grafana").Exists()).To(BeTrue())

			clientOpendistro := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "n5ygk3tenfzxi4tpzpzjzzeeeirsk")
			Expect(clientOpendistro.Exists()).To(BeTrue())
			Expect(clientOpendistro.Field("id").String()).To(Equal("dex-client-opendistro:test"))
			Expect(clientOpendistro.Field("secret").String()).To(Equal("test"))
			Expect(clientOpendistro.Field("redirectURIs").String()).To(MatchJSON(`["https://opendistro.example.com/callback"]`))
			Expect(hec.KubernetesResource("Secret", "test", "dex-client-opendistro").Exists()).To(BeTrue())
		})
	})
})
