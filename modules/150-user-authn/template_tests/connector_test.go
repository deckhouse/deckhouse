package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: user-authn :: helm template :: connectors", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.clusterVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.nodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")
	})
	Context("With gitlab provider in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.providers", `
- id: gitlabID
  name: gitlabName
  type: Gitlab
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})
		It("Should add gitlab provider Custom Object", func() {
			connector := hec.KubernetesResource("Connector", "d8-user-authn", "gitlabID")

			Expect(connector.Field("metadata.name").String()).To(Equal("gitlabID"))
			Expect(connector.Field("type").String()).To(Equal("gitlab"))
			Expect(connector.Field("name").String()).To(Equal("gitlabName"))
			Expect(connector.Field("id").String()).To(Equal("gitlabID"))
			Expect(connector.Field("email.baseURL").String()).To(Equal("https://example.com"))
			Expect(connector.Field("email.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(connector.Field("email.clientID").String()).To(Equal("clientID"))
			Expect(connector.Field("email.clientSecret").String()).To(Equal("secret"))
			Expect(connector.Field("email.groups").String()).To(MatchJSON(`["Admins","Everyone"]`))
		})
	})

	Context("With Atlassian Crowd provider in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.providers", `
- id: crowdID
  name: crowdName
  type: Crowd
  crowd:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})
		It("Should add crowd provider Custom Object", func() {
			connector := hec.KubernetesResource("Connector", "d8-user-authn", "crowdID")

			Expect(connector.Field("metadata.name").String()).To(Equal("crowdID"))
			Expect(connector.Field("type").String()).To(Equal("atlassian-crowd"))
			Expect(connector.Field("name").String()).To(Equal("crowdName"))
			Expect(connector.Field("id").String()).To(Equal("crowdID"))
			Expect(connector.Field("email.baseURL").String()).To(Equal("https://example.com"))
			Expect(connector.Field("email.clientID").String()).To(Equal("clientID"))
			Expect(connector.Field("email.groups").String()).To(MatchJSON(`["Admins","Everyone"]`))
			Expect(connector.Field("email.usernamePrompt").String()).To(Equal("Crowd username"))
		})
	})
	Context("With Github and OIDC providers in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.providers", `
- id: githubID
  name: githubName
  type: Github
  github:
    clientID: clientID
    clientSecret: secret
    useLoginAsID: true
    orgs:
    - name: TestOrg1
      teams:
      - Admins
      - Everyone
    - name: TestOrg2
- id: oidcID
  name: oidcName
  type: OIDC
  oidc:
    issuer: https://issuer.com
    clientID: clientID
    clientSecret: secret
    basicAuthUnsupported: true
    userIDKey: uuid
    userNameKey: username
    insecureSkipEmailVerified: true
    scopes:
    - groups
    - offline_access`)
			hec.HelmRender()
		})
		It("Should add github and oid providers Custom Object", func() {
			githubConnector := hec.KubernetesResource("Connector", "d8-user-authn", "githubID")

			Expect(githubConnector.Field("metadata.name").String()).To(Equal("githubID"))
			Expect(githubConnector.Field("type").String()).To(Equal("github"))
			Expect(githubConnector.Field("name").String()).To(Equal("githubName"))
			Expect(githubConnector.Field("id").String()).To(Equal("githubID"))
			Expect(githubConnector.Field("email.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(githubConnector.Field("email.clientID").String()).To(Equal("clientID"))
			Expect(githubConnector.Field("email.clientSecret").String()).To(Equal("secret"))
			Expect(githubConnector.Field("email.useLoginAsID").Bool()).To(Equal(true))
			Expect(githubConnector.Field("email.loadAllGroups").Bool()).To(Equal(false))
			Expect(githubConnector.Field("email.orgs").String()).To(MatchJSON(
				`[{"name":"TestOrg1","teams":["Admins","Everyone"]},{"name":"TestOrg2"}]`,
			))

			oidcConnector := hec.KubernetesResource("Connector", "d8-user-authn", "oidcID")

			Expect(oidcConnector.Field("metadata.name").String()).To(Equal("oidcID"))
			Expect(oidcConnector.Field("type").String()).To(Equal("oidc"))
			Expect(oidcConnector.Field("name").String()).To(Equal("oidcName"))
			Expect(oidcConnector.Field("id").String()).To(Equal("oidcID"))
			Expect(oidcConnector.Field("email.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(oidcConnector.Field("email.clientID").String()).To(Equal("clientID"))
			Expect(oidcConnector.Field("email.clientSecret").String()).To(Equal("secret"))
			Expect(oidcConnector.Field("email.userIDKey").String()).To(Equal("uuid"))
			Expect(oidcConnector.Field("email.userNameKey").String()).To(Equal("username"))
			Expect(oidcConnector.Field("email.basicAuthUnsupported").Bool()).To(Equal(true))
			Expect(oidcConnector.Field("email.insecureSkipEmailVerified").Bool()).To(Equal(true))
			Expect(oidcConnector.Field("email.scopes").String()).To(MatchJSON(`["groups","offline_access"]`))
		})
	})
})
