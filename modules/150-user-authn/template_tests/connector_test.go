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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: connectors", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
	})
	Context("With gitlab provider in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: gitlabID
  displayName: gitlabName
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
			configmap := hec.KubernetesResource("Secret", "d8-user-authn", "dex")

			data, err := base64.StdEncoding.DecodeString(configmap.Field("data.config\\.yaml").String())
			Expect(err).To(BeNil())

			data, err = ConvertYAMLToJSON(data)
			Expect(err).To(BeNil())

			connector := gjson.GetBytes(data, "connectors.0")

			Expect(connector.Get("type").String()).To(Equal("gitlab"))
			Expect(connector.Get("name").String()).To(Equal("gitlabName"))
			Expect(connector.Get("id").String()).To(Equal("gitlabID"))
			Expect(connector.Get("config.baseURL").String()).To(Equal("https://example.com"))
			Expect(connector.Get("config.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(connector.Get("config.clientID").String()).To(Equal("clientID"))
			Expect(connector.Get("config.clientSecret").String()).To(Equal("secret"))
			Expect(connector.Get("config.groups").String()).To(MatchJSON(`["Admins","Everyone"]`))
		})
	})

	Context("With Atlassian Crowd provider in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: crowdID
  displayName: crowdName
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
			configmap := hec.KubernetesResource("Secret", "d8-user-authn", "dex")

			data, err := base64.StdEncoding.DecodeString(configmap.Field("data.config\\.yaml").String())
			Expect(err).To(BeNil())

			data, err = ConvertYAMLToJSON(data)
			Expect(err).To(BeNil())

			connector := gjson.GetBytes(data, "connectors.0")

			Expect(connector.Get("type").String()).To(Equal("atlassian-crowd"))
			Expect(connector.Get("name").String()).To(Equal("crowdName"))
			Expect(connector.Get("id").String()).To(Equal("crowdID"))
			Expect(connector.Get("config.baseURL").String()).To(Equal("https://example.com"))
			Expect(connector.Get("config.clientID").String()).To(Equal("clientID"))
			Expect(connector.Get("config.groups").String()).To(MatchJSON(`["Admins","Everyone"]`))
			Expect(connector.Get("config.usernamePrompt").String()).To(Equal("Crowd username"))
		})
	})
	Context("With Github and OIDC providers in config values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: githubID
  displayName: githubName
  type: Github
  github:
    clientID: clientID
    clientSecret: secret
    useLoginAsID: true
    teamNameField: both
    orgs:
    - name: TestOrg1
      teams:
      - Admins
      - Everyone
    - name: TestOrg2
- id: oidcID
  displayName: oidcName
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
			configmap := hec.KubernetesResource("Secret", "d8-user-authn", "dex")

			data, err := base64.StdEncoding.DecodeString(configmap.Field("data.config\\.yaml").String())
			Expect(err).To(BeNil())

			data, err = ConvertYAMLToJSON(data)
			Expect(err).To(BeNil())

			githubConnector := gjson.GetBytes(data, "connectors.0")

			Expect(githubConnector.Get("type").String()).To(Equal("github"))
			Expect(githubConnector.Get("name").String()).To(Equal("githubName"))
			Expect(githubConnector.Get("id").String()).To(Equal("githubID"))
			Expect(githubConnector.Get("config.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(githubConnector.Get("config.clientID").String()).To(Equal("clientID"))
			Expect(githubConnector.Get("config.clientSecret").String()).To(Equal("secret"))
			Expect(githubConnector.Get("config.useLoginAsID").Bool()).To(Equal(true))
			Expect(githubConnector.Get("config.loadAllGroups").Bool()).To(Equal(false))
			Expect(githubConnector.Get("config.teamNameField").String()).To(Equal("both"))

			Expect(githubConnector.Get("config.orgs").String()).To(MatchJSON(
				`[{"name":"TestOrg1","teams":["Admins","Everyone"]},{"name":"TestOrg2"}]`,
			))

			oidcConnector := gjson.GetBytes(data, "connectors.1")

			Expect(oidcConnector.Get("type").String()).To(Equal("oidc"))
			Expect(oidcConnector.Get("name").String()).To(Equal("oidcName"))
			Expect(oidcConnector.Get("id").String()).To(Equal("oidcID"))
			Expect(oidcConnector.Get("config.redirectURI").String()).To(Equal("https://dex.example.com/callback"))
			Expect(oidcConnector.Get("config.clientID").String()).To(Equal("clientID"))
			Expect(oidcConnector.Get("config.clientSecret").String()).To(Equal("secret"))
			Expect(oidcConnector.Get("config.userIDKey").String()).To(Equal("uuid"))
			Expect(oidcConnector.Get("config.userNameKey").String()).To(Equal("username"))
			Expect(oidcConnector.Get("config.basicAuthUnsupported").Bool()).To(Equal(true))
			Expect(oidcConnector.Get("config.insecureSkipEmailVerified").Bool()).To(Equal(true))
			Expect(oidcConnector.Get("config.scopes").String()).To(MatchJSON(`["groups","offline_access"]`))
		})
	})
})
